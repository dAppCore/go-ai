package security

import (
	"testing"

	"forge.lthn.ai/core/cli/pkg/cli"
)

func TestAddSecurityCommands_Good(t *testing.T) {
	root := &cli.Command{Use: "core"}

	AddSecurityCommands(root)
	AddSecurityCommands(root)

	commands := root.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(commands))
	}
	if commands[0].Name() != "security" {
		t.Fatalf("expected top-level command security, got %s", commands[0].Name())
	}

	for _, path := range [][]string{
		{"security", "alerts"},
		{"security", "deps"},
		{"security", "scan"},
		{"security", "secrets"},
		{"security", "jobs"},
	} {
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if cmd.Name() != path[len(path)-1] {
			t.Fatalf("expected %s, got %s", path[len(path)-1], cmd.Name())
		}
	}
}

func TestAddSecurityCommands_Good_SubcommandsKeepFlagStateLocal(t *testing.T) {
	root := &cli.Command{Use: "core"}

	AddSecurityCommands(root)

	alertsCommand, _, err := root.Find([]string{"security", "alerts"})
	if err != nil {
		t.Fatalf("find alerts command: %v", err)
	}
	depsCommand, _, err := root.Find([]string{"security", "deps"})
	if err != nil {
		t.Fatalf("find deps command: %v", err)
	}

	if err := alertsCommand.Flags().Set("severity", "critical"); err != nil {
		t.Fatalf("set alerts --severity: %v", err)
	}

	alertsSeverity, err := alertsCommand.Flags().GetString("severity")
	if err != nil {
		t.Fatalf("get alerts --severity: %v", err)
	}
	depsSeverity, err := depsCommand.Flags().GetString("severity")
	if err != nil {
		t.Fatalf("get deps --severity: %v", err)
	}

	if alertsSeverity != "critical" {
		t.Fatalf("alerts severity = %q, want %q", alertsSeverity, "critical")
	}
	if depsSeverity != "" {
		t.Fatalf("deps severity leaked shared state: got %q, want empty default", depsSeverity)
	}
}
