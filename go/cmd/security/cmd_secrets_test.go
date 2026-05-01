package security

import (
	"testing"

	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func TestCmdSecrets_collectSecretAlerts_Good(t *testing.T) {
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "repos/acme/api/secret-scanning/alerts?state=open" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(`[
			{
				"number": 9,
				"state": "open",
				"secret_type": "aws_access_key",
				"secret": "AKIA...",
				"push_protection_bypassed": true,
				"resolution": "revoked"
			},
			{
				"number": 10,
				"state": "resolved",
				"secret_type": "slack_token",
				"push_protection_bypassed": false,
				"resolution": "revoked"
			}
		]`), nil
	})

	alertsResult := collectSecretAlerts(SecurityTarget{DisplayName: "api", FullName: "acme/api"})
	if !alertsResult.OK {
		t.Fatalf("collectSecretAlerts: %s", alertsResult.Error())
	}
	alerts := alertsResult.Value.([]SecretAlert)
	if len(alerts) != 1 || alerts[0].Number != 9 || !alerts[0].PushProtection {
		t.Fatalf("unexpected secret alerts: %+v", alerts)
	}
}

func TestCmdSecrets_runSecrets_Good_JSONOutput(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		if endpoint != "repos/acme/api/secret-scanning/alerts?state=open" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		return []byte(`[{
			"number": 9,
			"state": "open",
			"secret_type": "aws_access_key",
			"push_protection_bypassed": true,
			"resolution": "revoked"
		}]`), nil
	})

	output := captureStdout(t, func() {
		if r := runSecrets(SecuritySelectionOptions{ExternalTarget: "acme/api", JSONOutput: true}); !r.OK {
			t.Fatalf("runSecrets: %s", r.Error())
		}
	})

	var rows []SecretAlert
	if r := core.JSONUnmarshal([]byte(core.Trim(output)), &rows); !r.OK {
		t.Fatalf("runSecrets JSON output: %v\noutput: %s", r.Error(), output)
	}
	if len(rows) != 1 || rows[0].Number != 9 || !rows[0].PushProtection {
		t.Fatalf("unexpected secret rows: %+v", rows)
	}
}

func TestCmdSecrets_runSecrets_Bad_MultiTargetPartialFailureFailsClosed(t *testing.T) {
	withSecurityTempHome(t)
	withFakeGitHubCLI(t)
	registryPath := writeSecurityRegistry(t, "acme", "api", "web")

	stubGitHubAPI(t, func(endpoint string) ([]byte, error) {
		switch endpoint {
		case "repos/acme/api/secret-scanning/alerts?state=open":
			return []byte(`[]`), nil
		case "repos/acme/web/secret-scanning/alerts?state=open":
			return nil, assertiveError("secret scanning unavailable")
		default:
			t.Fatalf("unexpected endpoint: %s", endpoint)
			return nil, nil
		}
	})

	r := runSecrets(SecuritySelectionOptions{RegistryPath: registryPath})
	if r.OK {
		t.Fatal("expected multi-target partial failure to fail closed")
	}
	if !core.Contains(r.Error(), "security secrets failed") || !core.Contains(r.Error(), "acme/web") {
		t.Fatalf("unexpected error: %s", r.Error())
	}
}

func TestCmdSecrets_addSecretsCommand_Good_BindsFlagsPerCommandInstance(t *testing.T) {
	firstRoot := &cli.Command{Use: "core"}
	secondRoot := &cli.Command{Use: "core"}

	addSecretsCommand(firstRoot)
	addSecretsCommand(secondRoot)

	firstCommand, _, err := firstRoot.Find([]string{"secrets"})
	if err != nil {
		t.Fatalf("find first secrets command: %v", err)
	}
	secondCommand, _, err := secondRoot.Find([]string{"secrets"})
	if err != nil {
		t.Fatalf("find second secrets command: %v", err)
	}

	if err := firstCommand.Flags().Set("json", "true"); err != nil {
		t.Fatalf("set first secrets json: %v", err)
	}

	firstJSON, err := firstCommand.Flags().GetBool("json")
	if err != nil {
		t.Fatalf("get first secrets json: %v", err)
	}
	secondJSON, err := secondCommand.Flags().GetBool("json")
	if err != nil {
		t.Fatalf("get second secrets json: %v", err)
	}

	if !firstJSON {
		t.Fatal("first secrets json flag should be true")
	}
	if secondJSON {
		t.Fatal("second secrets json flag leaked shared state")
	}
}
