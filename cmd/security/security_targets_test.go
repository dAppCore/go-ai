package security

import (
	"reflect"
	"testing"

	"dappco.re/go/core/scm/repos"
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
