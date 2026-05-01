package security

import (
	"reflect"
	"testing"

	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/scm/repos"
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

func TestAlertSummaryString_Good(t *testing.T) {
	summary := &AlertSummary{
		Critical: 1,
		High:     2,
		Medium:   3,
		Low:      4,
		Unknown:  5,
	}

	got := summary.String()
	want := core.Join(" | ",
		cli.ErrorStyle.Render("1 critical"),
		cli.WarningStyle.Render("2 high"),
		cli.ValueStyle.Render("3 medium"),
		cli.DimStyle.Render("4 low"),
		cli.DimStyle.Render("5 unknown"),
	)
	if got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
}

func TestAlertSummaryString_Bad_EmptySummaryReturnsNoAlerts(t *testing.T) {
	if got := (&AlertSummary{}).String(); got != cli.SuccessStyle.Render("No alerts") {
		t.Fatalf("String() on empty summary = %q, want no-alerts indicator", got)
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
	originalCallGitHubAPIRequest := callGitHubAPIRequest
	t.Cleanup(func() {
		callGitHubAPIRequest = originalCallGitHubAPIRequest
	})

	callGitHubAPIRequest = func(endpoint string) core.Result {
		if endpoint != "orgs/acme/repos?per_page=100&type=all" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return core.Ok([]byte(`[[{"full_name":"acme/api"},{"full_name":"acme/web"}]]`))
	}

	reg := &repos.Registry{
		Org: "acme",
		Repos: map[string]*repos.Repo{
			"api": {Name: "api"},
			"web": {Name: "web"},
		},
	}

	result := resolveJobTargets("all", reg)
	if !result.OK {
		t.Fatalf("resolveJobTargets(all): %s", result.Error())
	}
	got := result.Value.([]string)

	want := []string{"acme/api", "acme/web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveJobTargets(all) = %v, want %v", got, want)
	}
}

func TestResolveJobTargets_Bad_AllFailsClosedWhenGitHubUnavailable(t *testing.T) {
	originalCallGitHubAPIRequest := callGitHubAPIRequest
	t.Cleanup(func() {
		callGitHubAPIRequest = originalCallGitHubAPIRequest
	})

	callGitHubAPIRequest = func(string) core.Result {
		return core.Fail(assertiveError("github unavailable"))
	}

	reg := &repos.Registry{
		Org: "acme",
		Repos: map[string]*repos.Repo{
			"api": {Name: "api"},
			"web": {Name: "web"},
		},
	}

	if result := resolveJobTargets("all", reg); result.OK {
		t.Fatal("expected resolveJobTargets(all) to fail closed when GitHub enumeration is unavailable")
	}
}

func TestResolveJobTargets_Good_MixedAndDeduped(t *testing.T) {
	reg := &repos.Registry{
		Org: "acme",
		Repos: map[string]*repos.Repo{
			"api": {Name: "api"},
		},
	}

	result := resolveJobTargets("api, acme/api, acme/worker, api", reg)
	if !result.OK {
		t.Fatalf("resolveJobTargets(mixed): %s", result.Error())
	}
	got := result.Value.([]string)

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

	if result := resolveJobTargets("missing", reg); result.OK {
		t.Fatal("expected unknown repo error, got nil")
	}
}

func TestCmdJobs_resolveJobTargetsForDryRun_Good_ExpandsAndDedupesRegistryTargets(t *testing.T) {
	reg := &repos.Registry{
		Org: "acme",
		Repos: map[string]*repos.Repo{
			"api":  {Name: "api"},
			"web":  {Name: "web"},
			"docs": {Name: "docs"},
		},
	}

	result := resolveJobTargetsForDryRun("api, acme/web, api, acme/docs", reg)
	if !result.OK {
		t.Fatalf("resolveJobTargetsForDryRun: %s", result.Error())
	}
	got := result.Value.([]string)

	want := []string{"acme/api", "acme/web", "acme/docs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveJobTargetsForDryRun = %v, want %v", got, want)
	}
}

func TestCmdJobs_resolveJobTargetsForDryRun_Bad_RequiresRegistryForAllAndShortNames(t *testing.T) {
	tests := []struct {
		name     string
		targets  string
		registry *repos.Registry
	}{
		{name: "all without registry", targets: "all", registry: nil},
		{name: "short name without registry", targets: "api", registry: nil},
		{name: "missing repo", targets: "missing", registry: &repos.Registry{Org: "acme", Repos: map[string]*repos.Repo{}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if result := resolveJobTargetsForDryRun(tc.targets, tc.registry); result.OK {
				t.Fatalf("expected resolveJobTargetsForDryRun(%q) to fail", tc.targets)
			}
		})
	}
}

func TestCmdJobs_resolveJobTargetsForDryRun_Ugly_RejectsBlankTargets(t *testing.T) {
	if result := resolveJobTargetsForDryRun("   ", &repos.Registry{Org: "acme"}); result.OK {
		t.Fatal("expected blank --targets to fail")
	}
}

func TestNormalizeJobWorkerCount_Good(t *testing.T) {
	if got := normalizeJobWorkerCount(1, 10); got != 1 {
		t.Fatalf("normalizeJobWorkerCount(1, 10) = %d, want 1", got)
	}
	if got := normalizeJobWorkerCount(10, 2); got != 2 {
		t.Fatalf("normalizeJobWorkerCount(10, 2) = %d, want 2", got)
	}
	if got := normalizeJobWorkerCount(100, 100); got != maxSecurityJobWorkers {
		t.Fatalf("normalizeJobWorkerCount(100, 100) = %d, want %d", got, maxSecurityJobWorkers)
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

	collectDependabotAlertsForJobs = func(target SecurityTarget, severityFilter string) core.Result {
		if target.FullName != "acme/api" || severityFilter != "" {
			t.Fatalf("unexpected dependabot target/filter: %+v %q", target, severityFilter)
		}
		return core.Ok([]DepAlert{{
			Repo:     "api",
			Severity: "critical",
			CVE:      "CVE-2026-0001",
			Summary:  "Upgrade OpenSSL",
		}})
	}
	collectCodeScanningAlertsForJobs = func(target SecurityTarget, commandOptions ScanCommandOptions) core.Result {
		if target.FullName != "acme/api" || commandOptions.Selection.SeverityFilter != "" || commandOptions.ToolName != "" {
			t.Fatalf("unexpected code scanning target/options: %+v %+v", target, commandOptions)
		}
		return core.Ok([]ScanAlert{{
			Repo:        "api",
			Severity:    "medium",
			RuleID:      "gosec/G401",
			Path:        "main.go",
			Line:        14,
			Description: "Potential weak crypto",
		}})
	}
	collectSecretScanningAlertsForJobs = func(target SecurityTarget) core.Result {
		if target.FullName != "acme/api" {
			t.Fatalf("unexpected secret scanning target: %+v", target)
		}
		return core.Ok([]SecretAlert{{
			Repo:       "api",
			Number:     9,
			SecretType: "aws_access_key",
		}})
	}

	result := collectJobRepoResult("acme/api")
	if !result.OK {
		t.Fatalf("collectJobRepoResult: %s", result.Error())
	}
	got := result.Value.(jobRepoResult)

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

	collectDependabotAlertsForJobs = func(SecurityTarget, string) core.Result {
		return core.Fail(assertiveError("dependabot failed"))
	}
	collectCodeScanningAlertsForJobs = func(SecurityTarget, ScanCommandOptions) core.Result {
		return core.Fail(assertiveError("code scanning failed"))
	}
	collectSecretScanningAlertsForJobs = func(SecurityTarget) core.Result {
		return core.Fail(assertiveError("secret scanning failed"))
	}

	if result := collectJobRepoResult("acme/api"); result.OK {
		t.Fatal("expected all-collectors-failed error, got nil")
	}
}

func TestCollectJobRepoResult_Bad_PartialFailureFailsClosed(t *testing.T) {
	originalCollectDependabotAlertsForJobs := collectDependabotAlertsForJobs
	originalCollectCodeScanningAlertsForJobs := collectCodeScanningAlertsForJobs
	originalCollectSecretScanningAlertsForJobs := collectSecretScanningAlertsForJobs
	t.Cleanup(func() {
		collectDependabotAlertsForJobs = originalCollectDependabotAlertsForJobs
		collectCodeScanningAlertsForJobs = originalCollectCodeScanningAlertsForJobs
		collectSecretScanningAlertsForJobs = originalCollectSecretScanningAlertsForJobs
	})

	collectDependabotAlertsForJobs = func(SecurityTarget, string) core.Result {
		return core.Ok([]DepAlert{{Repo: "api", Severity: "critical", CVE: "CVE-1", Summary: "dep"}})
	}
	collectCodeScanningAlertsForJobs = func(SecurityTarget, ScanCommandOptions) core.Result {
		return core.Fail(core.NewError("code scanning unavailable"))
	}
	collectSecretScanningAlertsForJobs = func(SecurityTarget) core.Result {
		return core.Ok([]SecretAlert{{Repo: "api", Number: 1, SecretType: "token"}})
	}

	if result := collectJobRepoResult("acme/api"); result.OK {
		t.Fatal("expected collectJobRepoResult to fail closed on partial collector failure")
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

func TestMergeAlertSummary_Good(t *testing.T) {
	dst := &AlertSummary{Critical: 1, Total: 1}
	src := &AlertSummary{High: 2, Low: 1, Total: 3}

	mergeAlertSummary(dst, src)

	if dst.Critical != 1 || dst.High != 2 || dst.Low != 1 || dst.Total != 4 {
		t.Fatalf("unexpected merged summary: %+v", dst)
	}
}

func TestBuildJobsIssueBody_Good_TruncatesFindingsAfterThree(t *testing.T) {
	body := buildJobsIssueBody(
		&AlertSummary{Critical: 1, High: 2, Total: 3},
		[]jobRepoResult{{
			Repo: "acme/api",
			Summary: AlertSummary{
				Critical: 1,
				High:     1,
				Total:    2,
			},
			Findings: []string{"one", "two", "three", "four"},
		}},
	)

	if !core.Contains(body, "## Security Scan Summary") || !core.Contains(body, "Summary: 1 critical | 2 high") {
		t.Fatalf("issue body missing summary text: %s", body)
	}
	if !core.Contains(body, "- acme/api — 1 critical | 1 high") {
		t.Fatalf("issue body missing repo summary: %s", body)
	}
	if !core.Contains(body, "  - ...") {
		t.Fatalf("issue body should truncate findings after three entries: %s", body)
	}
}

func TestRunJobWorkers_Good_SortsResults(t *testing.T) {
	originalCollectDependabotAlertsForJobs := collectDependabotAlertsForJobs
	originalCollectCodeScanningAlertsForJobs := collectCodeScanningAlertsForJobs
	originalCollectSecretScanningAlertsForJobs := collectSecretScanningAlertsForJobs
	t.Cleanup(func() {
		collectDependabotAlertsForJobs = originalCollectDependabotAlertsForJobs
		collectCodeScanningAlertsForJobs = originalCollectCodeScanningAlertsForJobs
		collectSecretScanningAlertsForJobs = originalCollectSecretScanningAlertsForJobs
	})

	collectDependabotAlertsForJobs = func(target SecurityTarget, _ string) core.Result {
		return core.Ok([]DepAlert{{Repo: target.DisplayName, Severity: "high", CVE: "CVE-1", Summary: "dep"}})
	}
	collectCodeScanningAlertsForJobs = func(target SecurityTarget, _ ScanCommandOptions) core.Result {
		return core.Ok([]ScanAlert{{Repo: target.DisplayName, Severity: "medium", RuleID: "R-1", Description: "scan"}})
	}
	collectSecretScanningAlertsForJobs = func(target SecurityTarget) core.Result {
		return core.Ok([]SecretAlert{{Repo: target.DisplayName, Number: 1, SecretType: "token"}})
	}

	results := runJobWorkers([]string{"acme/web", "acme/api"}, 2)
	if len(results) != 2 {
		t.Fatalf("runJobWorkers len = %d, want 2", len(results))
	}
	if results[0].repo.Repo != "acme/api" || results[1].repo.Repo != "acme/web" {
		t.Fatalf("runJobWorkers should sort results by repo: %+v", results)
	}
}

func TestRunJobs_Good_DryRunPrintsPlannedTargets(t *testing.T) {
	withSecurityTempHome(t)

	output := captureStdout(t, func() {
		if r := runJobs(JobsCommandOptions{
			Targets:     "acme/api, acme/web",
			DryRun:      true,
			WorkerCount: 4,
		}); !r.OK {
			t.Fatalf("runJobs dry-run: %s", r.Error())
		}
	})

	if !core.Contains(output, "Workers:") || !core.Contains(output, "[dry-run] Would scan: acme/api") || !core.Contains(output, "[dry-run] Would scan: acme/web") {
		t.Fatalf("dry-run output missing planned targets: %s", output)
	}
}

func TestRunJobs_Bad_InvalidWorkerCount(t *testing.T) {
	if r := runJobs(JobsCommandOptions{Targets: "acme/api", WorkerCount: 0}); r.OK {
		t.Fatal("expected invalid worker count error")
	}
}

func TestRunJobs_Bad_PartialFailureFailsClosedBeforeIssueCreation(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubScript(t, "#!/bin/sh\nif [ \"$1\" = issue ]; then\n  echo unexpected issue create >&2\n  exit 99\nfi\nexit 0\n")

	originalCollectDependabotAlertsForJobs := collectDependabotAlertsForJobs
	originalCollectCodeScanningAlertsForJobs := collectCodeScanningAlertsForJobs
	originalCollectSecretScanningAlertsForJobs := collectSecretScanningAlertsForJobs
	t.Cleanup(func() {
		collectDependabotAlertsForJobs = originalCollectDependabotAlertsForJobs
		collectCodeScanningAlertsForJobs = originalCollectCodeScanningAlertsForJobs
		collectSecretScanningAlertsForJobs = originalCollectSecretScanningAlertsForJobs
	})

	collectDependabotAlertsForJobs = func(target SecurityTarget, _ string) core.Result {
		if target.FullName == "acme/web" {
			return core.Fail(core.NewError("dependabot unavailable"))
		}
		return core.Ok([]DepAlert{{Repo: target.DisplayName, Severity: "high", CVE: "CVE-1", Summary: "dep"}})
	}
	collectCodeScanningAlertsForJobs = func(target SecurityTarget, _ ScanCommandOptions) core.Result {
		return core.Ok([]ScanAlert{{Repo: target.DisplayName, Severity: "medium", RuleID: "R-1", Description: "scan"}})
	}
	collectSecretScanningAlertsForJobs = func(target SecurityTarget) core.Result {
		return core.Ok([]SecretAlert{{Repo: target.DisplayName, Number: 1, SecretType: "token"}})
	}

	r := runJobs(JobsCommandOptions{
		Targets:         "acme/api,acme/web",
		IssueRepository: "acme/security",
		WorkerCount:     2,
	})
	if r.OK {
		t.Fatal("expected partial failure to abort jobs")
	}
	if !core.Contains(r.Error(), "security jobs failed") || !core.Contains(r.Error(), "acme/web") {
		t.Fatalf("unexpected error: %s", r.Error())
	}
}

func TestCreateJobsIssue_Good_ReturnsTrimmedOutput(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf 'https://github.com/acme/security/issues/1\\n'\n")

	result := createJobsIssue("acme/security", "Security scan summary", "body")
	if !result.OK {
		t.Fatalf("createJobsIssue: %s", result.Error())
	}
	got := result.Value.(string)
	if got != "https://github.com/acme/security/issues/1" {
		t.Fatalf("createJobsIssue = %q, want trimmed issue URL", got)
	}
}

func TestRunJobs_Good_DryRunDoesNotRequireGitHubCLI(t *testing.T) {
	originalCallGitHubAPIRequest := callGitHubAPIRequest
	t.Cleanup(func() {
		callGitHubAPIRequest = originalCallGitHubAPIRequest
	})

	callGitHubAPIRequest = func(string) core.Result {
		t.Fatal("dry-run should not invoke GitHub API helpers")
		return core.Ok(nil)
	}

	if r := runJobs(JobsCommandOptions{
		Targets:     "acme/api",
		DryRun:      true,
		WorkerCount: 1,
	}); !r.OK {
		t.Fatalf("runJobs dry-run: %s", r.Error())
	}
}

func TestRunJobs_Bad_EmptyTargetsFailsBeforeRegistryLookup(t *testing.T) {
	r := runJobs(JobsCommandOptions{
		Targets:     "",
		DryRun:      true,
		WorkerCount: 1,
	})
	if r.OK {
		t.Fatal("expected empty --targets error, got nil")
	}
	if !core.Contains(r.Error(), "--targets") {
		t.Fatalf("expected --targets validation error, got %s", r.Error())
	}
}

func TestValidateJobsIssueRepository_Good(t *testing.T) {
	result := validateJobsIssueRepository("acme/security")
	if !result.OK {
		t.Fatalf("validateJobsIssueRepository: %s", result.Error())
	}
	got := result.Value.(SecurityTarget)

	want := SecurityTarget{DisplayName: "security", FullName: "acme/security"}
	if got != want {
		t.Fatalf("validateJobsIssueRepository = %+v, want %+v", got, want)
	}
}

func TestValidateJobsIssueRepository_Bad(t *testing.T) {
	if result := validateJobsIssueRepository("bad repo"); result.OK {
		t.Fatal("expected invalid issue repo error")
	}
}

func TestValidateJobsIssueRepository_Ugly_BlankInputReturnsZeroTarget(t *testing.T) {
	result := validateJobsIssueRepository("")
	if !result.OK {
		t.Fatalf("validateJobsIssueRepository blank: %s", result.Error())
	}
	got := result.Value.(SecurityTarget)
	if got != (SecurityTarget{}) {
		t.Fatalf("validateJobsIssueRepository blank = %+v, want zero target", got)
	}
}

func TestJobsNeedRegistry_Good(t *testing.T) {
	tests := []struct {
		name    string
		targets string
		want    bool
	}{
		{name: "empty", targets: "", want: true},
		{name: "all", targets: "all", want: true},
		{name: "short-name", targets: "api, acme/web", want: true},
		{name: "fully-qualified", targets: "acme/api, acme/web", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := jobsNeedRegistry(tc.targets); got != tc.want {
				t.Fatalf("jobsNeedRegistry(%q) = %v, want %v", tc.targets, got, tc.want)
			}
		})
	}
}

func TestLoadRegistryForJobs_Good_LoadsRegistryWhenNeeded(t *testing.T) {
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
`), 0o644); !r.OK {
		t.Fatalf("write registry: %v", r.Error())
	}

	result := loadRegistryForJobs(JobsCommandOptions{
		RegistryPath: registryPath,
		Targets:      "api",
	})
	if !result.OK {
		t.Fatalf("loadRegistryForJobs: %s", result.Error())
	}
	registry := result.Value.(*repos.Registry)
	if registry == nil || registry.Org != "acme" {
		t.Fatalf("loadRegistryForJobs returned %+v, want acme registry", registry)
	}
}

func TestLoadRegistryForJobs_Ugly_SkipsRegistryForFullyQualifiedTargets(t *testing.T) {
	result := loadRegistryForJobs(JobsCommandOptions{
		RegistryPath: "ignored.yaml",
		Targets:      "acme/api, acme/web",
	})
	if !result.OK {
		t.Fatalf("loadRegistryForJobs: %s", result.Error())
	}
	registry, _ := result.Value.(*repos.Registry)
	if registry != nil {
		t.Fatalf("loadRegistryForJobs returned %+v, want nil for fully-qualified targets", registry)
	}
}

func TestRunJobs_Ugly_InvalidIssueRepoRejectsBeforeGitHubCLI(t *testing.T) {
	t.Setenv("PATH", "")

	r := runJobs(JobsCommandOptions{
		Targets:         "acme/api",
		IssueRepository: "bad repo",
		WorkerCount:     1,
	})
	if r.OK {
		t.Fatal("expected invalid issue repository to fail")
	}
	if !core.Contains(r.Error(), "invalid --issue-repo format") {
		t.Fatalf("unexpected error: %s", r.Error())
	}
}

type assertiveError string

func (e assertiveError) Error() string {
	return string(e)
}
