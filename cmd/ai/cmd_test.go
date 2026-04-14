package ai

import (
	"testing"

	"dappco.re/go/core/cli/pkg/cli"
)

func TestAddAICommands_Good(t *testing.T) {
	root := &cli.Command{Use: "core"}

	AddAICommands(root)
	AddAICommands(root)

	commands := root.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(commands))
	}
	if commands[0].Name() != "ai" {
		t.Fatalf("expected top-level command ai, got %s", commands[0].Name())
	}

	for _, path := range [][]string{
		{"ai", "metrics"},
		{"ai", "rag"},
		{"ai", "ml"},
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
