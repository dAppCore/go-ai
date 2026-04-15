package security

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"dappco.re/go/ai/ai"
	"forge.lthn.ai/core/cli/pkg/cli"
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
