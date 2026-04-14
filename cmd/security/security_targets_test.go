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
	for _, input := range []string{"", "wailsapp", "/wails", "wailsapp/"} {
		if _, err := parseSecurityTarget(input); err == nil {
			t.Fatalf("expected error for %q, got nil", input)
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
