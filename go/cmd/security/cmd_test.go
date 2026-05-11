package security

import (
	"testing"

	"dappco.re/go"
)

func TestAddSecurityCommands_Good(t *testing.T) {
	root := core.New()

	if r := AddSecurityCommands(root); !r.OK {
		t.Fatalf("register security commands: %s", r.Error())
	}
	if r := AddSecurityCommands(root); !r.OK {
		t.Fatalf("register duplicate security commands: %s", r.Error())
	}

	commands := root.Commands()
	if len(commands) != 6 {
		t.Fatalf("expected 6 security command paths, got %d: %#v", len(commands), commands)
	}

	for _, path := range []string{
		"security",
		"security/alerts",
		"security/deps",
		"security/scan",
		"security/secrets",
		"security/jobs",
	} {
		if cmd := root.Command(path); !cmd.OK {
			t.Fatalf("find %s: %s", path, cmd.Error())
		}
	}
}

func TestAddSecurityCommands_Good_SubcommandsKeepFlagStateLocal(t *testing.T) {
	root := core.New()

	if r := AddSecurityCommands(root); !r.OK {
		t.Fatalf("register security commands: %s", r.Error())
	}

	alertsResult := root.Command("security/alerts")
	if !alertsResult.OK {
		t.Fatalf("find alerts command: %s", alertsResult.Error())
	}
	depsResult := root.Command("security/deps")
	if !depsResult.OK {
		t.Fatalf("find deps command: %s", depsResult.Error())
	}
	alertsCommand := alertsResult.Value.(*core.Command)
	depsCommand := depsResult.Value.(*core.Command)

	alertsCommand.Flags.Set("severity", "critical")
	alertsSeverity := alertsCommand.Flags.String("severity")
	depsSeverity := depsCommand.Flags.String("severity")

	if alertsSeverity != "critical" {
		t.Fatalf("alerts severity = %q, want %q", alertsSeverity, "critical")
	}
	if depsSeverity != "" {
		t.Fatalf("deps severity leaked shared state: got %q, want empty default", depsSeverity)
	}
}
