---
title: go-ai Overview
description: The AI integration hub for the Lethean Go ecosystem — MCP server, metrics, and facade.
---

# go-ai

**Module**: `forge.lthn.ai/core/go-ai`
**Language**: Go 1.26
**Licence**: EUPL-1.2

go-ai is the **integration hub** for the Lethean AI stack. It imports specialised modules and exposes them as a unified MCP server with IDE bridge support, metrics recording, and a thin AI facade.

## Architecture

```
AI Clients (Claude, Cursor, any MCP-capable IDE)
          |  MCP JSON-RPC (stdio / TCP / Unix)
          v
  [ go-ai MCP Server ]          <-- this module
      |        |        |
      |        |        +-- ide/ subsystem --> Laravel core-agentic (WebSocket)
      |        +-- go-rag -----------------> Qdrant + Ollama
      +-- go-ml ---------------------------> inference backends (go-mlx, go-rocm, ...)

  Core CLI (forge.lthn.ai/core/cli) bootstraps and wires everything
```

go-ai is a pure library module. It contains no `main` package. The Core CLI (`core mcp serve`) imports `forge.lthn.ai/core/go-ai/mcp`, constructs a `mcp.Service`, and calls `Run()`.

## Package Layout

```
go-ai/
+-- ai/                          # AI facade: RAG queries and JSONL metrics
|   +-- ai.go                   # Package documentation and composition overview
|   +-- rag.go                  # QueryRAGForTask() with graceful degradation
|   +-- metrics.go              # Event, Record(), ReadEvents(), Summary()
|
+-- cmd/                         # CLI command registrations
|   +-- daemon/                  # core daemon (MCP server lifecycle)
|   +-- metrics/                 # core ai metrics viewer
|   +-- rag/                     # re-exports go-rag CLI commands
|   +-- security/                # security scanning tools (deps, alerts, secrets, scan, jobs)
|   +-- lab/                     # homelab monitoring dashboard
|   +-- embed-bench/             # embedding model benchmark utility
|
+-- docs/                        # This documentation
```

The MCP server and all its tool subsystems are provided by the separate `forge.lthn.ai/core/mcp` module. go-ai wires that server together with the `ai/` facade and the CLI command registrations.

## Imported Modules

| Module | Purpose |
|--------|---------|
| `forge.lthn.ai/core/go-ml` | Inference backends, scoring engine |
| `forge.lthn.ai/core/go-rag` | Vector search, embeddings |
| `forge.lthn.ai/core/go-inference` | Shared TextModel/Backend interfaces |
| `forge.lthn.ai/core/go-process` | Process lifecycle management |
| `forge.lthn.ai/core/go-log` | Structured logging with security levels |
| `forge.lthn.ai/core/go-io` | Sandboxed filesystem abstraction |
| `forge.lthn.ai/core/go-i18n` | Internationalisation |

## Quick Start

go-ai is not run directly. It is consumed by the Core CLI:

```bash
# Start the MCP server on stdio (default)
core mcp serve

# Start on TCP
core mcp serve --mcp-transport tcp --mcp-addr 127.0.0.1:9100

# Run as a background daemon
core daemon start

# View AI metrics
core ai metrics --since 7d
```

## Documentation

| Page | Description |
|------|-------------|
| [MCP Server](mcp-server.md) | Protocol implementation, transports, tool registration |
| [ML Pipeline](ml-pipeline.md) | ML scoring, model management, inference backends |
| [RAG Pipeline](rag.md) | Retrieval-augmented generation, vector search |
| [Agentic Client](agentic.md) | Security scanning, metrics, CLI commands |
| [IDE Bridge](ide-bridge.md) | IDE integration, WebSocket bridge to Laravel |

## Build and Test

```bash
go test ./...                       # Run all tests
go test -run TestName ./...         # Run a single test
go test -v -race ./...              # Verbose with race detector
go build ./...                      # Verify compilation (library -- no binary)
go vet ./...                        # Vet
```

Tests follow the `_Good`, `_Bad`, `_Ugly` suffix convention:

- `_Good` -- Happy path, valid input
- `_Bad` -- Expected error conditions
- `_Ugly` -- Panics and edge cases

## Dependencies

### Direct

| Module | Role |
|--------|------|
| `forge.lthn.ai/core/cli` | CLI framework (cobra-based command registration) |
| `forge.lthn.ai/core/go-api` | API server framework |
| `forge.lthn.ai/core/go-i18n` | Internationalisation strings |
| `forge.lthn.ai/core/go-inference` | Shared inference interfaces |
| `forge.lthn.ai/core/go-io` | Filesystem abstraction |
| `forge.lthn.ai/core/go-log` | Structured logging |
| `forge.lthn.ai/core/go-ml` | ML scoring and inference |
| `forge.lthn.ai/core/go-process` | Process lifecycle |
| `forge.lthn.ai/core/go-rag` | RAG pipeline |
| `github.com/modelcontextprotocol/go-sdk` | MCP Go SDK |
| `github.com/gorilla/websocket` | WebSocket client (IDE bridge) |
| `github.com/gin-gonic/gin` | HTTP router |

### Indirect (via go-ml and go-rag)

`go-mlx`, `go-rocm`, `go-duckdb`, `parquet-go`, `ollama`, `qdrant/go-client`, and the Arrow ecosystem are transitive dependencies not imported directly by go-ai.
