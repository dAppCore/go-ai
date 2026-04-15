package security

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"forge.lthn.ai/core/cli/pkg/cli"
)

func TestCmdAlerts_collectAlertOutputs_Good(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/dependabot/alerts?state=open":
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
		case "repos/acme/api/code-scanning/alerts?state=open":
			return []byte(`[
				{
					"number": 4,
					"state": "open",
					"rule": {"id": "gosec/G401", "severity": "medium", "description": "Weak crypto", "tags": ["security"]},
					"tool": {"name": "CodeQL", "version": "2.20.0"},
					"most_recent_instance": {
						"location": {"path": "main.go", "start_line": 14, "end_line": 14},
						"message": {"text": "Potential weak crypto"}
					}
				}
			]`), nil
		case "repos/acme/api/secret-scanning/alerts?state=open":
			return []byte(`[
				{
					"number": 9,
					"state": "open",
					"secret_type": "aws_access_key",
					"secret": "AKIA...",
					"push_protection_bypassed": true,
					"resolution": "revoked"
				}
			]`), nil
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	outputs, err := collectAlertOutputs(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, "")
	if err != nil {
		t.Fatalf("collectAlertOutputs: %v", err)
	}
	if len(outputs) != 3 {
		t.Fatalf("collectAlertOutputs len = %d, want 3", len(outputs))
	}
	if outputs[0].Type != "dependabot" || outputs[0].ID != "CVE-2026-0001" || outputs[0].Package != "openssl" {
		t.Fatalf("unexpected dependabot output: %+v", outputs[0])
	}
	if outputs[1].Type != "code-scanning" || outputs[1].Location != "main.go:14" {
		t.Fatalf("unexpected code scanning output: %+v", outputs[1])
	}
	if outputs[2].Type != "secret-scanning" || outputs[2].Severity != "high" {
		t.Fatalf("unexpected secret output: %+v", outputs[2])
	}
}

func TestCmdAlerts_collectAlertOutputs_Bad_AllCollectorsFail(t *testing.T) {
	stubGitHubAPI(t, func(string) ([]byte, error) {
		return nil, assertiveError("github unavailable")
	})

	if _, err := collectAlertOutputs(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, ""); err == nil {
		t.Fatal("expected collectAlertOutputs to fail when all collectors fail")
	}
}

func TestCmdAlerts_collectAlertOutputs_Bad_PartialFailureFailsClosed(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/dependabot/alerts?state=open":
			return []byte(`[{"number":7,"state":"open","security_advisory":{"severity":"high","cve_id":"CVE-1","summary":"dep","description":"dep"},"dependency":{"package":{"name":"pkg","ecosystem":"npm"},"manifest_path":"package.json"},"security_vulnerability":{"package":{"name":"pkg","ecosystem":"npm"},"vulnerable_version_range":"< 1.0.0"}}]`), nil
		case "repos/acme/api/code-scanning/alerts?state=open":
			return nil, errors.New("code scanning unavailable")
		case "repos/acme/api/secret-scanning/alerts?state=open":
			return []byte(`[{"number":9,"state":"open","secret_type":"aws_access_key","push_protection_bypassed":true}]`), nil
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	if _, err := collectAlertOutputs(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, ""); err == nil {
		t.Fatal("expected collectAlertOutputs to fail closed on partial collector failure")
	}
}

func TestCmdAlerts_runAlerts_Good_JSONOutput(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/dependabot/alerts?state=open":
			return []byte(`[{
				"number": 7,
				"state": "open",
				"security_advisory": {"severity": "critical", "cve_id": "CVE-2026-0001", "summary": "Upgrade OpenSSL", "description": "OpenSSL needs updating"},
				"dependency": {"package": {"name": "openssl", "ecosystem": "npm"}, "manifest_path": "package.json"},
				"security_vulnerability": {"package": {"name": "openssl", "ecosystem": "npm"}, "first_patched_version": {"identifier": "1.0.2"}, "vulnerable_version_range": "< 1.0.2"}
			}]`), nil
		case "repos/acme/api/code-scanning/alerts?state=open":
			return []byte(`[{
				"number": 4,
				"state": "open",
				"rule": {"id": "gosec/G401", "severity": "medium", "description": "Weak crypto", "tags": ["security"]},
				"tool": {"name": "CodeQL", "version": "2.20.0"},
				"most_recent_instance": {"location": {"path": "main.go", "start_line": 14, "end_line": 14}, "message": {"text": "Potential weak crypto"}}
			}]`), nil
		case "repos/acme/api/secret-scanning/alerts?state=open":
			return []byte(`[{
				"number": 9,
				"state": "open",
				"secret_type": "aws_access_key",
				"push_protection_bypassed": true,
				"resolution": "revoked"
			}]`), nil
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	output := captureStdout(t, func() {
		if err := runAlerts(SecuritySelectionOptions{ExternalTarget: "acme/api", JSONOutput: true}); err != nil {
			t.Fatalf("runAlerts: %v", err)
		}
	})

	var rows []AlertOutput
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &rows); err != nil {
		t.Fatalf("runAlerts JSON output: %v\noutput: %s", err, output)
	}
	if len(rows) != 3 {
		t.Fatalf("runAlerts JSON len = %d, want 3", len(rows))
	}
	if rows[0].Type != "dependabot" || rows[1].Type != "code-scanning" || rows[2].Type != "secret-scanning" {
		t.Fatalf("unexpected JSON rows: %+v", rows)
	}
}

func TestCmdAlerts_addAlertsCommand_Good_BindsFlagsPerCommandInstance(t *testing.T) {
	firstRoot := &cli.Command{Use: "core"}
	secondRoot := &cli.Command{Use: "core"}

	addAlertsCommand(firstRoot)
	addAlertsCommand(secondRoot)

	firstCommand, _, err := firstRoot.Find([]string{"alerts"})
	if err != nil {
		t.Fatalf("find first alerts command: %v", err)
	}
	secondCommand, _, err := secondRoot.Find([]string{"alerts"})
	if err != nil {
		t.Fatalf("find second alerts command: %v", err)
	}

	if err := firstCommand.Flags().Set("severity", "critical"); err != nil {
		t.Fatalf("set first alerts severity: %v", err)
	}

	firstSeverity, err := firstCommand.Flags().GetString("severity")
	if err != nil {
		t.Fatalf("get first alerts severity: %v", err)
	}
	secondSeverity, err := secondCommand.Flags().GetString("severity")
	if err != nil {
		t.Fatalf("get second alerts severity: %v", err)
	}

	if firstSeverity != "critical" {
		t.Fatalf("first alerts severity = %q, want critical", firstSeverity)
	}
	if secondSeverity != "" {
		t.Fatalf("second alerts severity leaked shared state: got %q", secondSeverity)
	}
}
