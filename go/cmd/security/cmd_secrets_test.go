package security

import (
	"testing"

	core "dappco.re/go"
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
	firstRoot := core.New()
	secondRoot := core.New()

	if r := addSecretsCommand(firstRoot, "secrets"); !r.OK {
		t.Fatalf("register first secrets command: %s", r.Error())
	}
	if r := addSecretsCommand(secondRoot, "secrets"); !r.OK {
		t.Fatalf("register second secrets command: %s", r.Error())
	}

	firstResult := firstRoot.Command("secrets")
	if !firstResult.OK {
		t.Fatalf("find first secrets command: %s", firstResult.Error())
	}
	secondResult := secondRoot.Command("secrets")
	if !secondResult.OK {
		t.Fatalf("find second secrets command: %s", secondResult.Error())
	}
	firstCommand := firstResult.Value.(*core.Command)
	secondCommand := secondResult.Value.(*core.Command)

	firstCommand.Flags.Set("json", true)
	firstJSON := firstCommand.Flags.Bool("json")
	secondJSON := secondCommand.Flags.Bool("json")

	if !firstJSON {
		t.Fatal("first secrets json flag should be true")
	}
	if secondJSON {
		t.Fatal("second secrets json flag leaked shared state")
	}
}
