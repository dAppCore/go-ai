package security

import (
	"reflect"
	"testing"

	"dappco.re/go/core/scm/repos"
)

func TestAlertSummaryPlainString_Good(t *testing.T) {
	summary := &AlertSummary{}
	summary.Add("critical")
	summary.Add("high")
	summary.Add("medium")
	summary.Add("low")
	summary.Add("weird")

	got := summary.PlainString()
	want := "1 critical | 1 high | 1 medium | 1 low | 1 unknown"
	if got != want {
		t.Fatalf("PlainString = %q, want %q", got, want)
	}
}

func TestResolveJobTargets_Good_All(t *testing.T) {
	reg := &repos.Registry{
		Org: "acme",
		Repos: map[string]*repos.Repo{
			"api": {Name: "api"},
			"web": {Name: "web"},
		},
	}

	got, err := resolveJobTargets("all", reg)
	if err != nil {
		t.Fatalf("resolveJobTargets(all): %v", err)
	}

	want := []string{"acme/api", "acme/web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveJobTargets(all) = %v, want %v", got, want)
	}
}

func TestResolveJobTargets_Good_MixedAndDeduped(t *testing.T) {
	reg := &repos.Registry{
		Org: "acme",
		Repos: map[string]*repos.Repo{
			"api": {Name: "api"},
		},
	}

	got, err := resolveJobTargets("api, acme/api, acme/worker, api", reg)
	if err != nil {
		t.Fatalf("resolveJobTargets(mixed): %v", err)
	}

	want := []string{"acme/api", "acme/worker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveJobTargets(mixed) = %v, want %v", got, want)
	}
}

func TestResolveJobTargets_Bad_UnknownRepo(t *testing.T) {
	reg := &repos.Registry{
		Org:   "acme",
		Repos: map[string]*repos.Repo{},
	}

	if _, err := resolveJobTargets("missing", reg); err == nil {
		t.Fatal("expected unknown repo error, got nil")
	}
}
