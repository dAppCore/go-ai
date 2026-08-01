<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/context.go — RAG context assembler adapter

**Package**: `dappco.re/go/ai/ai`
**File**: `go/ai/context.go`

## What this is

The **adapter** that wraps the package's RAG helper into a `ProviderContextAssembler` — the interface the ProviderRouter calls to inject context before sending a prompt to a provider.

A small file (~50 LOC) that bridges two existing shapes; no policy of its own.

## RAGContextAssembler

```go
type RAGContextAssembler struct {
    Task  TaskInfo                        // task hint that shapes the query
    Query func(TaskInfo) core.Result      // injected query function — defaults to ai.QueryRAGForTask
}

func (a RAGContextAssembler) AssembleContext(ctx, []Message) (string, error)
```

Constructed by:

```go
assembler := ai.RAGContextAssembler{
    Task: ai.TaskInfo{
        Title:       "Diagnose build failure",
        Description: chatHistorySummary,
    },
}

router.Chat(ctx, ai.ProviderChatRequest{
    Messages:         chatMessages,
    ContextAssembler: assembler,
    ContextRole:      "system",
    ContextPrefix:    "Relevant docs:\n",
})
```

The router calls `AssembleContext(ctx, messages)`; the assembler runs the RAG query; the result text is prepended to messages with the configured role and prefix.

## Why a separate file

Two reasons:

1. **Dependency direction.** `provider_router.go` doesn't import `rag.go` — the router defines the assembler interface; this file provides one concrete impl. Other impls (BookState in `book_state_demo.go`) can live elsewhere without circular imports.
2. **Testability.** The `Query` field is a function so tests can stub the RAG path without standing up a Qdrant fixture.

## Used by

- `mcp/tools_core.go` — agent tools that need doc context
- `cmd/lem-desktop/agent_runner.go` — agent loop pre-call hook
- ad-hoc CLI commands that want RAG-backed responses

## Related

- [provider_router.md](provider_router.md) — `ProviderContextAssembler` interface
- [rag.md](rag.md) — the RAG facade this wraps
- [book_state_demo.md](book_state_demo.md) — the BookState assembler (parallel concrete impl)
