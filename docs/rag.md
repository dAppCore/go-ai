---
title: RAG Pipeline
description: Retrieval-augmented generation via Qdrant vector search and Ollama embeddings.
---

# RAG Pipeline

go-ai integrates with the RAG (Retrieval-Augmented Generation) pipeline provided by `go-rag`. This surfaces as three MCP tools for vector search and a high-level facade function for programmatic use.

## Architecture

```
MCP Client                           Programmatic callers
    |                                       |
    v                                       v
rag_query / rag_ingest / rag_collections    ai.QueryRAGForTask()
    |                                       |
    +----------- go-rag --------------------+
                    |              |
                    v              v
                Qdrant          Ollama
              (vectors)       (embeddings)
```

## MCP Tools

### `rag_query`

Query the vector database for documents relevant to a natural-language question.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `question` | `string` | Yes | Natural-language query |
| `collection` | `string` | No | Qdrant collection name (default: `hostuk-docs`) |
| `limit` | `int` | No | Maximum results to return (default: 3) |
| `threshold` | `float64` | No | Minimum similarity score (default: 0.5) |

The tool embeds the question via Ollama, searches Qdrant with the specified parameters, and returns formatted context with source references.

### `rag_ingest`

Ingest a file into the vector database. The file is chunked (for Markdown, this respects heading boundaries), each chunk is embedded via Ollama, and the resulting vectors are stored in Qdrant.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | `string` | Yes | Path to the file to ingest (relative to workspace root) |
| `collection` | `string` | No | Target Qdrant collection |

This tool is logged at `Security` level due to its write nature.

### `rag_collections`

List all available collections in the connected Qdrant instance, with point counts and vector dimensions.

## AI Facade: QueryRAGForTask

The `ai` package provides a higher-level wrapper for programmatic RAG queries. It is used by agentic task planners to enrich task context without importing `go-rag` directly.

```go
type TaskInfo struct {
    Title       string
    Description string
}

func QueryRAGForTask(task TaskInfo) (string, error) {
    query := task.Title + " " + task.Description

    // Truncate to 500 runes to keep the embedding focused
    runes := []rune(query)
    if len(runes) > 500 {
        query = string(runes[:500])
    }

    qdrantCfg := rag.DefaultQdrantConfig()
    qdrantClient, err := rag.NewQdrantClient(qdrantCfg)
    if err != nil {
        return "", fmt.Errorf("rag qdrant client: %w", err)
    }
    defer qdrantClient.Close()

    ollamaCfg := rag.DefaultOllamaConfig()
    ollamaClient, err := rag.NewOllamaClient(ollamaCfg)
    if err != nil {
        return "", fmt.Errorf("rag ollama client: %w", err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    results, err := rag.Query(ctx, qdrantClient, ollamaClient, query, rag.QueryConfig{
        Collection: "hostuk-docs",
        Limit:      3,
        Threshold:  0.5,
    })
    if err != nil {
        return "", fmt.Errorf("rag query: %w", err)
    }
    return rag.FormatResultsContext(results), nil
}
```

Key design decisions:
- The query is capped at **500 runes** to keep the embedding vector focused on the task's core intent
- A **10-second timeout** prevents hanging when services are slow
- The function returns an error rather than silently degrading, giving callers the choice of how to handle failures

## External Service Dependencies

### Qdrant

Vector database storing embedded document chunks.

- Default address: `localhost:6334` (gRPC)
- Configuration: `rag.DefaultQdrantConfig()`

### Ollama

Local LLM server providing embedding generation.

- Default address: `localhost:11434` (HTTP)
- Configuration: `rag.DefaultOllamaConfig()`
- Default embedding model: `nomic-embed-text`

Both services must be running for RAG tools to function. In CI, tests that touch RAG tools are guarded with `skipIfShort(t)`.

## Embedding Benchmark

The `cmd/embed-bench/` utility compares embedding models for the OpenBrain knowledge store. It tests how well models separate semantically related vs unrelated agent memory pairs.

```bash
go run ./cmd/embed-bench
go run ./cmd/embed-bench -ollama http://localhost:11434
```

The benchmark evaluates:
- **Cluster separation** -- intra-group vs inter-group similarity
- **Query recall accuracy** -- top-1 and top-3 retrieval precision
- **Embedding throughput** -- milliseconds per memory

Models tested: `nomic-embed-text` and `embeddinggemma`.

## Testing

RAG tool tests cover handler validation (empty question/path fields, default behaviour) and graceful degradation when Qdrant or Ollama are unavailable. Full RAG round-trip tests require live services and are skipped in short mode.
