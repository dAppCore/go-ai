package ai

import (
	core "dappco.re/go"
	"testing"

	"dappco.re/go/cli/pkg/cli"
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

// --- AX-7 canonical triplets ---

func TestCmd_AddAICommands_Good(t *core.T) {
	root := &cli.Command{Use: "core"}
	AddAICommands(root)
	cmd, _, err := root.Find([]string{"ai"})

	core.AssertNoError(t, err)
	core.AssertEqual(t, "ai", cmd.Name())
}

func TestCmd_AddAICommands_Bad(t *core.T) {
	root := &cli.Command{Use: "core"}
	AddAICommands(root)
	AddAICommands(root)

	core.AssertLen(t, root.Commands(), 1)
	core.AssertEqual(t, "ai", root.Commands()[0].Name())
}

func TestCmd_AddAICommands_Ugly(t *core.T) {
	root := &cli.Command{Use: "core"}
	root.AddCommand(&cli.Command{Use: "ai"})
	AddAICommands(root)

	core.AssertLen(t, root.Commands(), 1)
	core.AssertEqual(t, "ai", root.Commands()[0].Name())
}
