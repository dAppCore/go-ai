# CLAUDE.md

This file provides guidance to Claude Code when working with the go-ai repository.

## Project Overview

**go-ai** is the MCP (Model Context Protocol) hub for the Lethean AI stack. It exposes 49 tools across file operations, RAG vector search, ML inference/scoring, process management, WebSocket streaming, browser automation, metrics, and IDE integration.

- **Module path**: `forge.lthn.ai/core/go-ai`
- **Language**: Go 1.25
- **Licence**: EUPL-1.2
- **LOC**: ~5.6K total (~3.5K non-test)

The MCP server is started by the Core CLI (`core mcp serve`) which imports `forge.lthn.ai/core/go-ai/mcp`.

See `docs/` for full architecture, tool reference, development guide, and project history.

## Build & Test Commands

```bash
go test ./...                       # Run all tests
go test -run TestName ./mcp/...     # Run a single test
go test -v -race ./...              # Verbose with race detector
go build ./...                      # Build (library — no main package)
go vet ./...                        # Vet
```

## Coding Standards

- **UK English** in comments and user-facing strings (colour, organisation, centre)
- **Conventional commits**: `type(scope): description`
- **Co-Author**: `Co-Authored-By: Virgil <virgil@lethean.io>`
- **Error handling**: Return wrapped errors with context, never panic
- **Test naming**: `_Good` (happy path), `_Bad` (expected errors), `_Ugly` (panics/edge cases)
- **Licence**: EUPL-1.2
