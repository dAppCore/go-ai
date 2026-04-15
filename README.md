[![Go Reference](https://pkg.go.dev/badge/dappco.re/go/ai.svg)](https://pkg.go.dev/dappco.re/go/ai)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](LICENSE.md)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)](go.mod)

# go-ai

Unified AI surface for the Core CLI. The library composes a thin AI facade, JSONL metrics logging, and a RAG query wrapper, while the CLI command packages expose metrics reporting, GitHub security scanning, RAG subcommands, a local lab dashboard, and the embedding benchmark binary.

**Module**: `dappco.re/go/ai`
**Licence**: EUPL-1.2
**Language**: Go 1.26

## Quick Start

```go
import "dappco.re/go/ai/ai"

contextText, err := ai.QueryRAGForTask(ai.TaskInfo{
    Title:       "Investigate build failure",
    Description: "CI compile step fails",
})
```

## Documentation

- [Architecture](docs/architecture.md) — package layout, metrics flow, RAG facade, security commands
- [Development Guide](docs/development.md) — building, testing, and extending the command surface
- [Project History](docs/history.md) — completed phases and known limitations

## Build & Test

```bash
go test ./...
go test -race ./...
go build ./...
```

## Licence

European Union Public Licence 1.2 — see [LICENCE](LICENCE) for details.
