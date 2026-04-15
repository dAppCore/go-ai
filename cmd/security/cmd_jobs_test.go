package security

import (
	"reflect"
	"strings"
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

func TestBuildAlertOutputs_Good_CombinesSpecializedOutputs(t *testing.T) {
	got := buildAlertOutputs(
		[]DepAlert{{
			Repo:       "api",
			Severity:   "critical",
			CVE:        "CVE-2026-0001",
			Package:    "openssl",
			Vulnerable: "< 1.0.0",
			Summary:    "Upgrade OpenSSL",
		}},
		[]ScanAlert{{
			Repo:     "api",
			Severity: "medium",
			RuleID:   "gosec/G401",
			Path:     "main.go",
			Line:     14,
			Message:  "Potential weak crypto",
		}},
		[]SecretAlert{{
			Repo:       "api",
			Number:     9,
			SecretType: "aws_access_key",
		}},
		"",
	)

	if len(got) != 3 {
		t.Fatalf("buildAlertOutputs count = %d, want 3", len(got))
	}
	if got[0].Type != "dependabot" || got[0].ID != "CVE-2026-0001" {
		t.Fatalf("unexpected dependabot alert: %+v", got[0])
	}
	if got[1].Type != "code-scanning" || got[1].Location != "main.go:14" {
		t.Fatalf("unexpected code scanning alert: %+v", got[1])
	}
	if got[2].Type != "secret-scanning" || got[2].Severity != "high" {
		t.Fatalf("unexpected secret alert: %+v", got[2])
	}
}

func TestBuildAlertOutputs_Good_HidesSecretsWhenSeverityFilterExcludesHigh(t *testing.T) {
	got := buildAlertOutputs(nil, nil, []SecretAlert{{
		Repo:       "api",
		Number:     9,
		SecretType: "aws_access_key",
	}}, "critical")

	if len(got) != 0 {
		t.Fatalf("buildAlertOutputs with critical filter = %d alerts, want 0", len(got))
	}
}

func TestResolveJobTargets_Good_All(t *testing.T) {
	originalRunGitHubAPIRequest := runGitHubAPIRequest
	t.Cleanup(func() {
		runGitHubAPIRequest = originalRunGitHubAPIRequest
	})

	runGitHubAPIRequest = func(endpoint string) ([]byte, error) {
		if endpoint != "orgs/acme/repos?per_page=100&type=all" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(`[[{"full_name":"acme/api"},{"full_name":"acme/web"}]]`), nil
	}

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

func TestResolveJobTargets_Good_AllFallsBackToRegistryWhenGitHubUnavailable(t *testing.T) {
	originalRunGitHubAPIRequest := runGitHubAPIRequest
	t.Cleanup(func() {
		runGitHubAPIRequest = originalRunGitHubAPIRequest
	})

	runGitHubAPIRequest = func(string) ([]byte, error) {
		return nil, assertiveError("github unavailable")
	}

	reg := &repos.Registry{
		Org: "acme",
		Repos: map[string]*repos.Repo{
			"api": {Name: "api"},
			"web": {Name: "web"},
		},
	}

	got, err := resolveJobTargets("all", reg)
	if err != nil {
		t.Fatalf("resolveJobTargets(all) fallback: %v", err)
	}

	want := []string{"acme/api", "acme/web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveJobTargets(all) fallback = %v, want %v", got, want)
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

func TestCollectJobRepoResult_Good_UsesSharedCollectors(t *testing.T) {
	originalCollectDependabotAlertsForJobs := collectDependabotAlertsForJobs
	originalCollectCodeScanningAlertsForJobs := collectCodeScanningAlertsForJobs
	originalCollectSecretScanningAlertsForJobs := collectSecretScanningAlertsForJobs
	t.Cleanup(func() {
		collectDependabotAlertsForJobs = originalCollectDependabotAlertsForJobs
		collectCodeScanningAlertsForJobs = originalCollectCodeScanningAlertsForJobs
		collectSecretScanningAlertsForJobs = originalCollectSecretScanningAlertsForJobs
	})

	collectDependabotAlertsForJobs = func(target SecurityTarget, severityFilter string) ([]DepAlert, error) {
		if target.FullName != "acme/api" || severityFilter != "" {
			t.Fatalf("unexpected dependabot target/filter: %+v %q", target, severityFilter)
		}
		return []DepAlert{{
			Repo:     "api",
			Severity: "critical",
			CVE:      "CVE-2026-0001",
			Summary:  "Upgrade OpenSSL",
		}}, nil
	}
	collectCodeScanningAlertsForJobs = func(target SecurityTarget, commandOptions ScanCommandOptions) ([]ScanAlert, error) {
		if target.FullName != "acme/api" || commandOptions.Selection.SeverityFilter != "" || commandOptions.ToolName != "" {
			t.Fatalf("unexpected code scanning target/options: %+v %+v", target, commandOptions)
		}
		return []ScanAlert{{
			Repo:        "api",
			Severity:    "medium",
			RuleID:      "gosec/G401",
			Path:        "main.go",
			Line:        14,
			Description: "Potential weak crypto",
		}}, nil
	}
	collectSecretScanningAlertsForJobs = func(target SecurityTarget) ([]SecretAlert, error) {
		if target.FullName != "acme/api" {
			t.Fatalf("unexpected secret scanning target: %+v", target)
		}
		return []SecretAlert{{
			Repo:       "api",
			Number:     9,
			SecretType: "aws_access_key",
		}}, nil
	}

	got, err := collectJobRepoResult("acme/api")
	if err != nil {
		t.Fatalf("collectJobRepoResult: %v", err)
	}

	if got.Repo != "acme/api" {
		t.Fatalf("repo = %q, want acme/api", got.Repo)
	}
	if got.Summary.Critical != 1 || got.Summary.High != 1 || got.Summary.Medium != 1 || got.Summary.Total != 3 {
		t.Fatalf("unexpected summary: %+v", got.Summary)
	}
	if len(got.Findings) != 3 {
		t.Fatalf("findings = %d, want 3", len(got.Findings))
	}
}

func TestCollectJobRepoResult_Bad_AllCollectorsFail(t *testing.T) {
	originalCollectDependabotAlertsForJobs := collectDependabotAlertsForJobs
	originalCollectCodeScanningAlertsForJobs := collectCodeScanningAlertsForJobs
	originalCollectSecretScanningAlertsForJobs := collectSecretScanningAlertsForJobs
	t.Cleanup(func() {
		collectDependabotAlertsForJobs = originalCollectDependabotAlertsForJobs
		collectCodeScanningAlertsForJobs = originalCollectCodeScanningAlertsForJobs
		collectSecretScanningAlertsForJobs = originalCollectSecretScanningAlertsForJobs
	})

	collectDependabotAlertsForJobs = func(SecurityTarget, string) ([]DepAlert, error) {
		return nil, assertiveError("dependabot failed")
	}
	collectCodeScanningAlertsForJobs = func(SecurityTarget, ScanCommandOptions) ([]ScanAlert, error) {
		return nil, assertiveError("code scanning failed")
	}
	collectSecretScanningAlertsForJobs = func(SecurityTarget) ([]SecretAlert, error) {
		return nil, assertiveError("secret scanning failed")
	}

	if _, err := collectJobRepoResult("acme/api"); err == nil {
		t.Fatal("expected all-collectors-failed error, got nil")
	}
}

func TestBuildJobsMetricsEvent_Good_IssueRepoWins(t *testing.T) {
	event := buildJobsMetricsEvent(
		JobsCommandOptions{
			Targets:         "all",
			IssueRepository: "acme/security",
		},
		&AlertSummary{
			Critical: 1,
			High:     2,
			Unknown:  1,
			Total:    4,
		},
		[]jobRepoResult{
			{Repo: "acme/api"},
			{Repo: "acme/web"},
		},
		"https://github.com/acme/security/issues/1",
	)

	if event.Type != "security.jobs" {
		t.Fatalf("event type = %q, want security.jobs", event.Type)
	}
	if event.Repo != "acme/security" {
		t.Fatalf("event repo = %q, want acme/security", event.Repo)
	}

	reposValue, ok := event.Data["repos"].([]string)
	if !ok {
		t.Fatalf("event repos = %T, want []string", event.Data["repos"])
	}
	if want := []string{"acme/api", "acme/web"}; !reflect.DeepEqual(reposValue, want) {
		t.Fatalf("event repos = %v, want %v", reposValue, want)
	}
	if event.Data["issue_url"] != "https://github.com/acme/security/issues/1" {
		t.Fatalf("event issue_url = %v", event.Data["issue_url"])
	}
	if event.Data["unknown"] != 1 {
		t.Fatalf("event unknown count = %v, want 1", event.Data["unknown"])
	}
}

func TestRunJobs_Good_DryRunDoesNotRequireGitHubCLI(t *testing.T) {
	originalRunGitHubAPIRequest := runGitHubAPIRequest
	t.Cleanup(func() {
		runGitHubAPIRequest = originalRunGitHubAPIRequest
	})

	runGitHubAPIRequest = func(string) ([]byte, error) {
		t.Fatal("dry-run should not invoke GitHub API helpers")
		return nil, nil
	}

	if err := runJobs(JobsCommandOptions{
		Targets:     "acme/api",
		DryRun:      true,
		WorkerCount: 1,
	}); err != nil {
		t.Fatalf("runJobs dry-run: %v", err)
	}
}

func TestRunJobs_Bad_EmptyTargetsFailsBeforeRegistryLookup(t *testing.T) {
	err := runJobs(JobsCommandOptions{
		Targets:     "",
		DryRun:      true,
		WorkerCount: 1,
	})
	if err == nil {
		t.Fatal("expected empty --targets error, got nil")
	}
	if !strings.Contains(err.Error(), "--targets") {
		t.Fatalf("expected --targets validation error, got %v", err)
	}
}

type assertiveError string

func (e assertiveError) Error() string {
	return string(e)
}
