package rag

import "testing"

func TestCmdRAG_Good_ReexportsSubcommands(t *testing.T) {
	if AddRAGSubcommands == nil {
		t.Fatal("expected AddRAGSubcommands to be wired to go-rag")
	}
}
