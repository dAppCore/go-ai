<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/ — facade, routing, demos

**Package**: `dappco.re/go/ai/ai`

## What this package owns

The **AI facade** consumed by the Core CLI and CoreAgent. Above the inference contracts (`go-inference`), above the native runtimes (`go-mlx`) and outbound providers (`providers/openai`), this package provides:

- Provider routing with fallback policy
- Context injection (RAG, BookState)
- Demo orchestration (BookState teacher/student)
- RAG retrieval facade over `dappco.re/go/rag`
- Event metrics (append-only JSONL)

## File map

| File | Doc | Role |
|------|-----|------|
| `ai.go` | [ai.md](ai.md) | Package doc + canonical example |
| `provider_router.go` | [provider_router.md](provider_router.md) | Fallback router across multiple providers |
| `book_state_demo.go` | [book_state_demo.md](book_state_demo.md) | Teacher/student book-state demo |
| `book_state_demo_http.go` | [book_state_demo_http.md](book_state_demo_http.md) | JSON HTTP handler for the demo |
| `context.go` | [context.md](context.md) | RAG context assembler adapter |
| `rag.go` | [rag.md](rag.md) | Task-scoped RAG facade |
| `metrics.go` | [metrics.md](metrics.md) | Append-only event log |

## Boundary

This package's lane:

| In `ai/` | NOT in `ai/` |
|----------|--------------|
| Provider routing **policy** | provider **implementations** (those live in `providers/`) |
| RAG **facade** (calls `dappco.re/go/rag`) | the Qdrant **client itself** (lives in go-rag) |
| BookState demo **orchestration** | KV bundle **production** (lives in go-mlx) |
| Context **assemblers** | the actual **generation** (lives in go-mlx + external) |
| Event **metric storage** (JSONL) | LLM **judging / scoring** (lives in go-ml) |

## Related

- [../providers/openai.md](../providers/openai.md) — outbound provider implementation
- [../mcp/service.md](../mcp/service.md) — MCP server that uses these
- [../cmd/book-state-demo.md](../cmd/book-state-demo.md) — entry binary for the demo
- `../../../go-inference/docs/inference/inference.md` — TextModel contract underneath
- `../../../go-mlx/docs/memory/agent_memory.md` — what produces the BookState data
