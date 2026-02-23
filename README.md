[![Go Reference](https://pkg.go.dev/badge/forge.lthn.ai/core/go-ai.svg)](https://pkg.go.dev/forge.lthn.ai/core/go-ai)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](LICENSE.md)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)](go.mod)

# go-ai

MCP (Model Context Protocol) hub for the Lethean AI stack. Exposes 49 tools across file operations, directory management, language detection, RAG vector search, ML inference and scoring, process management, WebSocket streaming, browser automation via Chrome DevTools Protocol, JSONL metrics, and an IDE bridge to the Laravel core-agentic backend. The package is a pure library — the Core CLI (`core mcp serve`) imports it and handles transport selection (stdio, TCP, or Unix socket).

**Module**: `forge.lthn.ai/core/go-ai`
**Licence**: EUPL-1.2
**Language**: Go 1.25

## Quick Start

```go
import "forge.lthn.ai/core/go-ai/mcp"

svc, err := mcp.New(
    mcp.WithWorkspaceRoot("/path/to/project"),
    mcp.WithProcessService(ps),
)
// Run as stdio server (default for AI client subprocess integration)
err = svc.Run(ctx)
// Or TCP: MCP_ADDR=127.0.0.1:9100 triggers ServeTCP automatically
```

## Documentation

- [Architecture](docs/architecture.md) — MCP server, subsystem plugin model, tool inventory, IDE bridge, transports
- [Development Guide](docs/development.md) — building, testing, adding tools
- [Project History](docs/history.md) — completed phases and known limitations

## Build & Test

```bash
go test ./...
go test -race ./...
go build ./...
```

## Licence

European Union Public Licence 1.2 — see [LICENCE](LICENCE) for details.
