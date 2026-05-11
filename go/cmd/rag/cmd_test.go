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
