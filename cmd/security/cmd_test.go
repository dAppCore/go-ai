package security

import (
	"testing"

	"dappco.re/go/core/cli/pkg/cli"
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
