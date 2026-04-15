package security

import (
	"encoding/json"
	"strings"
	"testing"

	"forge.lthn.ai/core/cli/pkg/cli"
)

func TestCmdDeps_collectDepAlerts_Good(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "repos/acme/api/dependabot/alerts?state=open" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(`[
			{
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
			},
			{
				"number": 8,
				"state": "closed",
				"security_advisory": {"severity": "critical", "cve_id": "CVE-2026-0002", "summary": "Closed", "description": "Closed"},
				"dependency": {"package": {"name": "pkg", "ecosystem": "npm"}, "manifest_path": "package.json"},
				"security_vulnerability": {"package": {"name": "pkg", "ecosystem": "npm"}, "vulnerable_version_range": "< 2.0.0"}
			}
		]`), nil
	})

	alerts, err := collectDepAlerts(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, "high")
	if err != nil {
		t.Fatalf("collectDepAlerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].CVE != "CVE-2026-0001" || alerts[0].PatchedVersion != "1.0.2" {
		t.Fatalf("unexpected dep alerts: %+v", alerts)
	}
}

func TestCmdDeps_runDeps_Good_JSONOutput(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "repos/acme/api/dependabot/alerts?state=open" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(`[{
			"number": 7,
			"state": "open",
			"security_advisory": {"severity": "critical", "cve_id": "CVE-2026-0001", "summary": "Upgrade OpenSSL", "description": "OpenSSL needs updating"},
			"dependency": {"package": {"name": "openssl", "ecosystem": "npm"}, "manifest_path": "package.json"},
			"security_vulnerability": {"package": {"name": "openssl", "ecosystem": "npm"}, "first_patched_version": {"identifier": "1.0.2"}, "vulnerable_version_range": "< 1.0.2"}
		}]`), nil
	})

	output := captureStdout(t, func() {
		if err := runDeps(SecuritySelectionOptions{ExternalTarget: "acme/api", JSONOutput: true}); err != nil {
			t.Fatalf("runDeps: %v", err)
		}
	})

	var rows []DepAlert
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &rows); err != nil {
		t.Fatalf("runDeps JSON output: %v\noutput: %s", err, output)
	}
	if len(rows) != 1 || rows[0].CVE != "CVE-2026-0001" || rows[0].Repo != "api" {
		t.Fatalf("unexpected JSON rows: %+v", rows)
	}
}

func TestCmdDeps_addDepsCommand_Good_BindsFlagsPerCommandInstance(t *testing.T) {
	firstRoot := &cli.Command{Use: "core"}
	secondRoot := &cli.Command{Use: "core"}

	addDepsCommand(firstRoot)
	addDepsCommand(secondRoot)

	firstCommand, _, err := firstRoot.Find([]string{"deps"})
	if err != nil {
		t.Fatalf("find first deps command: %v", err)
	}
	secondCommand, _, err := secondRoot.Find([]string{"deps"})
	if err != nil {
		t.Fatalf("find second deps command: %v", err)
	}

	if err := firstCommand.Flags().Set("severity", "high"); err != nil {
		t.Fatalf("set first deps severity: %v", err)
	}

	firstSeverity, err := firstCommand.Flags().GetString("severity")
	if err != nil {
		t.Fatalf("get first deps severity: %v", err)
	}
	secondSeverity, err := secondCommand.Flags().GetString("severity")
	if err != nil {
		t.Fatalf("get second deps severity: %v", err)
	}

	if firstSeverity != "high" {
		t.Fatalf("first deps severity = %q, want high", firstSeverity)
	}
	if secondSeverity != "" {
		t.Fatalf("second deps severity leaked shared state: got %q", secondSeverity)
	}
}
