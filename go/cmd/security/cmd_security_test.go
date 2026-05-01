package security

import (
	"reflect"
	"testing"
	"time"

	core "dappco.re/go"
	"dappco.re/go/ai/ai"
	"dappco.re/go/cli/pkg/cli"
)

func TestCmdSecurity_decodeGitHubArrayItems_Good(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{name: "empty", input: []byte("[]"), want: 0},
		{name: "flat", input: []byte(`[{"id":1},{"id":2}]`), want: 2},
		{name: "paged", input: []byte(`[[{"id":1},{"id":2}],[{"id":3}]]`), want: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := decodeGitHubArrayItems(tc.input)
			if !result.OK {
				t.Fatalf("decodeGitHubArrayItems: %s", result.Error())
			}
			got := result.Value.([]githubRawMessage)
			if len(got) != tc.want {
				t.Fatalf("decodeGitHubArrayItems(%s) len = %d, want %d", tc.name, len(got), tc.want)
			}
		})
	}
}

func TestCmdSecurity_decodeGitHubArrayItems_Bad(t *testing.T) {
	for _, input := range [][]byte{
		[]byte(`{"not":"an array"}`),
		[]byte(`[[{"id":1},bad]]`),
	} {
		if result := decodeGitHubArrayItems(input); result.OK {
			t.Fatalf("expected error for %s", input)
		}
	}
}

func TestCmdSecurity_decodeDependabotAlerts_Good(t *testing.T) {
	result := decodeDependabotAlerts([]byte(`[
		[{
			"number": 7,
			"state": "open",
			"security_advisory": {
				"severity": "high",
				"cve_id": "CVE-2026-0001",
				"summary": "Upgrade OpenSSL",
				"description": "OpenSSL needs updating"
			},
			"dependency": {
				"package": {"name": "openssl", "ecosystem": "npm"},
				"manifest_path": "package.json"
			},
			"security_vulnerability": {
				"package": {"name": "openssl", "ecosystem": "npm"},
				"first_patched_version": {"identifier": "1.0.2"},
				"vulnerable_version_range": "< 1.0.2"
			}
		}]
	]`))
	if !result.OK {
		t.Fatalf("decodeDependabotAlerts: %s", result.Error())
	}
	alerts := result.Value.([]DependabotAlert)
	if len(alerts) != 1 || alerts[0].Advisory.CVEID != "CVE-2026-0001" || alerts[0].SecurityVulnerability.FirstPatchedVersion.Identifier != "1.0.2" {
		t.Fatalf("unexpected dependabot alert: %+v", alerts)
	}
}

func TestCmdSecurity_decodeCodeScanningAlerts_Good(t *testing.T) {
	payload := "[[{\"number\":4,\"state\":\"open\",\"rule\":{\"id\":\"gosec/G401\",\"severity\":\"medium\",\"description\":\"Weak crypto\",\"tags\":[\"security\"]},\"tool\":{\"name\":\"CodeQL\",\"version\":\"2.20.0\"},\"most_recent_instance\":{\"location\":{\"\x70ath\":\"main.go\",\"start_line\":14,\"end_line\":14},\"message\":{\"text\":\"Potential weak crypto\"}}}]]"
	result := decodeCodeScanningAlerts([]byte(payload))
	if !result.OK {
		t.Fatalf("decodeCodeScanningAlerts: %s", result.Error())
	}
	alerts := result.Value.([]CodeScanningAlert)
	if len(alerts) != 1 || alerts[0].Rule.ID != "gosec/G401" || alerts[0].MostRecentInstance.Location.Path != "main.go" {
		t.Fatalf("unexpected code scanning alert: %+v", alerts)
	}
}

func TestCmdSecurity_decodeSecretScanningAlerts_Good(t *testing.T) {
	result := decodeSecretScanningAlerts([]byte(`[
		[{
			"number": 9,
			"state": "open",
			"secret_type": "aws_access_key",
			"secret": "AKIA...",
			"push_protection_bypassed": true,
			"resolution": "revoked"
		}]
	]`))
	if !result.OK {
		t.Fatalf("decodeSecretScanningAlerts: %s", result.Error())
	}
	alerts := result.Value.([]SecretScanningAlert)
	if len(alerts) != 1 || !alerts[0].PushProtection || alerts[0].Resolution != "revoked" {
		t.Fatalf("unexpected secret scanning alert: %+v", alerts)
	}
}

func TestCmdSecurity_decodeGitHubRepositoryNames_Good(t *testing.T) {
	result := decodeGitHubRepositoryNames([]byte(`[
		[{"full_name":"acme/z"},{"full_name":"acme/a"},{"full_name":"acme/a"},{"full_name":""}]
	]`))
	if !result.OK {
		t.Fatalf("decodeGitHubRepositoryNames: %s", result.Error())
	}
	names := result.Value.([]string)
	want := []string{"acme/a", "acme/z"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("decodeGitHubRepositoryNames = %v, want %v", names, want)
	}
}

func TestCmdSecurity_buildSecurityMetricsEvent_Good(t *testing.T) {
	startedAt := time.Now().Add(-time.Second)
	event := buildSecurityMetricsEvent("security.alerts", startedAt, "acme/api", map[string]any{
		"total": 3,
	})

	if event.Type != "security.alerts" || event.Repo != "acme/api" {
		t.Fatalf("unexpected event fields: %+v", event)
	}
	if event.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", event.Duration)
	}
	if event.Timestamp.IsZero() {
		t.Fatal("expected event timestamp to be populated")
	}
	if event.Data["total"].(int) != 3 {
		t.Fatalf("unexpected event data: %+v", event.Data)
	}
}

func TestCmdSecurity_severityStyle_Good(t *testing.T) {
	tests := []struct {
		severity string
		want     *cli.AnsiStyle
	}{
		{severity: "critical", want: cli.ErrorStyle},
		{severity: "high", want: cli.WarningStyle},
		{severity: "medium", want: cli.ValueStyle},
		{severity: "low", want: cli.DimStyle},
		{severity: "unknown", want: cli.DimStyle},
	}

	for _, tc := range tests {
		t.Run(tc.severity, func(t *testing.T) {
			if got := severityStyle(tc.severity); got != tc.want {
				t.Fatalf("severityStyle(%q) = %p, want %p", tc.severity, got, tc.want)
			}
		})
	}
}

func TestCmdSecurity_filterBySeverity_Good(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		filter   string
		want     bool
	}{
		{name: "empty filter", severity: "high", filter: "", want: true},
		{name: "exact match", severity: "high", filter: "high", want: true},
		{name: "multi match", severity: "high", filter: "critical,high", want: true},
		{name: "trimmed match", severity: "critical", filter: " low , critical ", want: true},
		{name: "miss", severity: "medium", filter: "critical,high", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := filterBySeverity(tc.severity, tc.filter); got != tc.want {
				t.Fatalf("filterBySeverity(%q, %q) = %v, want %v", tc.severity, tc.filter, got, tc.want)
			}
		})
	}
}

func TestCmdSecurity_AlertSummary_Good(t *testing.T) {
	var summary AlertSummary
	if got := summary.String(); !core.Contains(got, "No alerts") {
		t.Fatalf("zero-value summary should report no alerts, got %q", got)
	}
	if got := summary.PlainString(); got != "No alerts" {
		t.Fatalf("zero-value PlainString = %q, want No alerts", got)
	}

	summary.Add("critical")
	summary.Add("high")
	summary.Add("medium")
	summary.Add("low")
	summary.Add("what even is this")

	if summary.Total != 5 || summary.Critical != 1 || summary.High != 1 || summary.Medium != 1 || summary.Low != 1 || summary.Unknown != 1 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if got := summary.PlainString(); got != "1 critical | 1 high | 1 medium | 1 low | 1 unknown" {
		t.Fatalf("PlainString = %q", got)
	}
	if got := normalizeWhitespace(summary.String()); !core.Contains(got, "critical") || !core.Contains(got, "unknown") {
		t.Fatalf("styled summary missing expected severities: %q", got)
	}
}

func TestCmdSecurity_recordSecurityMetricsEvent_Ugly_DoesNotPanic(t *testing.T) {
	withSecurityTempHome(t)

	// recordSecurityMetricsEvent intentionally ignores write errors, so this test
	// only verifies that the wrapper stays no-op from the caller's perspective.
	recordSecurityMetricsEvent(ai.Event{Type: "security.alerts"})
	eventsResult := ai.ReadEvents(time.Now().Add(-time.Minute))
	if !eventsResult.OK {
		t.Fatalf("ReadEvents after recordSecurityMetricsEvent: %s", eventsResult.Error())
	}
	events := eventsResult.Value.([]ai.Event)
	if len(events) != 1 || events[0].Type != "security.alerts" {
		t.Fatalf("unexpected metrics events: %+v", events)
	}
}

func TestCmdSecurity_runGitHubAPI_Good_ReturnsStdout(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '[{\"full_name\":\"acme/api\"}]'\n")

	result := runGitHubAPI("repos/acme/api/dependabot/alerts?state=open")
	if !result.OK {
		t.Fatalf("runGitHubAPI: %s", result.Error())
	}
	got := result.Value.([]byte)
	if string(got) != `[{"full_name":"acme/api"}]` {
		t.Fatalf("runGitHubAPI = %s, want JSON output", string(got))
	}
}

func TestCmdSecurity_runGitHubAPI_Bad_404ReturnsEmptyArray(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '404 Not Found' >&2\nexit 1\n")

	result := runGitHubAPI("repos/acme/api/dependabot/alerts?state=open")
	if !result.OK {
		t.Fatalf("runGitHubAPI 404 should not fail: %s", result.Error())
	}
	got := result.Value.([]byte)
	if string(got) != "[]" {
		t.Fatalf("runGitHubAPI 404 = %s, want []", string(got))
	}
}

func TestCmdSecurity_runGitHubAPIStrict_Bad_DoesNotFallbackOnMissingEndpoint(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '404 Not Found' >&2\nexit 1\n")

	if result := runGitHubAPIStrict("repos/acme/api/dependabot/alerts?state=open"); result.OK {
		t.Fatalf("runGitHubAPIStrict returned %q, want error", string(result.Value.([]byte)))
	}
}

func TestCmdSecurity_runGitHubAPIWithMode_Good_RetriesTransientFailures(t *testing.T) {
	counterFile := core.PathJoin(t.TempDir(), "attempts")
	script := core.Sprintf(`#!/bin/sh
count=0
if [ -f %[1]q ]; then
  count=$(cat %[1]q)
fi
count=$((count + 1))
printf '%%s' "$count" > %[1]q
if [ "$count" -lt 3 ]; then
  printf 'temporary GitHub API failure' >&2
  exit 1
fi
printf '[]'
`, counterFile)
	withFakeGitHubScript(t, script)

	result := runGitHubAPIWithMode("repos/acme/api/dependabot/alerts?state=open", true)
	if !result.OK {
		t.Fatalf("runGitHubAPIWithMode retry path: %s", result.Error())
	}
	got := result.Value.([]byte)
	if string(got) != "[]" {
		t.Fatalf("runGitHubAPIWithMode retry path = %q, want []", string(got))
	}
}

func TestCmdSecurity_runGitHubAPIWithMode_Bad_DoesNotRetryAccessDenied(t *testing.T) {
	counterFile := core.PathJoin(t.TempDir(), "attempts")
	script := core.Sprintf(`#!/bin/sh
count=0
if [ -f %[1]q ]; then
  count=$(cat %[1]q)
fi
count=$((count + 1))
printf '%%s' "$count" > %[1]q
printf '403 Forbidden' >&2
exit 1
`, counterFile)
	withFakeGitHubScript(t, script)

	result := runGitHubAPIWithMode("repos/acme/api/dependabot/alerts?state=open", true)
	err, _ := coreResultError(result).(error)
	if result.OK || !core.Is(err, errGitHubAPIAccessDenied) {
		t.Fatalf("runGitHubAPIWithMode() = %v, expected access denied error", err)
	}

	attempts := core.ReadFile(counterFile)
	if !attempts.OK {
		t.Fatalf("read attempts: %v", attempts.Error())
	}
	if core.Trim(string(attempts.Value.([]byte))) != "1" {
		t.Fatalf("runGitHubAPIWithMode retried access denied error: %s", attempts.Value.([]byte))
	}
}

func TestCmdSecurity_checkGitHubCLI_Good_Found(t *testing.T) {
	withFakeGitHubCLI(t)

	if r := checkGitHubCLI(); !r.OK {
		t.Fatalf("checkGitHubCLI() = %s, want nil", r.Error())
	}
}

func TestCmdSecurity_checkGitHubCLI_Bad_MissingBinary(t *testing.T) {
	t.Setenv("PATH", "")

	if r := checkGitHubCLI(); r.OK {
		t.Fatal("checkGitHubCLI should fail when gh is unavailable")
	}
}

func TestCmdSecurity_loadRegistry_Bad_ExplicitPathReturnsError(t *testing.T) {
	if result := loadRegistry(core.PathJoin(t.TempDir(), "missing-registry.yaml")); result.OK {
		t.Fatal("expected loadRegistry to fail for a missing explicit path")
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Good(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")

	result := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open")
	if !result.OK {
		t.Fatalf("runGitHubAPIRequest: %s", result.Error())
	}
	got := result.Value.([]byte)
	if string(got) != `{"ok":true}` {
		t.Fatalf("runGitHubAPIRequest() = %s, want payload", string(got))
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Bad_Maps404(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '404 Not Found' >&2\nexit 1\n")

	result := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open")
	err, _ := coreResultError(result).(error)
	if result.OK || !core.Is(err, errGitHubAPIEndpointNotFound) {
		t.Fatalf("runGitHubAPIRequest() = %v, expected errGitHubAPIEndpointNotFound", err)
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Bad_Maps403(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '403 Forbidden' >&2\nexit 1\n")

	result := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open")
	err, _ := coreResultError(result).(error)
	if result.OK || !core.Is(err, errGitHubAPIAccessDenied) {
		t.Fatalf("runGitHubAPIRequest() = %v, expected errGitHubAPIAccessDenied", err)
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Ugly_PreservesUnknownExitError(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf 'auth failed' >&2\nexit 2\n")

	if result := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open"); result.OK {
		t.Fatal("expected generic command error to be propagated")
	}
}

func TestCmdSecurity_isRetryableGitHubAPIError_Good(t *testing.T) {
	if !isRetryableGitHubAPIError(assertiveError("temporary")) {
		t.Fatal("expected unknown errors to be retryable")
	}
}

func TestCmdSecurity_isRetryableGitHubAPIError_Bad_NonRetryableErrors(t *testing.T) {
	if isRetryableGitHubAPIError(errGitHubAPIEndpointNotFound) {
		t.Fatal("expected endpoint-not-found errors to be non-retryable")
	}
	if isRetryableGitHubAPIError(errGitHubAPIAccessDenied) {
		t.Fatal("expected access-denied errors to be non-retryable")
	}
}

func TestCmdSecurity_isRetryableGitHubAPIError_Ugly_NilError(t *testing.T) {
	if !isRetryableGitHubAPIError(nil) {
		t.Fatal("expected nil error to be retryable by default")
	}
}

func TestCmdSecurity_combineSecurityCollectorErrors_Good_Empty(t *testing.T) {
	if r := combineSecurityCollectorErrors("acme/api", map[string]error{}); !r.OK {
		t.Fatalf("combineSecurityCollectorErrors empty map = %s", r.Error())
	}
}

func TestCmdSecurity_combineSecurityCollectorErrors_Bad_ReportsFailures(t *testing.T) {
	r := combineSecurityCollectorErrors("acme/api", map[string]error{
		"dependabot": core.NewError("dependabot failed"),
	})
	if r.OK {
		t.Fatal("expected error for failed collector")
	}
	if !core.Contains(r.Error(), "dependabot") || !core.Contains(r.Error(), "acme/api") {
		t.Fatalf("unexpected combined error: %s", r.Error())
	}
}

func TestCmdSecurity_combineSecurityCollectorErrors_Ugly_SortsCollectorsAlphabetically(t *testing.T) {
	r := combineSecurityCollectorErrors("acme/api", map[string]error{
		"code-scanning": core.NewError("code failed"),
		"dependabot":    core.NewError("dep failed"),
	})
	if r.OK {
		t.Fatal("expected collector combination error")
	}

	got := r.Error()
	dependabotPos := securityIndex(got, "dependabot")
	codeScanningPos := securityIndex(got, "code-scanning")
	if dependabotPos == -1 || codeScanningPos == -1 {
		t.Fatalf("combined error missing expected collector names: %v", got)
	}
	if codeScanningPos > dependabotPos {
		t.Fatalf("combined collector names are not sorted: %v", got)
	}
}

func TestCmdSecurity_combineSecurityTargetErrors_Good_Empty(t *testing.T) {
	if r := combineSecurityTargetErrors("security scan", map[string]error{}); !r.OK {
		t.Fatalf("combineSecurityTargetErrors empty map = %s", r.Error())
	}
}

func TestCmdSecurity_combineSecurityTargetErrors_Bad_ReportsTargetList(t *testing.T) {
	r := combineSecurityTargetErrors("security scan", map[string]error{
		"acme/api": assertiveError("api failed"),
		"acme/web": assertiveError("web failed"),
	})
	if r.OK {
		t.Fatal("expected target errors to be reported")
	}
	if !core.Contains(r.Error(), "security scan") ||
		!core.Contains(r.Error(), "acme/api") ||
		!core.Contains(r.Error(), "acme/web") {
		t.Fatalf("unexpected combined target error: %s", r.Error())
	}
}

func TestCmdSecurity_combineSecurityTargetErrors_Ugly_SortsTargetsAlphabetically(t *testing.T) {
	r := combineSecurityTargetErrors("security scan", map[string]error{
		"acme/web":  assertiveError("web failed"),
		"acme/api":  assertiveError("api failed"),
		"acme/docs": assertiveError("docs failed"),
	})
	if r.OK {
		t.Fatal("expected target errors")
	}

	got := r.Error()
	if securityIndex(got, "acme/api") > securityIndex(got, "acme/docs") ||
		securityIndex(got, "acme/docs") > securityIndex(got, "acme/web") {
		t.Fatalf("combined target errors are not sorted: %v", got)
	}
}

// --- AX-7 canonical triplets ---

func TestCmdSecurity_AlertSummary_Add_Good(t *core.T) {
	summary := &AlertSummary{}
	summary.Add("critical")
	summary.Add("high")

	core.AssertEqual(t, 2, summary.Total)
	core.AssertEqual(t, 1, summary.Critical)
	core.AssertEqual(t, 1, summary.High)
}

func TestCmdSecurity_AlertSummary_Add_Bad(t *core.T) {
	summary := &AlertSummary{}
	summary.Add("unknown-severity")
	got := summary.Unknown

	core.AssertEqual(t, 1, got)
	core.AssertEqual(t, 1, summary.Total)
}

func TestCmdSecurity_AlertSummary_Add_Ugly(t *core.T) {
	summary := &AlertSummary{}
	summary.Add("HIGH")
	summary.Add("")

	core.AssertEqual(t, 1, summary.High)
	core.AssertEqual(t, 1, summary.Unknown)
}

func TestCmdSecurity_AlertSummary_String_Good(t *core.T) {
	summary := &AlertSummary{}
	summary.Add("critical")
	got := summary.String()

	core.AssertContains(t, got, "critical")
	core.AssertContains(t, got, "1")
}

func TestCmdSecurity_AlertSummary_String_Bad(t *core.T) {
	summary := &AlertSummary{}
	got := summary.String()
	want := "No alerts"

	core.AssertContains(t, got, want)
}

func TestCmdSecurity_AlertSummary_String_Ugly(t *core.T) {
	summary := &AlertSummary{Low: 2, Unknown: 1, Total: 3}
	got := summary.String()
	plain := summary.PlainString()

	core.AssertContains(t, got, "low")
	core.AssertEqual(t, "2 low | 1 unknown", plain)
}

func TestCmdSecurity_AlertSummary_PlainString_Good(t *core.T) {
	summary := &AlertSummary{Critical: 1, High: 2, Total: 3}
	got := summary.PlainString()
	want := "1 critical | 2 high"

	core.AssertEqual(t, want, got)
}

func TestCmdSecurity_AlertSummary_PlainString_Bad(t *core.T) {
	summary := &AlertSummary{}
	got := summary.PlainString()
	want := "No alerts"

	core.AssertEqual(t, want, got)
}

func TestCmdSecurity_AlertSummary_PlainString_Ugly(t *core.T) {
	summary := &AlertSummary{Medium: 1, Low: 1, Unknown: 1, Total: 3}
	got := summary.PlainString()
	want := "1 medium | 1 low | 1 unknown"

	core.AssertEqual(t, want, got)
}

func TestCmdSecurity_AddSecurityCommands_Good(t *core.T) {
	root := &cli.Command{Use: "core"}
	AddSecurityCommands(root)
	cmd, _, err := root.Find([]string{"security"})

	core.AssertNoError(t, err)
	core.AssertEqual(t, "security", cmd.Name())
}

func TestCmdSecurity_AddSecurityCommands_Bad(t *core.T) {
	root := &cli.Command{Use: "core"}
	AddSecurityCommands(root)
	AddSecurityCommands(root)

	core.AssertLen(t, root.Commands(), 1)
	core.AssertEqual(t, "security", root.Commands()[0].Name())
}

func TestCmdSecurity_AddSecurityCommands_Ugly(t *core.T) {
	root := &cli.Command{Use: "core"}
	root.AddCommand(&cli.Command{Use: "security"})
	AddSecurityCommands(root)

	core.AssertLen(t, root.Commands(), 1)
	core.AssertEqual(t, "security", root.Commands()[0].Name())
}
