package security

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
			got, err := decodeGitHubArrayItems(tc.input)
			if err != nil {
				t.Fatalf("decodeGitHubArrayItems: %v", err)
			}
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
		if _, err := decodeGitHubArrayItems(input); err == nil {
			t.Fatalf("expected error for %s", input)
		}
	}
}

func TestCmdSecurity_decodeDependabotAlerts_Good(t *testing.T) {
	alerts, err := decodeDependabotAlerts([]byte(`[
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
	if err != nil {
		t.Fatalf("decodeDependabotAlerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].Advisory.CVEID != "CVE-2026-0001" || alerts[0].SecurityVulnerability.FirstPatchedVersion.Identifier != "1.0.2" {
		t.Fatalf("unexpected dependabot alert: %+v", alerts)
	}
}

func TestCmdSecurity_decodeCodeScanningAlerts_Good(t *testing.T) {
	alerts, err := decodeCodeScanningAlerts([]byte(`[
		[{
			"number": 4,
			"state": "open",
			"rule": {"id": "gosec/G401", "severity": "medium", "description": "Weak crypto", "tags": ["security"]},
			"tool": {"name": "CodeQL", "version": "2.20.0"},
			"most_recent_instance": {
				"location": {"path": "main.go", "start_line": 14, "end_line": 14},
				"message": {"text": "Potential weak crypto"}
			}
		}]
	]`))
	if err != nil {
		t.Fatalf("decodeCodeScanningAlerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].Rule.ID != "gosec/G401" || alerts[0].MostRecentInstance.Location.Path != "main.go" {
		t.Fatalf("unexpected code scanning alert: %+v", alerts)
	}
}

func TestCmdSecurity_decodeSecretScanningAlerts_Good(t *testing.T) {
	alerts, err := decodeSecretScanningAlerts([]byte(`[
		[{
			"number": 9,
			"state": "open",
			"secret_type": "aws_access_key",
			"secret": "AKIA...",
			"push_protection_bypassed": true,
			"resolution": "revoked"
		}]
	]`))
	if err != nil {
		t.Fatalf("decodeSecretScanningAlerts: %v", err)
	}
	if len(alerts) != 1 || !alerts[0].PushProtection || alerts[0].Resolution != "revoked" {
		t.Fatalf("unexpected secret scanning alert: %+v", alerts)
	}
}

func TestCmdSecurity_decodeGitHubRepositoryNames_Good(t *testing.T) {
	names, err := decodeGitHubRepositoryNames([]byte(`[
		[{"full_name":"acme/z"},{"full_name":"acme/a"},{"full_name":"acme/a"},{"full_name":""}]
	]`))
	if err != nil {
		t.Fatalf("decodeGitHubRepositoryNames: %v", err)
	}
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
	if got := summary.String(); !strings.Contains(got, "No alerts") {
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
	if got := normalizeWhitespace(summary.String()); !strings.Contains(got, "critical") || !strings.Contains(got, "unknown") {
		t.Fatalf("styled summary missing expected severities: %q", got)
	}
}

func TestCmdSecurity_recordSecurityMetricsEvent_Ugly_DoesNotPanic(t *testing.T) {
	withSecurityTempHome(t)

	// recordSecurityMetricsEvent intentionally ignores write errors, so this test
	// only verifies that the wrapper stays no-op from the caller's perspective.
	recordSecurityMetricsEvent(ai.Event{Type: "security.alerts"})
	events, err := ai.ReadEvents(time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("ReadEvents after recordSecurityMetricsEvent: %v", err)
	}
	if len(events) != 1 || events[0].Type != "security.alerts" {
		t.Fatalf("unexpected metrics events: %+v", events)
	}
}

func TestCmdSecurity_runGitHubAPI_Good_ReturnsStdout(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '[{\"full_name\":\"acme/api\"}]'\n")

	got, err := runGitHubAPI("repos/acme/api/dependabot/alerts?state=open")
	if err != nil {
		t.Fatalf("runGitHubAPI: %v", err)
	}
	if !bytes.Equal(got, []byte(`[{"full_name":"acme/api"}]`)) {
		t.Fatalf("runGitHubAPI = %s, want JSON output", string(got))
	}
}

func TestCmdSecurity_runGitHubAPI_Bad_404ReturnsEmptyArray(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '404 Not Found' >&2\nexit 1\n")

	got, err := runGitHubAPI("repos/acme/api/dependabot/alerts?state=open")
	if err != nil {
		t.Fatalf("runGitHubAPI 404 should not fail: %v", err)
	}
	if !bytes.Equal(got, []byte("[]")) {
		t.Fatalf("runGitHubAPI 404 = %s, want []", string(got))
	}
}

func TestCmdSecurity_runGitHubAPIStrict_Bad_DoesNotFallbackOnMissingEndpoint(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '404 Not Found' >&2\nexit 1\n")

	if got, err := runGitHubAPIStrict("repos/acme/api/dependabot/alerts?state=open"); err == nil {
		t.Fatalf("runGitHubAPIStrict returned %q, want error", string(got))
	}
}

func TestCmdSecurity_runGitHubAPIWithMode_Good_RetriesTransientFailures(t *testing.T) {
	counterFile := filepath.Join(t.TempDir(), "attempts")
	script := fmt.Sprintf(`#!/bin/sh
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

	got, err := runGitHubAPIWithMode("repos/acme/api/dependabot/alerts?state=open", true)
	if err != nil {
		t.Fatalf("runGitHubAPIWithMode retry path: %v", err)
	}
	if string(got) != "[]" {
		t.Fatalf("runGitHubAPIWithMode retry path = %q, want []", string(got))
	}
}

func TestCmdSecurity_runGitHubAPIWithMode_Bad_DoesNotRetryAccessDenied(t *testing.T) {
	counterFile := filepath.Join(t.TempDir(), "attempts")
	script := fmt.Sprintf(`#!/bin/sh
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

	_, err := runGitHubAPIWithMode("repos/acme/api/dependabot/alerts?state=open", true)
	if !errors.Is(err, errGitHubAPIAccessDenied) {
		t.Fatalf("runGitHubAPIWithMode() = %v, expected access denied error", err)
	}

	attempts, readErr := os.ReadFile(counterFile)
	if readErr != nil {
		t.Fatalf("read attempts: %v", readErr)
	}
	if strings.TrimSpace(string(attempts)) != "1" {
		t.Fatalf("runGitHubAPIWithMode retried access denied error: %s", attempts)
	}
}

func TestCmdSecurity_checkGitHubCLI_Good_Found(t *testing.T) {
	withFakeGitHubCLI(t)

	if err := checkGitHubCLI(); err != nil {
		t.Fatalf("checkGitHubCLI() = %v, want nil", err)
	}
}

func TestCmdSecurity_checkGitHubCLI_Bad_MissingBinary(t *testing.T) {
	t.Setenv("PATH", "")

	if err := checkGitHubCLI(); err == nil {
		t.Fatal("checkGitHubCLI should fail when gh is unavailable")
	}
}

func TestCmdSecurity_loadRegistry_Bad_ExplicitPathReturnsError(t *testing.T) {
	if _, err := loadRegistry(filepath.Join(t.TempDir(), "missing-registry.yaml")); err == nil {
		t.Fatal("expected loadRegistry to fail for a missing explicit path")
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Good(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")

	got, err := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open")
	if err != nil {
		t.Fatalf("runGitHubAPIRequest: %v", err)
	}
	if !bytes.Equal(got, []byte(`{"ok":true}`)) {
		t.Fatalf("runGitHubAPIRequest() = %s, want payload", string(got))
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Bad_Maps404(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '404 Not Found' >&2\nexit 1\n")

	_, err := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open")
	if !errors.Is(err, errGitHubAPIEndpointNotFound) {
		t.Fatalf("runGitHubAPIRequest() = %v, expected errGitHubAPIEndpointNotFound", err)
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Bad_Maps403(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf '403 Forbidden' >&2\nexit 1\n")

	_, err := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open")
	if !errors.Is(err, errGitHubAPIAccessDenied) {
		t.Fatalf("runGitHubAPIRequest() = %v, expected errGitHubAPIAccessDenied", err)
	}
}

func TestCmdSecurity_runGitHubAPIRequest_Ugly_PreservesUnknownExitError(t *testing.T) {
	withFakeGitHubScript(t, "#!/bin/sh\nprintf 'auth failed' >&2\nexit 2\n")

	if _, err := runGitHubAPIRequest("repos/acme/api/dependabot/alerts?state=open"); err == nil {
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
	if err := combineSecurityCollectorErrors("acme/api", map[string]error{}); err != nil {
		t.Fatalf("combineSecurityCollectorErrors empty map = %v", err)
	}
}

func TestCmdSecurity_combineSecurityCollectorErrors_Bad_ReportsFailures(t *testing.T) {
	err := combineSecurityCollectorErrors("acme/api", map[string]error{
		"dependabot": errors.New("dependabot failed"),
	})
	if err == nil {
		t.Fatal("expected error for failed collector")
	}
	if !strings.Contains(err.Error(), "dependabot") || !strings.Contains(err.Error(), "acme/api") {
		t.Fatalf("unexpected combined error: %v", err)
	}
}

func TestCmdSecurity_combineSecurityCollectorErrors_Ugly_SortsCollectorsAlphabetically(t *testing.T) {
	err := combineSecurityCollectorErrors("acme/api", map[string]error{
		"code-scanning": errors.New("code failed"),
		"dependabot":    errors.New("dep failed"),
	})
	if err == nil {
		t.Fatal("expected collector combination error")
	}

	got := err.Error()
	dependabotPos := strings.Index(got, "dependabot")
	codeScanningPos := strings.Index(got, "code-scanning")
	if dependabotPos == -1 || codeScanningPos == -1 {
		t.Fatalf("combined error missing expected collector names: %v", got)
	}
	if codeScanningPos > dependabotPos {
		t.Fatalf("combined collector names are not sorted: %v", got)
	}
}

func TestCmdSecurity_combineSecurityTargetErrors_Good_Empty(t *testing.T) {
	if err := combineSecurityTargetErrors("security scan", map[string]error{}); err != nil {
		t.Fatalf("combineSecurityTargetErrors empty map = %v", err)
	}
}

func TestCmdSecurity_combineSecurityTargetErrors_Bad_ReportsTargetList(t *testing.T) {
	err := combineSecurityTargetErrors("security scan", map[string]error{
		"acme/api": assertiveError("api failed"),
		"acme/web": assertiveError("web failed"),
	})
	if err == nil {
		t.Fatal("expected target errors to be reported")
	}
	if !strings.Contains(err.Error(), "security scan") ||
		!strings.Contains(err.Error(), "acme/api") ||
		!strings.Contains(err.Error(), "acme/web") {
		t.Fatalf("unexpected combined target error: %v", err)
	}
}

func TestCmdSecurity_combineSecurityTargetErrors_Ugly_SortsTargetsAlphabetically(t *testing.T) {
	err := combineSecurityTargetErrors("security scan", map[string]error{
		"acme/web":  assertiveError("web failed"),
		"acme/api":  assertiveError("api failed"),
		"acme/docs": assertiveError("docs failed"),
	})
	if err == nil {
		t.Fatal("expected target errors")
	}

	got := err.Error()
	if strings.Index(got, "acme/api") > strings.Index(got, "acme/docs") ||
		strings.Index(got, "acme/docs") > strings.Index(got, "acme/web") {
		t.Fatalf("combined target errors are not sorted: %v", got)
	}
}
