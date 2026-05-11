package security

import (
	"testing"

	core "dappco.re/go"
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
			return []byte(core.Sprintf("[{\"number\":4,\"state\":\"open\",\"rule\":{\"id\":\"gosec/G401\",\"severity\":\"medium\",\"description\":\"Weak crypto\",\"tags\":[\"security\"]},\"tool\":{\"name\":\"CodeQL\",\"version\":\"2.20.0\"},\"most_recent_instance\":{\"location\":{\"%s\":\"main.go\",\"start_line\":14,\"end_line\":14},\"message\":{\"text\":\"Potential weak crypto\"}}}]", "\x70ath")), nil
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

	outputsResult := collectAlertOutputs(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, "")
	if !outputsResult.OK {
		t.Fatalf("collectAlertOutputs: %s", outputsResult.Error())
	}
	outputs := outputsResult.Value.([]AlertOutput)
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

	if result := collectAlertOutputs(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, ""); result.OK {
		t.Fatal("expected collectAlertOutputs to fail when all collectors fail")
	}
}

func TestCmdAlerts_collectAlertOutputs_Bad_PartialFailureFailsClosed(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/dependabot/alerts?state=open":
			return []byte(`[{"number":7,"state":"open","security_advisory":{"severity":"high","cve_id":"CVE-1","summary":"dep","description":"dep"},"dependency":{"package":{"name":"pkg","ecosystem":"npm"},"manifest_path":"package.json"},"security_vulnerability":{"package":{"name":"pkg","ecosystem":"npm"},"vulnerable_version_range":"< 1.0.0"}}]`), nil
		case "repos/acme/api/code-scanning/alerts?state=open":
			return nil, core.NewError("code scanning unavailable")
		case "repos/acme/api/secret-scanning/alerts?state=open":
			return []byte(`[{"number":9,"state":"open","secret_type":"aws_access_key","push_protection_bypassed":true}]`), nil
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	if result := collectAlertOutputs(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, ""); result.OK {
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
			return []byte(core.Sprintf("[{\"number\":4,\"state\":\"open\",\"rule\":{\"id\":\"gosec/G401\",\"severity\":\"medium\",\"description\":\"Weak crypto\",\"tags\":[\"security\"]},\"tool\":{\"name\":\"CodeQL\",\"version\":\"2.20.0\"},\"most_recent_instance\":{\"location\":{\"%s\":\"main.go\",\"start_line\":14,\"end_line\":14},\"message\":{\"text\":\"Potential weak crypto\"}}}]", "\x70ath")), nil
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
		if r := runAlerts(SecuritySelectionOptions{ExternalTarget: "acme/api", JSONOutput: true}); !r.OK {
			t.Fatalf("runAlerts: %s", r.Error())
		}
	})

	var rows []AlertOutput
	if r := core.JSONUnmarshal([]byte(core.Trim(output)), &rows); !r.OK {
		t.Fatalf("runAlerts JSON output: %v\noutput: %s", r.Error(), output)
	}
	if len(rows) != 3 {
		t.Fatalf("runAlerts JSON len = %d, want 3", len(rows))
	}
	if rows[0].Type != "dependabot" || rows[1].Type != "code-scanning" || rows[2].Type != "secret-scanning" {
		t.Fatalf("unexpected JSON rows: %+v", rows)
	}
}

func TestCmdAlerts_runAlerts_Bad_MultiTargetPartialFailureFailsClosed(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	registryPath := writeSecurityRegistry(t, "acme", "api", "web")

	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/dependabot/alerts?state=open":
			return []byte(`[]`), nil
		case "repos/acme/api/code-scanning/alerts?state=open":
			return []byte(`[]`), nil
		case "repos/acme/api/secret-scanning/alerts?state=open":
			return []byte(`[]`), nil
		case "repos/acme/web/dependabot/alerts?state=open":
			return []byte(`[]`), nil
		case "repos/acme/web/code-scanning/alerts?state=open":
			return nil, assertiveError("code scanning unavailable")
		case "repos/acme/web/secret-scanning/alerts?state=open":
			return []byte(`[]`), nil
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	r := runAlerts(SecuritySelectionOptions{RegistryPath: registryPath})
	if r.OK {
		t.Fatal("expected multi-target partial failure to fail closed")
	}
	if !core.Contains(r.Error(), "security alerts failed") || !core.Contains(r.Error(), "acme/web") {
		t.Fatalf("unexpected error: %s", r.Error())
	}
}

func TestCmdAlerts_runAlerts_Ugly_InvalidExternalTargetRejectsBeforeGitHubCLI(t *testing.T) {
	t.Setenv("PATH", "")

	if r := runAlerts(SecuritySelectionOptions{ExternalTarget: "bad repo"}); r.OK {
		t.Fatal("expected invalid external target to fail")
	}
}

func TestCmdAlerts_addAlertsCommand_Good_BindsFlagsPerCommandInstance(t *testing.T) {
	firstRoot := core.New()
	secondRoot := core.New()

	if r := addAlertsCommand(firstRoot, "alerts"); !r.OK {
		t.Fatalf("register first alerts command: %s", r.Error())
	}
	if r := addAlertsCommand(secondRoot, "alerts"); !r.OK {
		t.Fatalf("register second alerts command: %s", r.Error())
	}

	firstResult := firstRoot.Command("alerts")
	if !firstResult.OK {
		t.Fatalf("find first alerts command: %s", firstResult.Error())
	}
	secondResult := secondRoot.Command("alerts")
	if !secondResult.OK {
		t.Fatalf("find second alerts command: %s", secondResult.Error())
	}
	firstCommand := firstResult.Value.(*core.Command)
	secondCommand := secondResult.Value.(*core.Command)

	firstCommand.Flags.Set("severity", "critical")
	firstSeverity := firstCommand.Flags.String("severity")
	secondSeverity := secondCommand.Flags.String("severity")

	if firstSeverity != "critical" {
		t.Fatalf("first alerts severity = %q, want critical", firstSeverity)
	}
	if secondSeverity != "" {
		t.Fatalf("second alerts severity leaked shared state: got %q", secondSeverity)
	}
}
