---
title: MCP Server
description: Model Context Protocol server implementation, transports, and tool registration.
---

# MCP Server

The MCP server is the core of go-ai. It exposes 49 tools across file operations, RAG vector search, ML inference, process management, WebSocket streaming, browser automation, metrics, and IDE integration via the [Model Context Protocol](https://modelcontextprotocol.io/).

## The Service Struct

`mcp.Service` is the central container. It wraps the upstream MCP Go SDK server and owns all optional services:

```go
type Service struct {
    server         *mcp.Server       // upstream go-sdk server instance
    workspaceRoot  string            // sandboxed root for file operations
    medium         io.Medium         // filesystem abstraction (sandboxed or global)
    subsystems     []Subsystem       // plugin subsystems registered via WithSubsystem
    logger         *log.Logger       // audit logger for tool execution
    processService *process.Service  // optional: process lifecycle management
    wsHub          *ws.Hub           // optional: WebSocket hub for streaming
}
```

## Construction

`New()` uses functional options. All options are applied before tools are registered:

```go
svc, err := mcp.New(
    mcp.WithWorkspaceRoot("/path/to/project"),
    mcp.WithProcessService(ps),
    mcp.WithWSHub(hub),
    mcp.WithSubsystem(ide.New(hub, ide.WithToken(token))),
    mcp.WithSubsystem(mcp.NewMLSubsystem(mlSvc)),
)
```

**Construction sequence:**

1. Allocate `Service` with an empty `mcp.Server` (name `core-cli`, version `0.1.0`).
2. Default workspace root to `os.Getwd()` and create a sandboxed medium.
3. Apply each `Option` in order.
4. Register built-in file, directory, and language tools (10 tools).
5. Register RAG, metrics, and conditionally WebSocket and process tools.
6. Iterate subsystems and call `sub.RegisterTools(s.server)` for each plugin.

### Available Options

| Option | Effect |
|--------|--------|
| `WithWorkspaceRoot(root)` | Restrict file operations to `root`; empty string removes restriction |
| `WithProcessService(ps)` | Enable process management tools |
| `WithWSHub(hub)` | Enable WebSocket streaming tools |
| `WithSubsystem(sub)` | Append a Subsystem plugin |

## Workspace Sandboxing

The `io.Medium` abstraction (from `forge.lthn.ai/core/go-io`) isolates file access. When a workspace root is configured, every read, write, list, and stat call is validated against that root. Paths that escape the sandbox are rejected before reaching the operating system.

```go
func WithWorkspaceRoot(root string) Option {
    return func(s *Service) error {
        if root == "" {
            s.medium = io.Local   // unrestricted global filesystem
            return nil
        }
        abs, _ := filepath.Abs(root)
        m, err := io.NewSandboxed(abs)
        s.medium = m
        return nil
    }
}
```

An empty root switches the medium to `io.Local` with no path restrictions. Production deployments should always provide an explicit root.

## Transports

The server supports three transports. `Run()` auto-selects between stdio and TCP based on the `MCP_ADDR` environment variable.

### Stdio (default)

Standard integration mode for AI clients (Claude, Cursor) that spawn the server as a subprocess:

```go
func (s *Service) ServeStdio(ctx context.Context) error {
    return s.server.Run(ctx, &mcp.StdioTransport{})
}
```

`Run()` delegates to `ServeStdio` when `MCP_ADDR` is unset.

### TCP

```go
const DefaultTCPAddr = "127.0.0.1:9100"
```

Each accepted TCP connection receives its own fresh `mcp.Server` instance to prevent per-session state from leaking between clients. Messages are framed as newline-delimited JSON-RPC with a 10 MB maximum message size.

```bash
# Start in TCP mode
MCP_ADDR=127.0.0.1:9100 core mcp serve
```

A warning is emitted when binding to `0.0.0.0`; local-only access is strongly preferred.

### Unix Domain Socket

```go
func (s *Service) ServeUnix(ctx context.Context, socketPath string) error
```

The socket file is removed before binding (to recover from unclean shutdowns) and again on shutdown. Like TCP, each connection spawns an independent server instance. Logging uses the `Security` level because socket access implies filesystem-based access control.

### Transport Comparison

| Transport | Activation | Use Case |
|-----------|-----------|----------|
| Stdio | No `MCP_ADDR` set | AI client subprocess integration |
| TCP | `MCP_ADDR=host:port` | Remote clients, multi-client daemons |
| Unix | Explicit `ServeUnix()` call | Local IPC with OS-level access control |

## Subsystem Plugin Model

### Interfaces

```go
// Subsystem registers additional MCP tools at startup.
type Subsystem interface {
    Name() string
    RegisterTools(server *mcp.Server)
}

// SubsystemWithShutdown extends Subsystem with graceful cleanup.
type SubsystemWithShutdown interface {
    Subsystem
    Shutdown(ctx context.Context) error
}
```

`RegisterTools` is called once during `New()`, after built-in tools are registered. `Shutdown` is optional -- the `Service.Shutdown(ctx)` method type-asserts each subsystem and calls `Shutdown` if implemented.

### Built-in and Plugin Subsystems

| Subsystem | Type | Source |
|-----------|------|--------|
| File, directory, language tools | Built-in | `mcp/mcp.go` |
| RAG tools | Built-in | `mcp/tools_rag.go` |
| Metrics tools | Built-in | `mcp/tools_metrics.go` |
| Process tools | Built-in (conditional) | `mcp/tools_process.go` |
| WebSocket tools | Built-in (conditional) | `mcp/tools_ws.go` |
| Webview tools | Built-in | `mcp/tools_webview.go` |
| ML subsystem | Plugin (`MLSubsystem`) | `mcp/tools_ml.go` |
| IDE subsystem | Plugin (`ide.Subsystem`) | `mcp/ide/` |

## Tool Registration Pattern

Every tool follows an identical pattern: a descriptor with name and description, and a typed handler:

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "file_read",
    Description: "Read the contents of a file",
}, s.readFile)
```

The handler signature is:

```go
func(ctx context.Context, req *mcp.CallToolRequest, input InputStruct) (*mcp.CallToolResult, OutputStruct, error)
```

The MCP Go SDK deserialises JSON-RPC `params` into `InputStruct` and serialises `OutputStruct` into the response. Returning a non-nil error produces a JSON-RPC error response.

### Audit Logging

Mutating operations (`file_write`, `file_delete`, `rag_ingest`, `ws_start`) are logged at `Security` level. Read-only operations use `Info`. The current OS username is captured via `log.Username()` and attached to every log entry.

## Full Tool Inventory

49 tools across 12 groups:

| Group | Tools | Source |
|-------|-------|--------|
| File operations | `file_read`, `file_write`, `file_delete`, `file_rename`, `file_exists`, `file_edit` | `mcp/mcp.go` |
| Directory operations | `dir_list`, `dir_create` | `mcp/mcp.go` |
| Language detection | `lang_detect`, `lang_list` | `mcp/mcp.go` |
| RAG | `rag_query`, `rag_ingest`, `rag_collections` | `mcp/tools_rag.go` |
| ML inference | `ml_generate`, `ml_score`, `ml_probe`, `ml_status`, `ml_backends` | `mcp/tools_ml.go` |
| Metrics | `metrics_record`, `metrics_query` | `mcp/tools_metrics.go` |
| Process management | `process_start`, `process_stop`, `process_kill`, `process_list`, `process_output`, `process_input` | `mcp/tools_process.go` |
| WebSocket | `ws_start`, `ws_info` | `mcp/tools_ws.go` |
| Browser automation | `webview_connect`, `webview_disconnect`, `webview_navigate`, `webview_click`, `webview_type`, `webview_query`, `webview_console`, `webview_eval`, `webview_screenshot`, `webview_wait` | `mcp/tools_webview.go` |
| IDE chat | `ide_chat_send`, `ide_chat_history`, `ide_session_list`, `ide_session_create`, `ide_plan_status` | `mcp/ide/tools_chat.go` |
| IDE build | `ide_build_status`, `ide_build_list`, `ide_build_logs` | `mcp/ide/tools_build.go` |
| IDE dashboard | `ide_dashboard_overview`, `ide_dashboard_activity`, `ide_dashboard_metrics` | `mcp/ide/tools_dashboard.go` |

## Daemon Mode

The `cmd/daemon` package provides background service management:

```go
type Config struct {
    MCPTransport string // stdio, tcp, socket
    MCPAddr      string // address/path for tcp or socket
    HealthAddr   string // health check endpoint (default: 127.0.0.1:9101)
    PIDFile      string // PID file path
}
```

Configuration can be set via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `CORE_MCP_TRANSPORT` | `tcp` | Transport type |
| `CORE_MCP_ADDR` | `127.0.0.1:9100` | Listen address |
| `CORE_HEALTH_ADDR` | `127.0.0.1:9101` | Health endpoint |
| `CORE_PID_FILE` | `~/.core/daemon.pid` | PID file |

```bash
core daemon start                          # Start in background
core daemon start --mcp-transport socket   # Unix socket mode
core daemon stop                           # Graceful shutdown
core daemon status                         # Check if running
```
