package security

import (
	"reflect"
	"testing"

	core "dappco.re/go"
	"dappco.re/go/scm/repos"
)

func TestParseSecurityTarget_Good(t *testing.T) {
	result := parseSecurityTarget("wailsapp/wails")
	if !result.OK {
		t.Fatalf("parseSecurityTarget: %s", result.Error())
	}
	got := result.Value.(SecurityTarget)

	want := SecurityTarget{
		DisplayName: "wails",
		FullName:    "wailsapp/wails",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseSecurityTarget = %+v, want %+v", got, want)
	}
}

func TestParseSecurityTarget_Bad(t *testing.T) {
	for _, input := range []string{"", "wailsapp", "/wails", "wailsapp/", "wailsapp/wails/extra", "wails app/wails"} {
		if result := parseSecurityTarget(input); result.OK {
			t.Fatalf("expected error for %q, got nil", input)
		}
	}
}

func TestIsSafeGitHubPathComponent_Good(t *testing.T) {
	for _, input := range []string{"wailsapp", "go-ai", "go_ai", "go.ai"} {
		if !isSafeGitHubPathComponent(input) {
			t.Fatalf("expected %q to be accepted", input)
		}
	}
}

func TestSelectRegistryRepos_Good_Filter(t *testing.T) {
	registry := &repos.Registry{
		Org: "core",
		Repos: map[string]*repos.Repo{
			"go-ai":  {Name: "go-ai"},
			"go-rag": {Name: "go-rag"},
		},
	}

	got := selectRegistryRepos(registry, "go-rag")
	want := []*repos.Repo{{Name: "go-rag"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectRegistryRepos(filter) = %+v, want %+v", got, want)
	}
}

func TestSecuritySectionLabel_Good(t *testing.T) {
	if got := securitySectionLabel("Alerts", ""); got != "Alerts" {
		t.Fatalf("securitySectionLabel without target = %q", got)
	}
	if got := securitySectionLabel("Alerts", "wailsapp/wails"); got != "Alerts (wailsapp/wails)" {
		t.Fatalf("securitySectionLabel with target = %q", got)
	}
}

func TestMetricRepositoryForTargets_Good(t *testing.T) {
	if got := metricRepositoryForTargets(nil); got != "" {
		t.Fatalf("metricRepositoryForTargets(nil) = %q, want empty string", got)
	}
	if got := metricRepositoryForTargets([]SecurityTarget{{FullName: "acme/api"}}); got != "acme/api" {
		t.Fatalf("metricRepositoryForTargets(one) = %q, want acme/api", got)
	}
	if got := metricRepositoryForTargets([]SecurityTarget{{FullName: "acme/api"}, {FullName: "acme/web"}}); got != "" {
		t.Fatalf("metricRepositoryForTargets(many) = %q, want empty string", got)
	}
}

func TestResolveSecurityTargets_Good_ExternalTarget(t *testing.T) {
	result := resolveSecurityTargets("", "", "acme/api")
	if !result.OK {
		t.Fatalf("resolveSecurityTargets: %s", result.Error())
	}
	got := result.Value.([]SecurityTarget)
	want := []SecurityTarget{{DisplayName: "api", FullName: "acme/api"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveSecurityTargets = %+v, want %+v", got, want)
	}
}

func TestResolveSecurityTargets_Bad_InvalidExternalTarget(t *testing.T) {
	if result := resolveSecurityTargets("", "", "acme api"); result.OK {
		t.Fatal("expected invalid external target to fail")
	}
}

func TestListGitHubOrgTargets_Good(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "orgs/acme/repos?per_page=100&type=all" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(`[
			{"full_name":"acme/web"},
			{"full_name":"acme/api"},
			{"full_name":"acme/api"},
			{"full_name":""}
		]`), nil
	})

	result := listGitHubOrgTargets("acme")
	if !result.OK {
		t.Fatalf("listGitHubOrgTargets: %s", result.Error())
	}
	got := result.Value.([]string)
	want := []string{"acme/api", "acme/web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listGitHubOrgTargets = %v, want %v", got, want)
	}
}

func TestSecurityTargets_listGitHubOrgTargets_Bad_RejectsInvalidOrgBeforeGitHubCall(t *testing.T) {
	stubGitHubAPI(t, func(string) ([]byte, error) {
		t.Fatal("GitHub API should not be called for an invalid org name")
		return nil, nil
	})

	if result := listGitHubOrgTargets("bad org"); result.OK {
		t.Fatal("expected invalid org to fail")
	}
}

func TestListGitHubOrgTargets_Bad_InvalidRepositoryReturnedByGitHub(t *testing.T) {
	stubGitHubAPI(t, func(string) ([]byte, error) {
		return []byte(`[{"full_name":"bad repo"}]`), nil
	})

	if result := listGitHubOrgTargets("acme"); result.OK {
		t.Fatal("expected invalid repository target error")
	}
}

func TestResolveSecurityTargets_Good_RegistryPath(t *testing.T) {
	dir := t.TempDir()
	registryPath := core.PathJoin(dir, "repos.yaml")
	if r := core.WriteFile(registryPath, []byte(`
version: 1
org: acme
base_path: `+dir+`
repos:
  api:
    type: module
    description: API
  web:
    type: module
    description: Web
`), 0o644); !r.OK {
		t.Fatalf("write registry: %v", r.Error())
	}

	result := resolveSecurityTargets(registryPath, "api", "")
	if !result.OK {
		t.Fatalf("resolveSecurityTargets registry: %s", result.Error())
	}
	got := result.Value.([]SecurityTarget)
	want := []SecurityTarget{{DisplayName: "api", FullName: "acme/api"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveSecurityTargets registry = %+v, want %+v", got, want)
	}
}

func TestResolveSecurityTargets_Bad_RegistryRepoMissing(t *testing.T) {
	dir := t.TempDir()
	registryPath := core.PathJoin(dir, "repos.yaml")
	if r := core.WriteFile(registryPath, []byte(`
version: 1
org: acme
base_path: `+dir+`
repos:
  api:
    type: module
`), 0o644); !r.OK {
		t.Fatalf("write registry: %v", r.Error())
	}

	if result := resolveSecurityTargets(registryPath, "missing", ""); result.OK {
		t.Fatal("expected missing registry repo error")
	}
}
