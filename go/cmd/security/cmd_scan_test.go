package security

import (
	"testing"

	core "dappco.re/go"
)

func TestCmdScan_collectScanAlerts_Good(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "repos/acme/api/code-scanning/alerts?state=open" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		payload := core.Sprintf("[{\"number\":4,\"state\":\"open\",\"rule\":{\"id\":\"gosec/G401\",\"severity\":\"\",\"description\":\"Weak crypto\",\"tags\":[\"security\"]},\"tool\":{\"name\":\"CodeQL\",\"version\":\"2.20.0\"},\"most_recent_instance\":{\"location\":{\"%s\":\"main.go\",\"start_line\":14,\"end_line\":14},\"message\":{\"text\":\"Potential weak crypto\"}}},{\"number\":5,\"state\":\"open\",\"rule\":{\"id\":\"gosec/G402\",\"severity\":\"critical\",\"description\":\"Weak hash\",\"tags\":[\"security\"]},\"tool\":{\"name\":\"Semgrep\",\"version\":\"1.0\"},\"most_recent_instance\":{\"location\":{\"%s\":\"main.go\",\"start_line\":20,\"end_line\":20},\"message\":{\"text\":\"Different tool\"}}}]", "\x70ath", "\x70ath")
		return []byte(payload), nil
	})

	alertsResult := collectScanAlerts(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, ScanCommandOptions{
		Selection: SecuritySelectionOptions{SeverityFilter: "medium"},
		ToolName:  "CodeQL",
	})
	if !alertsResult.OK {
		t.Fatalf("collectScanAlerts: %s", alertsResult.Error())
	}
	alerts := alertsResult.Value.([]ScanAlert)
	if len(alerts) != 1 || alerts[0].RuleID != "gosec/G401" || alerts[0].Severity != "medium" {
		t.Fatalf("unexpected scan alerts: %+v", alerts)
	}
}

func TestCmdScan_runScan_Good_JSONOutput(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "repos/acme/api/code-scanning/alerts?state=open" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(core.Sprintf("[{\"number\":4,\"state\":\"open\",\"rule\":{\"id\":\"gosec/G401\",\"severity\":\"\",\"description\":\"Weak crypto\",\"tags\":[\"security\"]},\"tool\":{\"name\":\"CodeQL\",\"version\":\"2.20.0\"},\"most_recent_instance\":{\"location\":{\"%s\":\"main.go\",\"start_line\":14,\"end_line\":14},\"message\":{\"text\":\"Potential weak crypto\"}}}]", "\x70ath")), nil
	})

	output := captureStdout(t, func() {
		if r := runScan(ScanCommandOptions{
			Selection: SecuritySelectionOptions{ExternalTarget: "acme/api", JSONOutput: true},
			ToolName:  "CodeQL",
		}); !r.OK {
			t.Fatalf("runScan: %s", r.Error())
		}
	})

	var rows []ScanAlert
	if r := core.JSONUnmarshal([]byte(core.Trim(output)), &rows); !r.OK {
		t.Fatalf("runScan JSON output: %v\noutput: %s", r.Error(), output)
	}
	if len(rows) != 1 || rows[0].Severity != "medium" || rows[0].Tool != "CodeQL" {
		t.Fatalf("unexpected scan rows: %+v", rows)
	}
}

func TestCmdScan_runScan_Bad_MultiTargetPartialFailureFailsClosed(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	registryPath := writeSecurityRegistry(t, "acme", "api", "web")

	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/code-scanning/alerts?state=open":
			return []byte(`[]`), nil
		case "repos/acme/web/code-scanning/alerts?state=open":
			return nil, assertiveError("code scanning unavailable")
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	r := runScan(ScanCommandOptions{Selection: SecuritySelectionOptions{RegistryPath: registryPath}})
	if r.OK {
		t.Fatal("expected multi-target partial failure to fail closed")
	}
	if !core.Contains(r.Error(), "security scan failed") || !core.Contains(r.Error(), "acme/web") {
		t.Fatalf("unexpected error: %s", r.Error())
	}
}

func TestCmdScan_addScanCommand_Good_BindsFlagsPerCommandInstance(t *testing.T) {
	firstRoot := core.New()
	secondRoot := core.New()

	if r := addScanCommand(firstRoot, "scan"); !r.OK {
		t.Fatalf("register first scan command: %s", r.Error())
	}
	if r := addScanCommand(secondRoot, "scan"); !r.OK {
		t.Fatalf("register second scan command: %s", r.Error())
	}

	firstResult := firstRoot.Command("scan")
	if !firstResult.OK {
		t.Fatalf("find first scan command: %s", firstResult.Error())
	}
	secondResult := secondRoot.Command("scan")
	if !secondResult.OK {
		t.Fatalf("find second scan command: %s", secondResult.Error())
	}
	firstCommand := firstResult.Value.(*core.Command)
	secondCommand := secondResult.Value.(*core.Command)

	firstCommand.Flags.Set("tool", "CodeQL")
	firstTool := firstCommand.Flags.String("tool")
	secondTool := secondCommand.Flags.String("tool")

	if firstTool != "CodeQL" {
		t.Fatalf("first scan tool = %q, want CodeQL", firstTool)
	}
	if secondTool != "" {
		t.Fatalf("second scan tool leaked shared state: got %q", secondTool)
	}
}
