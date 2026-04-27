package security

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"dappco.re/go/scm/repos"
)

func TestParseSecurityTarget_Good(t *testing.T) {
	got, err := parseSecurityTarget("wailsapp/wails")
	if err != nil {
		t.Fatalf("parseSecurityTarget: %v", err)
	}

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
		if _, err := parseSecurityTarget(input); err == nil {
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
	got, err := resolveSecurityTargets("", "", "acme/api")
	if err != nil {
		t.Fatalf("resolveSecurityTargets: %v", err)
	}
	want := []SecurityTarget{{DisplayName: "api", FullName: "acme/api"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveSecurityTargets = %+v, want %+v", got, want)
	}
}

func TestResolveSecurityTargets_Bad_InvalidExternalTarget(t *testing.T) {
	if _, err := resolveSecurityTargets("", "", "acme api"); err == nil {
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

	got, err := listGitHubOrgTargets("acme")
	if err != nil {
		t.Fatalf("listGitHubOrgTargets: %v", err)
	}
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

	if _, err := listGitHubOrgTargets("bad org"); err == nil {
		t.Fatal("expected invalid org to fail")
	}
}

func TestListGitHubOrgTargets_Bad_InvalidRepositoryReturnedByGitHub(t *testing.T) {
	stubGitHubAPI(t, func(string) ([]byte, error) {
		return []byte(`[{"full_name":"bad repo"}]`), nil
	})

	if _, err := listGitHubOrgTargets("acme"); err == nil {
		t.Fatal("expected invalid repository target error")
	}
}

func TestResolveSecurityTargets_Good_RegistryPath(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(registryPath, []byte(`
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
`), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	got, err := resolveSecurityTargets(registryPath, "api", "")
	if err != nil {
		t.Fatalf("resolveSecurityTargets registry: %v", err)
	}
	want := []SecurityTarget{{DisplayName: "api", FullName: "acme/api"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveSecurityTargets registry = %+v, want %+v", got, want)
	}
}

func TestResolveSecurityTargets_Bad_RegistryRepoMissing(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(registryPath, []byte(`
version: 1
org: acme
base_path: `+dir+`
repos:
  api:
    type: module
`), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	if _, err := resolveSecurityTargets(registryPath, "missing", ""); err == nil {
		t.Fatal("expected missing registry repo error")
	}
}
