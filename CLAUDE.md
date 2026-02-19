# CLAUDE.md

This file provides guidance to Claude Code when working with the go-ai repository.

## Project Overview

**go-ai** is the MCP (Model Context Protocol) hub for the Lethean AI stack. It exposes 49 tools across file operations, RAG vector search, ML inference/scoring, process management, WebSocket streaming, browser automation, metrics, and IDE integration.

- **Module path**: `forge.lthn.ai/core/go-ai`
- **Language**: Go 1.25
- **Licence**: EUPL-1.2
- **LOC**: ~5.6K total (~3.5K non-test)
- **Tests**: 84 passing (unit + transport)

The orchestrator is **Virgil in core/go** — the Core CLI that bootstraps go-ai's MCP server and wires subsystems together.

## Build & Test Commands

```bash
# Run all tests
go test ./...

# Run a single test
go test -run TestName ./mcp/...

# Verbose
go test -v ./...

# Build (library — no main package in this module)
go build ./...

# Vet
go vet ./...
```

There is no `main` package in this module. The MCP server is started by the Core CLI (`core mcp serve`) which imports `forge.lthn.ai/core/go-ai/mcp`.

## Architecture

### MCP Server (`mcp/`)

`mcp.Service` is the central server. It owns:

- An `*mcp.Server` (from the MCP Go SDK)
- A sandboxed filesystem `io.Medium` for file operations
- Optional `process.Service` and `ws.Hub` for process management and WebSocket streaming
- A slice of `Subsystem` plugins registered via `WithSubsystem()`

Construction uses the functional options pattern:

```go
svc, err := mcp.New(
    mcp.WithWorkspaceRoot("/path/to/workspace"),
    mcp.WithProcessService(ps),
    mcp.WithWSHub(hub),
    mcp.WithSubsystem(ide.New(hub)),
    mcp.WithSubsystem(mcp.NewMLSubsystem(mlSvc)),
)
```

### Subsystem Interface

Plugins implement `Subsystem` to register additional tools:

```go
type Subsystem interface {
    Name() string
    RegisterTools(server *mcp.Server)
}
```

Optional graceful shutdown:

```go
type SubsystemWithShutdown interface {
    Subsystem
    Shutdown(ctx context.Context) error
}
```

### Tool Registration Pattern

Tools are registered via `mcp.AddTool()` with typed input/output structs:

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "tool_name",
    Description: "What it does",
}, s.handlerFunc)
```

Handler signature: `func(ctx, *CallToolRequest, InputStruct) (*CallToolResult, OutputStruct, error)`

### Transports

The server supports three transports:

| Transport | Method | Trigger |
|-----------|--------|---------|
| Stdio | `ServeStdio()` | Default (no `MCP_ADDR`) |
| TCP | `ServeTCP()` | `MCP_ADDR=:9100` |
| Unix | `ServeUnix()` | Explicit socket path |

`Run()` auto-selects Stdio or TCP based on `MCP_ADDR`.

## Package Layout

```
go-ai/
├── ai/                    # Facade: RAG queries, metrics recording (JSONL)
│   ├── ai.go              # Package doc
│   ├── rag.go             # QueryRAGForTask() — graceful degradation
│   └── metrics.go         # Event, Record(), ReadEvents(), Summary()
├── mcp/                   # MCP server and built-in tools
│   ├── mcp.go             # Service, New(), registerTools (10 file/dir/lang tools)
│   ├── subsystem.go       # Subsystem + SubsystemWithShutdown interfaces
│   ├── tools_rag.go       # rag_query, rag_ingest, rag_collections
│   ├── tools_ml.go        # MLSubsystem: ml_generate, ml_score, ml_probe, ml_status, ml_backends
│   ├── tools_metrics.go   # metrics_record, metrics_query
│   ├── tools_process.go   # process_start/stop/kill/list/output/input
│   ├── tools_ws.go        # ws_start, ws_info
│   ├── tools_webview.go   # webview_connect/disconnect/navigate/click/type/query/console/eval/screenshot/wait
│   ├── transport_stdio.go # Stdio transport
│   ├── transport_tcp.go   # TCP transport with per-connection server instances
│   ├── transport_unix.go  # Unix domain socket transport
│   └── ide/               # IDE subsystem (bridges to Laravel)
│       ├── ide.go         # Subsystem impl, RegisterTools, Shutdown
│       ├── config.go      # Config, DefaultConfig, Options
│       ├── bridge.go      # WebSocket bridge to Laravel core-agentic
│       ├── tools_chat.go  # ide_chat_send/history, ide_session_list/create, ide_plan_status
│       ├── tools_build.go # ide_build_status/list/logs
│       └── tools_dashboard.go # ide_dashboard_overview/activity/metrics
└── test-mlx.go            # Standalone scoring pipeline test script
```

## Tool Inventory (49 tools)

| Group | Tools | Source |
|-------|-------|--------|
| File ops | file_read, file_write, file_delete, file_rename, file_exists, file_edit | mcp.go |
| Directory | dir_list, dir_create | mcp.go |
| Language | lang_detect, lang_list | mcp.go |
| RAG | rag_query, rag_ingest, rag_collections | tools_rag.go |
| ML | ml_generate, ml_score, ml_probe, ml_status, ml_backends | tools_ml.go (MLSubsystem) |
| Metrics | metrics_record, metrics_query | tools_metrics.go |
| Process | process_start, process_stop, process_kill, process_list, process_output, process_input | tools_process.go |
| WebSocket | ws_start, ws_info | tools_ws.go |
| Webview | webview_connect, webview_disconnect, webview_navigate, webview_click, webview_type, webview_query, webview_console, webview_eval, webview_screenshot, webview_wait | tools_webview.go |
| IDE Chat | ide_chat_send, ide_chat_history, ide_session_list, ide_session_create, ide_plan_status | ide/tools_chat.go |
| IDE Build | ide_build_status, ide_build_list, ide_build_logs | ide/tools_build.go |
| IDE Dashboard | ide_dashboard_overview, ide_dashboard_activity, ide_dashboard_metrics | ide/tools_dashboard.go |

## Dependencies

| Module | Role |
|--------|------|
| `forge.lthn.ai/core/go` | Core framework: `pkg/io` (filesystem), `pkg/log`, `pkg/process`, `pkg/ws`, `pkg/webview` |
| `forge.lthn.ai/core/go-ml` | ML scoring engine: heuristic scores, judge backend, probes, InfluxDB status |
| `forge.lthn.ai/core/go-rag` | RAG: Qdrant vector DB client, Ollama embeddings, markdown chunking |
| `forge.lthn.ai/core/go-mlx` | Native Metal GPU inference (Apple Silicon) |
| `github.com/modelcontextprotocol/go-sdk` | MCP Go SDK (server, transports, JSON-RPC) |
| `github.com/gorilla/websocket` | WebSocket client for IDE bridge |
| `github.com/marcboeker/go-duckdb` | DuckDB (analytics) |
| `github.com/qdrant/go-client` | Qdrant gRPC client |
| `github.com/ollama/ollama` | Ollama API types |

All `forge.lthn.ai/core/*` dependencies use `replace` directives pointing to local sibling directories during development.

## Integration Points

- **Core CLI** (`core/go`): Bootstraps the MCP Service, wires subsystems, starts transports
- **Laravel core-agentic**: IDE bridge connects via WebSocket (`ws://localhost:9876/ws`) to forward chat, build, and dashboard data
- **Qdrant + Ollama**: RAG tools query vectors and generate embeddings
- **Chrome DevTools Protocol**: Webview tools automate browser via CDP
- **JSONL metrics**: Stored at `~/.core/ai/metrics/YYYY-MM-DD.jsonl`

## Coding Standards

- **UK English** in comments and user-facing strings (colour, organisation, centre)
- **Conventional commits**: `type(scope): description`
- **Co-Author**: `Co-Authored-By: Virgil <virgil@lethean.io>`
- **Error handling**: Return wrapped errors with context, never panic
- **Test naming**: `_Good` (happy path), `_Bad` (expected errors), `_Ugly` (panics/edge cases)
- **Licence**: EUPL-1.2
