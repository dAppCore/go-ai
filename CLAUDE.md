# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**go-ai** is a thin facade layer in the Lethean AI stack. After a March 2026 refactor, the MCP server and all 49 tools were extracted to `forge.lthn.ai/core/mcp`. What remains here is the AI metrics system, a RAG query wrapper, and CLI command wrappers that delegate to other modules.

- **Module path**: `forge.lthn.ai/core/go-ai`
- **Language**: Go 1.26
- **Licence**: EUPL-1.2

## Build & Test Commands

```bash
go build forge.lthn.ai/core/go-ai/...            # Build (library — no main package)
go test forge.lthn.ai/core/go-ai/...             # Run all tests
go test -run TestName forge.lthn.ai/core/go-ai/ai  # Run a single test
go test -v -race forge.lthn.ai/core/go-ai/...    # Verbose with race detector
go test -bench=. forge.lthn.ai/core/go-ai/ai     # Run benchmarks (metrics)
go vet forge.lthn.ai/core/go-ai/...              # Vet
golangci-lint run ./...                           # Lint (from module root)
```

## Architecture

### `ai/` — Core facade package

Two concerns, no external service calls at import time:

1. **Metrics** (`metrics.go`) — Append-only JSONL event storage at `~/.core/ai/metrics/YYYY-MM-DD.jsonl`. Thread-safe via `sync.Mutex`. Key functions: `Record(Event)`, `ReadEvents(since)`, `Summary([]Event)`.

2. **RAG** (`rag.go`) — `QueryRAGForTask(TaskInfo)` wraps `go-rag` to query Qdrant for documentation context. Truncates to 500 runes, returns top-3 results above 0.5 threshold. Returns error (not empty string) on failure for graceful degradation at call sites.

### `cmd/` — CLI command wrappers

Each subpackage exposes an `Add*Command(root)` function that registers cobra commands. They delegate to other modules:

| Subpackage | Delegates to |
|---|---|
| `embed-bench/` | Ollama API — embedding model benchmarking tool |
| `lab/` | `forge.lthn.ai/lthn/lem/pkg/lab` — homelab monitoring dashboard |
| `metrics/` | `ai.ReadEvents()` / `ai.Summary()` |
| `rag/` | `forge.lthn.ai/core/go-rag/cmd/rag` (re-export) |
| `security/` | GitHub API via `gh` CLI (alerts, deps, secrets, scanning) |

### Key sibling modules

The MCP server and tools live in separate modules. When working on tool registration or transport, you need `core/mcp`, not this repo.

- `forge.lthn.ai/core/mcp` — MCP server, transports, tool registration, IDE bridge
- `forge.lthn.ai/core/go-rag` — Qdrant vector DB + Ollama embeddings
- `forge.lthn.ai/core/go-ml` — Scoring engine, heuristics, probes
- `forge.lthn.ai/core/go-inference` — Shared ML backend interfaces
- `forge.lthn.ai/core/cli` — CLI framework (`cli.Command`, command registration)
- `forge.lthn.ai/core/go-i18n` — Internationalisation strings

## Coding Standards

- **UK English** in comments and user-facing strings (colour, organisation, centre)
- **Conventional commits**: `type(scope): description`
- **Co-Author**: `Co-Authored-By: Virgil <virgil@lethean.io>`
- **Error handling**: Use `coreerr.E("pkg.Func", "what failed", err)` from `go-log`, never `fmt.Errorf` or panic
- **Test naming**: `TestFoo_Good` (happy path), `TestFoo_Bad` (expected errors), `TestFoo_Ugly` (panics/edge cases)
- **Licence**: EUPL-1.2 (SPDX header on new files)
