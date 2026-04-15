//go:build ignore
// +build ignore

package lab

import (
	"testing"

	"forge.lthn.ai/core/cli/pkg/cli"
)

func TestCmdLab_HasCommand_Good(t *testing.T) {
	root := &cli.Command{Use: "core"}
	root.AddCommand(&cli.Command{Use: "lab"})

	if !hasCommand(root, "lab") {
		t.Fatal("expected hasCommand to detect existing lab command")
	}
	if hasCommand(root, "missing") {
		t.Fatal("expected hasCommand to ignore missing command")
	}
}

func TestCmdLab_AddLabCommands_Good(t *testing.T) {
	root := &cli.Command{Use: "core"}

	AddLabCommands(root)
	AddLabCommands(root)

	commands := root.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected one lab command, got %d", len(commands))
	}
	if commands[0].Name() != "lab" {
		t.Fatalf("expected top-level command lab, got %s", commands[0].Name())
	}

	cmd, _, err := root.Find([]string{"lab", "serve"})
	if err != nil {
		t.Fatalf("find lab serve command: %v", err)
	}
	if cmd.Name() != "serve" {
		t.Fatalf("expected serve subcommand, got %s", cmd.Name())
	}
}
