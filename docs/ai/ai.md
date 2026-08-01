<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/ai.go — package entry doc + canonical re-export

**Package**: `dappco.re/go/ai/ai`
**File**: `go/ai/ai.go`

## What this is

The package doc comment that lays out the `ai` subpackage's role + a canonical usage example. The actual API lives in the sibling files (metrics, rag, context, provider_router, book_state_demo); this file is the entry point a `go doc` reader hits first.

## Package role

Canonical AI facade for the Core CLI + CoreAgent. Sits **above** the inference contracts in `go-inference`, **above** the native runtimes (go-mlx) and external providers (`providers/openai`), and provides:

- Provider routing with fallback policy ([`provider_router.md`](provider_router.md))
- Demo orchestration ([`book_state_demo.md`](book_state_demo.md))
- Context injection ([`context.md`](context.md))
- RAG facade over `dappco.re/go/rag` Qdrant client ([`rag.md`](rag.md))
- Event metrics ([`metrics.md`](metrics.md))

## Canonical example

```go
ctx, err := ai.QueryRAGForTask(ai.TaskInfo{
    Title:       "Investigate build failure",
    Description: "CI compile step fails",
})
if err != nil { return err }

ai.Record(ai.Event{Type: "security.scan", Repo: "wailsapp/wails"})
```

The example walks the two pre-router lanes — RAG retrieval and metric emission — that pre-date the BookState / ProviderRouter pieces. Both still work; they're foundational shapes for the higher-level orchestration.

## What lives here vs what doesn't

| In `ai/` | NOT in `ai/` |
|----------|--------------|
| provider routing policy | provider implementations (those live in `providers/`) |
| RAG facade (calls `dappco.re/go/rag`) | the Qdrant client itself (lives in go-rag) |
| BookState demo (high-level) | KV bundle production (lives in go-mlx) |
| context assemblers | the actual generation (lives in go-mlx / external providers) |
| event metric storage (JSONL) | LLM judging / scoring (lives in `go-ml`) |

## Related

- Sibling docs: [provider_router.md](provider_router.md), [book_state_demo.md](book_state_demo.md), [context.md](context.md), [rag.md](rag.md), [metrics.md](metrics.md)
- [../README.md](../README.md) — go-ai package overview
