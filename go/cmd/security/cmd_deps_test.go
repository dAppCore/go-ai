package security

import (
	"testing"

	core "dappco.re/go"
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

	alertsResult := collectDepAlerts(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, "high")
	if !alertsResult.OK {
		t.Fatalf("collectDepAlerts: %s", alertsResult.Error())
	}
	alerts := alertsResult.Value.([]DepAlert)
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
		if r := runDeps(SecuritySelectionOptions{ExternalTarget: "acme/api", JSONOutput: true}); !r.OK {
			t.Fatalf("runDeps: %s", r.Error())
		}
	})

	var rows []DepAlert
	if r := core.JSONUnmarshal([]byte(core.Trim(output)), &rows); !r.OK {
		t.Fatalf("runDeps JSON output: %v\noutput: %s", r.Error(), output)
	}
	if len(rows) != 1 || rows[0].CVE != "CVE-2026-0001" || rows[0].Repo != "api" {
		t.Fatalf("unexpected JSON rows: %+v", rows)
	}
}

func TestCmdDeps_runDeps_Bad_MultiTargetPartialFailureFailsClosed(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	registryPath := writeSecurityRegistry(t, "acme", "api", "web")

	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/dependabot/alerts?state=open":
			return []byte(`[]`), nil
		case "repos/acme/web/dependabot/alerts?state=open":
			return nil, assertiveError("dependabot unavailable")
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	r := runDeps(SecuritySelectionOptions{RegistryPath: registryPath})
	if r.OK {
		t.Fatal("expected multi-target partial failure to fail closed")
	}
	if !core.Contains(r.Error(), "security deps failed") || !core.Contains(r.Error(), "acme/web") {
		t.Fatalf("unexpected error: %s", r.Error())
	}
}

func TestCmdDeps_addDepsCommand_Good_BindsFlagsPerCommandInstance(t *testing.T) {
	firstRoot := core.New()
	secondRoot := core.New()

	if r := addDepsCommand(firstRoot, "deps"); !r.OK {
		t.Fatalf("register first deps command: %s", r.Error())
	}
	if r := addDepsCommand(secondRoot, "deps"); !r.OK {
		t.Fatalf("register second deps command: %s", r.Error())
	}

	firstResult := firstRoot.Command("deps")
	if !firstResult.OK {
		t.Fatalf("find first deps command: %s", firstResult.Error())
	}
	secondResult := secondRoot.Command("deps")
	if !secondResult.OK {
		t.Fatalf("find second deps command: %s", secondResult.Error())
	}
	firstCommand := firstResult.Value.(*core.Command)
	secondCommand := secondResult.Value.(*core.Command)

	firstCommand.Flags.Set("severity", "high")
	firstSeverity := firstCommand.Flags.String("severity")
	secondSeverity := secondCommand.Flags.String("severity")

	if firstSeverity != "high" {
		t.Fatalf("first deps severity = %q, want high", firstSeverity)
	}
	if secondSeverity != "" {
		t.Fatalf("second deps severity leaked shared state: got %q", secondSeverity)
	}
}
