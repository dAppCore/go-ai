package security

import (
	"dappco.re/go/core"
	"dappco.re/go/core/scm/repos"
	"forge.lthn.ai/core/cli/pkg/cli"
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
		targets = append(targets, SecurityTarget{
			DisplayName: repository.Name,
			FullName:    core.Sprintf("%s/%s", registry.Org, repository.Name),
		})
	}
	return targets, nil
}

// parseSecurityTarget("wailsapp/wails") converts an external owner/repo string into the shared target shape.
func parseSecurityTarget(target string) (SecurityTarget, error) {
	parts := core.SplitN(target, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return SecurityTarget{}, cli.Err("invalid target format: use owner/repo (e.g. wailsapp/wails)")
	}

	return SecurityTarget{
		DisplayName: parts[1],
		FullName:    target,
	}, nil
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
	endpoint := core.Sprintf("orgs/%s/repos?per_page=100&type=all", org)
	output, err := runGitHubAPIRequest(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, "list GitHub repositories for "+org)
	}

	targets, err := decodeGitHubRepositoryNames(output)
	if err != nil {
		return nil, cli.Wrap(err, "parse GitHub repositories for "+org)
	}

	return targets, nil
}
