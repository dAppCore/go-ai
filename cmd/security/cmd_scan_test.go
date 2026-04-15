package security

import (
	"encoding/json"
	"strings"
	"testing"

	"forge.lthn.ai/core/cli/pkg/cli"
)

func TestCmdScan_collectScanAlerts_Good(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "repos/acme/api/code-scanning/alerts?state=open" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(`[
			{
				"number": 4,
				"state": "open",
				"rule": {"id": "gosec/G401", "severity": "", "description": "Weak crypto", "tags": ["security"]},
				"tool": {"name": "CodeQL", "version": "2.20.0"},
				"most_recent_instance": {
					"location": {"path": "main.go", "start_line": 14, "end_line": 14},
					"message": {"text": "Potential weak crypto"}
				}
			},
			{
				"number": 5,
				"state": "open",
				"rule": {"id": "gosec/G402", "severity": "critical", "description": "Weak hash", "tags": ["security"]},
				"tool": {"name": "Semgrep", "version": "1.0"},
				"most_recent_instance": {
					"location": {"path": "main.go", "start_line": 20, "end_line": 20},
					"message": {"text": "Different tool"}
				}
			}
		]`), nil
	})

	alerts, err := collectScanAlerts(SecurityTarget{DisplayName: "api", FullName: "acme/api"}, ScanCommandOptions{
		Selection: SecuritySelectionOptions{SeverityFilter: "medium"},
		ToolName:  "CodeQL",
	})
	if err != nil {
		t.Fatalf("collectScanAlerts: %v", err)
	}
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
		return []byte(`[{
			"number": 4,
			"state": "open",
			"rule": {"id": "gosec/G401", "severity": "", "description": "Weak crypto", "tags": ["security"]},
			"tool": {"name": "CodeQL", "version": "2.20.0"},
			"most_recent_instance": {"location": {"path": "main.go", "start_line": 14, "end_line": 14}, "message": {"text": "Potential weak crypto"}}
		}]`), nil
	})

	output := captureStdout(t, func() {
		if err := runScan(ScanCommandOptions{
			Selection: SecuritySelectionOptions{ExternalTarget: "acme/api", JSONOutput: true},
			ToolName:  "CodeQL",
		}); err != nil {
			t.Fatalf("runScan: %v", err)
		}
	})

	var rows []ScanAlert
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &rows); err != nil {
		t.Fatalf("runScan JSON output: %v\noutput: %s", err, output)
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

	err := runScan(ScanCommandOptions{Selection: SecuritySelectionOptions{RegistryPath: registryPath}})
	if err == nil {
		t.Fatal("expected multi-target partial failure to fail closed")
	}
	if !strings.Contains(err.Error(), "security scan failed") || !strings.Contains(err.Error(), "acme/web") {
		t.Fatalf("unexpected error: %v", err)
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
