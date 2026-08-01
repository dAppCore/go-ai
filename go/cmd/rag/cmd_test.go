package rag

import (
	"testing"

	"dappco.re/go"
)

func TestCmdRAG_Good_ReexportsSubcommands(t *testing.T) {
	root := core.New()
	if r := AddRAGSubcommands(root, "ai/rag"); !r.OK {
		t.Fatalf("register RAG subcommands: %s", r.Error())
	}
	if r := root.Command("ai/rag"); !r.OK {
		t.Fatalf("expected ai/rag command to be registered: %s", r.Error())
	}
}

func TestCmd_AddRAGSubcommands_Good(t *testing.T) {
	root := core.New()
	result := AddRAGSubcommands(root, "ai/rag")

	if !result.OK {
		t.Fatalf("AddRAGSubcommands() error = %s", result.Error())
	}
	if command := root.Command("ai/rag"); !command.OK {
		t.Fatalf("AddRAGSubcommands() did not register command: %s", command.Error())
	}
}

func TestCmd_AddRAGSubcommands_Bad(t *testing.T) {
	root := core.New()
	result := AddRAGSubcommands(root, "/rag")

	if result.OK {
		t.Fatal("AddRAGSubcommands() OK = true, want invalid path failure")
	}
}

func TestCmd_AddRAGSubcommands_Ugly(t *testing.T) {
	root := core.New()
	first := AddRAGSubcommands(root)
	second := AddRAGSubcommands(root)

	if !first.OK || !second.OK {
		t.Fatalf("AddRAGSubcommands() idempotent registration failed: first=%s second=%s", first.Error(), second.Error())
	}
	if commands := root.Commands(); len(commands) != 1 || commands[0] != "rag" {
		t.Fatalf("AddRAGSubcommands() commands = %#v, want one rag command", commands)
	}
}
