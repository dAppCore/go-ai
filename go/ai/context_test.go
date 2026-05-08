// SPDX-License-Identifier: EUPL-1.2

package ai

import (
	"context"
	"testing"

	core "dappco.re/go"
	"dappco.re/go/inference"
)

func TestContext_RAGContextAssembler_Good_UsesLastUserMessage(t *testing.T) {
	assembler := RAGContextAssembler{
		Query: func(task TaskInfo) core.Result {
			if task.Title != "How do I fix this build?" {
				t.Fatalf("task title = %q, want last user message", task.Title)
			}
			return core.Ok("build runbook context")
		},
	}

	got, err := assembler.AssembleContext(context.Background(), []inference.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "How do I fix this build?"},
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if got != "build runbook context" {
		t.Fatalf("AssembleContext() = %q, want build runbook context", got)
	}
}

func TestContext_RAGContextAssembler_Bad_BlankMessagesSkipQuery(t *testing.T) {
	called := false
	assembler := RAGContextAssembler{
		Query: func(TaskInfo) core.Result {
			called = true
			return core.Ok("unexpected")
		},
	}

	got, err := assembler.AssembleContext(context.Background(), []inference.Message{{Role: "user", Content: "   "}})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if got != "" {
		t.Fatalf("AssembleContext() = %q, want empty context", got)
	}
	if called {
		t.Fatal("AssembleContext() called query for blank messages")
	}
}
