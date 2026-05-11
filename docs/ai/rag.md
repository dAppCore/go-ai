<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/rag.go — RAG facade for task-scoped doc lookup

**Package**: `dappco.re/go/ai/ai`
**File**: `go/ai/rag.go`

## What this is

Thin facade over `dappco.re/go/rag` (Qdrant + Ollama embeddings) for **task-scoped documentation retrieval**. Used by agent flows that want "find me docs relevant to this task" without binding to a specific vector store.

Returns a `core.Result` whose value is a single string (top-K results, formatted) — easy to inject as context into a prompt.

## Constants

```go
ragTaskCollection          = "hostuk-docs"  // Qdrant collection name
ragTaskResultLimit         = 3              // top-K
ragTaskSimilarityThreshold = 0.5            // cosine cutoff
ragTaskQueryRuneLimit      = 500            // truncate task descriptions
```

The 500-rune truncation is the load-bearing limit: long task descriptions degrade retrieval quality (the embedding starts averaging across unrelated concerns). 500 runes ≈ one paragraph of focused intent.

## TaskInfo

```go
type TaskInfo struct {
    Title       string
    Description string
    Labels      map[string]string
}
```

The query shape — title + description in, embedding query out. Labels (not used today) are reserved for filtering hits (e.g. exclude legacy docs).

## QueryRAGForTask

```go
result := ai.QueryRAGForTask(task)
if !result.OK { return result }    // returns empty string on graceful failure
text := result.Value.(string)
```

Returns top-3 hits above 0.5 similarity, joined with newlines. **Failure mode is graceful** — Qdrant unreachable, no hits above threshold, embedding service down → returns empty string with `OK: false`, but the calling agent can choose to proceed without context rather than fail.

## Dependency injection

```go
var (
    newQdrantClient = func(cfg rag.QdrantConfig) (*rag.QdrantClient, error) { /* default */ }
    newEmbedder     = func(cfg rag.OllamaConfig) (*rag.OllamaEmbedder, error) { /* default */ }
)
```

These are package vars that tests override. Production uses the default factory functions; tests stub them with in-memory fakes. This is the pattern across the package — explicit DI via package vars, not interface plumbing through every call site.

## Why "task-scoped" not "general-purpose"

The collection name + similarity threshold + result limit are tuned for "agent investigating a task wants 1-3 paragraphs of relevant docs". A general-purpose RAG client would expose more knobs; this facade picks defaults that work for the agent use case and hides the rest.

For full-control RAG, callers reach for `dappco.re/go/rag` directly — this facade is the convenience layer.

## Used by

- `RAGContextAssembler` ([context.md](context.md)) — wraps this for ProviderRouter
- `cmd/lem-desktop/agent_runner.go` — agent loop pre-call
- `mcp/tools_core.go` — RAG-backed MCP tool

## Why this lives in go-ai not in go-rag

`go-rag` is the **engine** (Qdrant + Ollama clients, generic similarity). `ai/rag.go` is the **task-scoped recipe** that picks the collection / threshold / limit. Different packages, different audiences:

- A library writer wanting custom RAG → imports go-rag
- The agent wanting "find docs for this task" → imports this facade

## Related

- `dappco.re/go/rag` — the Qdrant + embedding engine
- [context.md](context.md) — adapter wrapping this for ProviderRouter
- [provider_router.md](provider_router.md) — consumer of the assembler
