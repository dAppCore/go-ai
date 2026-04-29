package security

import (
	"dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/scm/repos"
)

// SecurityTarget{DisplayName: "go-ai", FullName: "core/go-ai"} is the canonical repo target shape used by security commands.
type SecurityTarget struct {
	DisplayName string
	FullName    string
}

// resolveSecurityTargets("", "go-ai", "") returns the registry-backed target for core/go-ai.
func resolveSecurityTargets(registryPath, repoFilter, externalTarget string) core.Result {
	if externalTarget != "" {
		targetResult := parseSecurityTarget(externalTarget)
		if !targetResult.OK {
			return targetResult
		}
		return core.Ok([]SecurityTarget{targetResult.Value.(SecurityTarget)})
	}

	registryResult := loadRegistry(registryPath)
	if !registryResult.OK {
		return registryResult
	}
	registry := registryResult.Value.(*repos.Registry)

	repositories := selectRegistryRepos(registry, repoFilter)
	if len(repositories) == 0 {
		return core.Fail(cli.Err("repo not found: %s", repoFilter))
	}

	targets := make([]SecurityTarget, 0, len(repositories))
	for _, repository := range repositories {
		targetResult := parseSecurityTarget(core.Sprintf("%s/%s", registry.Org, repository.Name))
		if !targetResult.OK {
			return core.Fail(cli.Err("invalid repository target in registry: %s/%s", registry.Org, repository.Name))
		}
		targets = append(targets, targetResult.Value.(SecurityTarget))
	}
	return core.Ok(targets)
}

// metricRepositoryForTargets returns the repository name to record in metrics when a
// security command is scoped to exactly one repository.
func metricRepositoryForTargets(targets []SecurityTarget) string {
	if len(targets) != 1 {
		return ""
	}
	return targets[0].FullName
}

// parseSecurityTarget("wailsapp/wails") converts an external owner/repo string into the shared target shape.
func parseSecurityTarget(target string) core.Result {
	parts := core.SplitN(target, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return core.Fail(cli.Err("invalid target format: use owner/repo (e.g. wailsapp/wails)"))
	}
	if !isSafeGitHubPathComponent(parts[0]) || !isSafeGitHubPathComponent(parts[1]) {
		return core.Fail(cli.Err("invalid target format: use owner/repo (e.g. wailsapp/wails)"))
	}

	return core.Ok(SecurityTarget{
		DisplayName: parts[1],
		FullName:    target,
	})
}

func isSafeGitHubPathComponent(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

func selectRegistryRepos(registry *repos.Registry, repoFilter string) []*repos.Repo {
	if repoFilter != "" {
		if repository, ok := registry.Get(repoFilter); ok {
			return []*repos.Repo{securityRepoView(repository)}
		}
		return nil
	}
	repositories := registry.List()
	selected := make([]*repos.Repo, 0, len(repositories))
	for _, repository := range repositories {
		selected = append(selected, securityRepoView(repository))
	}
	return selected
}

func securityRepoView(repository *repos.Repo) *repos.Repo {
	if repository == nil {
		return nil
	}
	return &repos.Repo{Name: repository.Name}
}

func securitySectionLabel(label, externalTarget string) string {
	if externalTarget == "" {
		return label
	}
	return label + " (" + externalTarget + ")"
}

func listGitHubOrgTargets(org string) core.Result {
	if !isSafeGitHubPathComponent(org) {
		return core.Fail(cli.Err("invalid org value: %q", org))
	}

	endpoint := core.Sprintf("orgs/%s/repos?per_page=100&type=all", org)
	outputResult := callGitHubAPIRequest(endpoint)
	if !outputResult.OK {
		err, _ := coreResultError(outputResult).(error)
		return core.Fail(cli.Wrap(err, "list GitHub repositories for "+org))
	}
	output := outputResult.Value.([]byte)

	targetsResult := decodeGitHubRepositoryNames(output)
	if !targetsResult.OK {
		err, _ := coreResultError(targetsResult).(error)
		return core.Fail(cli.Wrap(err, "parse GitHub repositories for "+org))
	}
	targets := targetsResult.Value.([]string)

	for _, target := range targets {
		if r := parseSecurityTarget(target); !r.OK {
			return core.Fail(cli.Err("invalid repository target returned by GitHub: %s", target))
		}
	}

	return core.Ok(targets)
}
