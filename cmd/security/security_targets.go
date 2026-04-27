package security

import (
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/core"
	"dappco.re/go/scm/repos"
)

// SecurityTarget{DisplayName: "go-ai", FullName: "core/go-ai"} is the canonical repo target shape used by security commands.
type SecurityTarget struct {
	DisplayName string
	FullName    string
}

// resolveSecurityTargets("", "go-ai", "") returns the registry-backed target for core/go-ai.
func resolveSecurityTargets(registryPath, repoFilter, externalTarget string) ([]SecurityTarget, error) {
	if externalTarget != "" {
		target, err := parseSecurityTarget(externalTarget)
		if err != nil {
			return nil, err
		}
		return []SecurityTarget{target}, nil
	}

	registry, err := loadRegistry(registryPath)
	if err != nil {
		return nil, err
	}

	repositories := selectRegistryRepos(registry, repoFilter)
	if len(repositories) == 0 {
		return nil, cli.Err("repo not found: %s", repoFilter)
	}

	targets := make([]SecurityTarget, 0, len(repositories))
	for _, repository := range repositories {
		target, err := parseSecurityTarget(core.Sprintf("%s/%s", registry.Org, repository.Name))
		if err != nil {
			return nil, cli.Err("invalid repository target in registry: %s/%s", registry.Org, repository.Name)
		}
		targets = append(targets, target)
	}
	return targets, nil
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
func parseSecurityTarget(target string) (SecurityTarget, error) {
	parts := core.SplitN(target, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return SecurityTarget{}, cli.Err("invalid target format: use owner/repo (e.g. wailsapp/wails)")
	}
	if !isSafeGitHubPathComponent(parts[0]) || !isSafeGitHubPathComponent(parts[1]) {
		return SecurityTarget{}, cli.Err("invalid target format: use owner/repo (e.g. wailsapp/wails)")
	}

	return SecurityTarget{
		DisplayName: parts[1],
		FullName:    target,
	}, nil
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
			return []*repos.Repo{repository}
		}
		return nil
	}
	return registry.List()
}

func securitySectionLabel(label, externalTarget string) string {
	if externalTarget == "" {
		return label
	}
	return label + " (" + externalTarget + ")"
}

func listGitHubOrgTargets(org string) ([]string, error) {
	if !isSafeGitHubPathComponent(org) {
		return nil, cli.Err("invalid org value: %q", org)
	}

	endpoint := core.Sprintf("orgs/%s/repos?per_page=100&type=all", org)
	output, err := callGitHubAPIRequest(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, "list GitHub repositories for "+org)
	}

	targets, err := decodeGitHubRepositoryNames(output)
	if err != nil {
		return nil, cli.Wrap(err, "parse GitHub repositories for "+org)
	}

	for _, target := range targets {
		if _, err := parseSecurityTarget(target); err != nil {
			return nil, cli.Err("invalid repository target returned by GitHub: %s", target)
		}
	}

	return targets, nil
}
