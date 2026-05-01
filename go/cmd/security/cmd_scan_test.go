package security

import (
	"testing"

	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
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
	firstRoot := &cli.Command{Use: "core"}
	secondRoot := &cli.Command{Use: "core"}

	addScanCommand(firstRoot)
	addScanCommand(secondRoot)

	firstCommand, _, err := firstRoot.Find([]string{"scan"})
	if err != nil {
		t.Fatalf("find first scan command: %v", err)
	}
	secondCommand, _, err := secondRoot.Find([]string{"scan"})
	if err != nil {
		t.Fatalf("find second scan command: %v", err)
	}

	if err := firstCommand.Flags().Set("tool", "CodeQL"); err != nil {
		t.Fatalf("set first scan tool: %v", err)
	}

	firstTool, err := firstCommand.Flags().GetString("tool")
	if err != nil {
		t.Fatalf("get first scan tool: %v", err)
	}
	secondTool, err := secondCommand.Flags().GetString("tool")
	if err != nil {
		t.Fatalf("get second scan tool: %v", err)
	}

	if firstTool != "CodeQL" {
		t.Fatalf("first scan tool = %q, want CodeQL", firstTool)
	}
	if secondTool != "" {
		t.Fatalf("second scan tool leaked shared state: got %q", secondTool)
	}
}
