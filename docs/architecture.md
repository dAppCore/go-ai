# go-ai Architecture

**Module**: `forge.lthn.ai/core/go-ai`
**Language**: Go 1.25
**Licence**: EUPL-1.2
**LOC**: ~5.6 K total (~3.5 K non-test), 84 tests passing

---

## 1. Overview

`go-ai` is the MCP (Model Context Protocol) hub for the Lethean AI stack. It exposes 49 tools across file operations, RAG vector search, ML inference and scoring, process management, WebSocket streaming, browser automation via CDP, metrics recording, and IDE integration with the Laravel `core-agentic` backend.

### Position in the Lethean Stack

```
AI Clients (Claude, Cursor, any MCP-capable IDE)
          |  MCP JSON-RPC (stdio / TCP / Unix)
          v
  [ go-ai MCP Server ]          ← this module
      |        |        |
      |        |        └─ ide/ subsystem ─→ Laravel core-agentic (WebSocket)
      |        └─ go-rag ──────────────────→ Qdrant + Ollama
      └─ go-ml ────────────────────────────→ inference backends (go-mlx, go-rocm, …)

  Core CLI (forge.lthn.ai/core/go) bootstraps and wires everything
```

`go-ai` is a pure library module. It contains no `main` package. The Core CLI (`core mcp serve`) imports `forge.lthn.ai/core/go-ai/mcp`, constructs a `mcp.Service`, and calls `Run()`.

---

## 2. MCP Server

### 2.1 The Service Struct

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
    wsServer       *http.Server      // optional: HTTP server hosting ws hub
    wsAddr         string            // address the ws server is bound to
}
```

The `io.Medium` abstraction (from `forge.lthn.ai/core/go/pkg/io`) isolates file access. When a workspace root is configured, every read, write, list, and stat call is validated against that root. Paths that escape the workspace root are rejected by the sandboxed medium before they reach the operating system.

### 2.2 Construction: Functional Options

`New()` uses the functional options pattern. All options are applied before tools are registered, so subsystems and service dependencies are available when `registerTools` runs.

```go
svc, err := mcp.New(
    mcp.WithWorkspaceRoot("/path/to/project"),
    mcp.WithProcessService(ps),
    mcp.WithWSHub(hub),
    mcp.WithSubsystem(ide.New(hub, ide.WithToken(token))),
    mcp.WithSubsystem(mcp.NewMLSubsystem(mlSvc)),
)
```

**Construction sequence inside `New()`:**

1. Allocate `Service` with an empty `mcp.Server` (implementation name `core-cli`, version `0.1.0`).
2. Default the workspace root to `os.Getwd()` and create a sandboxed medium for it.
3. Apply each `Option` in order — later options override earlier ones.
4. Call `s.registerTools(s.server)` to install the 10 built-in file, directory, and language tools.
5. Call `registerRAGTools`, `registerMetricsTools`, and conditionally `registerWSTools` and `registerProcessTools`.
6. Iterate `s.subsystems` and call `sub.RegisterTools(s.server)` for each plugin.

**Available options:**

| Option | Effect |
|---|---|
| `WithWorkspaceRoot(root string)` | Restrict file ops to `root`; empty string removes the restriction |
| `WithProcessService(ps)` | Enable process management tools |
| `WithWSHub(hub)` | Enable WebSocket streaming tools |
| `WithSubsystem(sub)` | Append a Subsystem plugin |

### 2.3 Workspace Sandboxing

```go
func WithWorkspaceRoot(root string) Option {
    return func(s *Service) error {
        if root == "" {
            s.medium = io.Local   // unrestricted global filesystem
            return nil
        }
        abs, _ := filepath.Abs(root)
        m, err := io.NewSandboxed(abs)
        // ...
        s.medium = m
        return nil
    }
}
```

An empty root is accepted but not recommended; it switches the medium to `io.Local`, which has no path restrictions. All production deployments should provide an explicit workspace root.

---

## 3. Subsystem Plugin Model

### 3.1 Interfaces

```go
// Subsystem registers additional MCP tools at startup.
// Implementations must be safe to call concurrently.
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

`RegisterTools` is called once during `New()`, after built-in tools are registered. Subsystems receive the raw `*mcp.Server` and may register any number of tools.

`Shutdown` is optional. The `Service.Shutdown(ctx)` method iterates all subsystems, type-asserts each to `SubsystemWithShutdown`, and calls `Shutdown` if the assertion succeeds. This allows stateless subsystems (those without background goroutines or open connections) to omit the interface entirely.

### 3.2 Registration

```go
func WithSubsystem(sub Subsystem) Option {
    return func(s *Service) error {
        s.subsystems = append(s.subsystems, sub)
        return nil
    }
}
```

Subsystems are appended in declaration order. Their tools appear in the MCP tool list after the built-in tools.

### 3.3 Built-in and Plugin Subsystems

| Subsystem | Type | Source |
|---|---|---|
| File, directory, language tools | Built-in (methods on `Service`) | `mcp/mcp.go` |
| RAG tools | Built-in | `mcp/tools_rag.go` |
| Metrics tools | Built-in | `mcp/tools_metrics.go` |
| Process tools | Built-in (conditional on `WithProcessService`) | `mcp/tools_process.go` |
| WebSocket tools | Built-in (conditional on `WithWSHub`) | `mcp/tools_ws.go` |
| Webview tools | Built-in | `mcp/tools_webview.go` |
| ML subsystem | Plugin (`MLSubsystem`) | `mcp/tools_ml.go` |
| IDE subsystem | Plugin (`ide.Subsystem`) | `mcp/ide/` |

---

## 4. Tool Registration Pattern

### 4.1 Typed Handlers

Every tool follows an identical pattern: a descriptor struct with `Name` and `Description`, and a handler function with a fixed signature.

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

The MCP Go SDK deserialises the JSON-RPC `params` object into `InputStruct` before calling the handler, and serialises `OutputStruct` into the JSON-RPC response. Returning a non-nil `error` produces a JSON-RPC error response; returning a nil error with zero-value outputs is valid for no-op operations.

### 4.2 Input/Output Structs

Every tool has a dedicated pair of structs. Fields use `json:` tags for wire names and `omitempty` on optional fields.

```go
type ReadFileInput struct {
    Path string `json:"path"`
}

type ReadFileOutput struct {
    Content  string `json:"content"`
    Language string `json:"language"`
    Path     string `json:"path"`
}
```

This produces self-documenting schemas that the MCP SDK can expose to clients via the `tools/list` capability.

### 4.3 Security Logging

Sensitive tools log at the `Security` level (a custom level in `forge.lthn.ai/core/go/pkg/log`) rather than `Info`, producing a distinct audit trail. The current username is captured from the OS via `log.Username()` and attached to every log entry.

```go
// Elevated security log for write and ingest operations
s.logger.Security("MCP tool execution",
    "tool", "rag_ingest",
    "path", input.Path,
    "collection", collection,
    "user", log.Username(),
)

// Standard info log for read-only operations
s.logger.Info("MCP tool execution",
    "tool", "rag_query",
    "question", input.Question,
    "user", log.Username(),
)
```

Mutating file operations (`file_write`, `file_delete`, `file_rename`, `file_edit`, `rag_ingest`, `ws_start`) use the `Security` level. Read-only operations use `Info`.

---

## 5. Transports

The server supports three transports. `Run()` auto-selects between stdio and TCP based on the `MCP_ADDR` environment variable. Unix domain socket mode must be started explicitly.

### 5.1 Stdio (default)

```go
func (s *Service) ServeStdio(ctx context.Context) error {
    s.logger.Info("MCP Stdio server starting", "user", log.Username())
    return s.server.Run(ctx, &mcp.StdioTransport{})
}
```

`Run()` delegates to `ServeStdio` when `MCP_ADDR` is unset:

```go
func (s *Service) Run(ctx context.Context) error {
    addr := os.Getenv("MCP_ADDR")
    if addr != "" {
        return s.ServeTCP(ctx, addr)
    }
    return s.server.Run(ctx, &mcp.StdioTransport{})
}
```

Stdio is the standard integration mode for AI clients such as Claude and Cursor, which spawn the server as a subprocess and communicate over the process's stdin/stdout.

### 5.2 TCP

```go
const DefaultTCPAddr = "127.0.0.1:9100"
```

`ServeTCP` starts a net.Listener and accepts connections in a loop. Each accepted connection receives its own fresh `mcp.Server` instance and its own `connTransport`:

```go
func (s *Service) handleConnection(ctx context.Context, conn net.Conn) {
    impl := &mcp.Implementation{Name: "core-cli", Version: "0.1.0"}
    server := mcp.NewServer(impl, nil)
    s.registerTools(server)
    transport := &connTransport{conn: conn}
    if err := server.Run(ctx, transport); err != nil {
        diagPrintf("Connection error: %v\n", err)
    }
}
```

Creating a new server per connection ensures that per-session state in the MCP SDK does not leak between clients. The shared `Service` fields (`medium`, `processService`, `wsHub`) are accessed concurrently and must themselves be concurrency-safe, which the underlying packages guarantee.

The `connTransport` uses a buffered line scanner with a 10 MB maximum message size:

```go
const maxMCPMessageSize = 10 * 1024 * 1024

scanner := bufio.NewScanner(conn)
scanner.Buffer(make([]byte, 64*1024), maxMCPMessageSize)
```

Messages are framed as newline-delimited JSON-RPC. A warning is emitted to stderr when the server binds to `0.0.0.0`; local-only access (`127.0.0.1`) is strongly preferred.

**Activate TCP mode:**
```bash
MCP_ADDR=127.0.0.1:9100 core mcp serve
```

### 5.3 Unix Domain Socket

```go
func (s *Service) ServeUnix(ctx context.Context, socketPath string) error {
    _ = os.Remove(socketPath)   // clean up stale socket file
    listener, err := net.Listen("unix", socketPath)
    // ...
    defer func() {
        _ = listener.Close()
        _ = os.Remove(socketPath)   // clean up on shutdown
    }()
    s.logger.Security("MCP Unix server listening", "path", socketPath, "user", log.Username())
    // accept loop identical to TCP
}
```

The socket file is removed before binding (to recover from a previous unclean shutdown) and again on shutdown via `defer`. Like TCP, each connection spawns an independent `mcp.Server` instance via `handleConnection`. Logging uses the `Security` level because Unix socket path access implies filesystem permissions-based access control.

**Transport comparison:**

| Transport | Method | Activation | Use case |
|---|---|---|---|
| Stdio | `ServeStdio()` | No `MCP_ADDR` set | AI client subprocess integration |
| TCP | `ServeTCP()` | `MCP_ADDR=host:port` | Remote clients, multi-client daemons |
| Unix | `ServeUnix()` | Explicit call | Local IPC with OS-level access control |

---

## 6. IDE Bridge

The `mcp/ide` package implements the IDE subsystem. Its primary role is to bridge the desktop MCP server to the Laravel `core-agentic` backend running on `ws://localhost:9876/ws` (or a configured URL).

### 6.1 Subsystem Structure

```go
type Subsystem struct {
    cfg    Config
    bridge *Bridge    // nil in headless mode
    hub    *ws.Hub    // local WebSocket hub for real-time forwarding
}
```

When a `ws.Hub` is provided, the subsystem creates a `Bridge` that actively connects to Laravel. Without a hub (`hub == nil`), the subsystem runs in headless mode: tools are still registered and return stub responses, but no real-time forwarding occurs.

### 6.2 Configuration

```go
type Config struct {
    LaravelWSURL         string        // WebSocket endpoint (default: ws://localhost:9876/ws)
    WorkspaceRoot        string        // local path for workspace context
    Token                string        // Bearer token for Authorization header
    ReconnectInterval    time.Duration // base backoff (default: 2s)
    MaxReconnectInterval time.Duration // cap for exponential backoff (default: 30s)
}
```

All fields are overridable via functional options (`WithLaravelURL`, `WithToken`, `WithWorkspaceRoot`, `WithReconnectInterval`).

### 6.3 WebSocket Bridge

`Bridge` maintains a persistent WebSocket connection to Laravel and forwards inbound messages to the local `ws.Hub`.

**Connection lifecycle:**

```
StartBridge(ctx)
    └─ go connectLoop(ctx)
           ├─ dial(ctx)              ← WebSocket upgrade with Bearer token
           │      sets b.connected = true
           └─ readLoop(ctx)         ← blocks reading frames
                  └─ dispatch(msg)  ← routes to ws.Hub channel
                  [on read error]
                  sets b.connected = false, returns to connectLoop
```

**Exponential backoff:**

```go
delay := b.cfg.ReconnectInterval    // starts at 2s
for {
    if err := b.dial(ctx); err != nil {
        // wait delay, then double it up to MaxReconnectInterval
        delay = min(delay*2, b.cfg.MaxReconnectInterval)
        continue
    }
    delay = b.cfg.ReconnectInterval // reset on successful connection
    b.readLoop(ctx)
}
```

The backoff sequence for default settings is: 2 s, 4 s, 8 s, 16 s, 30 s, 30 s, … The delay resets to 2 s on every successful connection, so a brief network interruption does not permanently slow reconnection.

**Authentication:**

```go
var header http.Header
if b.cfg.Token != "" {
    header = http.Header{}
    header.Set("Authorization", "Bearer "+b.cfg.Token)
}
conn, _, err := dialer.DialContext(ctx, b.cfg.LaravelWSURL, header)
```

When `Token` is empty, no `Authorization` header is sent. This is appropriate for development environments where `core-agentic` is running without authentication.

### 6.4 Message Dispatch

Inbound frames from Laravel are deserialised into `BridgeMessage`:

```go
type BridgeMessage struct {
    Type      string    `json:"type"`
    Channel   string    `json:"channel,omitempty"`
    SessionID string    `json:"sessionId,omitempty"`
    Data      any       `json:"data,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}
```

The `dispatch` method routes the message to the local `ws.Hub`:

```go
func (b *Bridge) dispatch(msg BridgeMessage) {
    channel := msg.Channel
    if channel == "" {
        channel = "ide:" + msg.Type   // synthetic channel name
    }
    b.hub.SendToChannel(channel, ws.Message{Type: ws.TypeEvent, Data: msg.Data})
}
```

This allows browser-based UIs connected to the local WebSocket hub to receive real-time updates from Laravel without polling.

### 6.5 Outbound Messages

MCP tool handlers call `bridge.Send()` to push requests to Laravel:

```go
func (b *Bridge) Send(msg BridgeMessage) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.conn == nil {
        return fmt.Errorf("bridge: not connected")
    }
    msg.Timestamp = time.Now()
    data, _ := json.Marshal(msg)
    return b.conn.WriteMessage(websocket.TextMessage, data)
}
```

The mutex ensures that `Send` and the `readLoop` do not race on `b.conn`. If the bridge is not currently connected, `Send` returns an error, which the tool handler propagates to the MCP client as a JSON-RPC error.

### 6.6 IDE Tool Groups

The subsystem registers 11 tools across three groups:

**Chat tools** (`tools_chat.go`):

| Tool | Description |
|---|---|
| `ide_chat_send` | Send a message to an agent chat session |
| `ide_chat_history` | Retrieve message history for a session |
| `ide_session_list` | List active agent sessions |
| `ide_session_create` | Create a new agent session |
| `ide_plan_status` | Get current plan status for a session |

**Build tools** (`tools_build.go`):

| Tool | Description |
|---|---|
| `ide_build_status` | Get the status of a specific build |
| `ide_build_list` | List recent builds, optionally filtered by repository |
| `ide_build_logs` | Retrieve log output for a build |

**Dashboard tools** (`tools_dashboard.go`):

| Tool | Description |
|---|---|
| `ide_dashboard_overview` | High-level platform overview (repos, services, sessions, builds, bridge status) |
| `ide_dashboard_activity` | Recent activity feed |
| `ide_dashboard_metrics` | Aggregate build and agent metrics for a time period |

All IDE tools follow a fire-and-forward pattern: the tool sends a `BridgeMessage` to Laravel and returns an immediate acknowledgement or stub response. The real data arrives asynchronously via the WebSocket read loop and is forwarded to `ws.Hub` subscribers. The `ide_dashboard_overview` tool is the one exception — it reads `bridge.Connected()` synchronously to populate `BridgeOnline`.

---

## 7. AI Facade (`ai/` Package)

The `ai` package is the canonical entry point for AI functionality within the Lethean Go ecosystem. It composes `go-rag` and exposes a metrics layer, avoiding circular imports by keeping the package dependency-lean.

### 7.1 RAG Integration

```go
func QueryRAGForTask(task TaskInfo) string {
    query := task.Title + " " + task.Description
    // Truncate to 500 runes to keep the embedding focused
    runes := []rune(query)
    if len(runes) > 500 {
        query = string(runes[:500])
    }

    qdrantClient, err := rag.NewQdrantClient(rag.DefaultQdrantConfig())
    if err != nil {
        return ""   // graceful degradation
    }
    defer qdrantClient.Close()

    ollamaClient, err := rag.NewOllamaClient(rag.DefaultOllamaConfig())
    if err != nil {
        return ""   // graceful degradation
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    results, err := rag.Query(ctx, qdrantClient, ollamaClient, query, rag.QueryConfig{
        Collection: "hostuk-docs",
        Limit:      3,
        Threshold:  0.5,
    })
    if err != nil {
        return ""   // graceful degradation
    }
    return rag.FormatResultsContext(results)
}
```

Every failure path returns an empty string rather than propagating an error. This is intentional: callers (typically agentic task planners) should continue operating without RAG context when Qdrant or Ollama are unavailable. The absence of context is preferable to a hard failure.

The query is capped at 500 runes before embedding to keep the vector focused on the task's core intent rather than noise from long descriptions.

### 7.2 JSONL Metrics Storage

Events are recorded to daily JSONL files at:

```
~/.core/ai/metrics/YYYY-MM-DD.jsonl
```

Each line is a JSON-encoded `Event`:

```go
type Event struct {
    Type      string         `json:"type"`
    Timestamp time.Time      `json:"timestamp"`
    AgentID   string         `json:"agent_id,omitempty"`
    Repo      string         `json:"repo,omitempty"`
    Duration  time.Duration  `json:"duration,omitempty"`
    Data      map[string]any `json:"data,omitempty"`
}
```

**Writing:**

```go
func Record(event Event) error {
    dir, _ := metricsDir()
    os.MkdirAll(dir, 0o755)
    path := metricsFilePath(dir, event.Timestamp)
    f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    defer f.Close()
    data, _ := json.Marshal(event)
    f.Write(append(data, '\n'))
}
```

`OpenFile` with `O_APPEND` is atomically safe for single-process concurrent writers on POSIX systems. Multiple goroutines may call `Record` without external synchronisation.

**Reading:**

```go
func ReadEvents(since time.Time) ([]Event, error)
```

`ReadEvents` iterates calendar days from `since` to today, opens each daily file, and filters events by timestamp. Missing files are silently skipped (the metric directory may be sparse for gaps in activity).

Malformed JSONL lines are skipped without returning an error, providing forward compatibility when the `Event` struct gains new fields.

**Aggregation:**

```go
func Summary(events []Event) map[string]any {
    // returns: by_type, by_repo, by_agent, recent
    // each count map is keyed by the group value
}
```

`Summary` is a pure function with no I/O. It is used by the `metrics_query` MCP tool to build its response.

### 7.3 MCP Metrics Tools

The `metrics_record` and `metrics_query` tools expose the JSONL layer directly to MCP clients:

```
metrics_record → ai.Record(Event{...})
metrics_query  → ai.ReadEvents(since) → ai.Summary(events)
```

The `since` parameter accepts human-readable shorthand: `30m`, `24h`, `7d`. The parser maps these to `time.Duration` values before calling `ReadEvents`.

---

## 8. Full Tool Inventory

49 tools across 12 groups:

| Group | Tools | Source file |
|---|---|---|
| File operations | `file_read`, `file_write`, `file_delete`, `file_rename`, `file_exists`, `file_edit` | `mcp/mcp.go` |
| Directory operations | `dir_list`, `dir_create` | `mcp/mcp.go` |
| Language detection | `lang_detect`, `lang_list` | `mcp/mcp.go` |
| RAG | `rag_query`, `rag_ingest`, `rag_collections` | `mcp/tools_rag.go` |
| ML inference | `ml_generate`, `ml_score`, `ml_probe`, `ml_status`, `ml_backends` | `mcp/tools_ml.go` |
| Metrics | `metrics_record`, `metrics_query` | `mcp/tools_metrics.go` |
| Process management | `process_start`, `process_stop`, `process_kill`, `process_list`, `process_output`, `process_input` | `mcp/tools_process.go` |
| WebSocket | `ws_start`, `ws_info` | `mcp/tools_ws.go` |
| Webview (CDP) | `webview_connect`, `webview_disconnect`, `webview_navigate`, `webview_click`, `webview_type`, `webview_query`, `webview_console`, `webview_eval`, `webview_screenshot`, `webview_wait` | `mcp/tools_webview.go` |
| IDE chat | `ide_chat_send`, `ide_chat_history`, `ide_session_list`, `ide_session_create`, `ide_plan_status` | `mcp/ide/tools_chat.go` |
| IDE build | `ide_build_status`, `ide_build_list`, `ide_build_logs` | `mcp/ide/tools_build.go` |
| IDE dashboard | `ide_dashboard_overview`, `ide_dashboard_activity`, `ide_dashboard_metrics` | `mcp/ide/tools_dashboard.go` |

---

## 9. Package Layout

```
go-ai/
├── ai/                          # AI facade: RAG queries and JSONL metrics
│   ├── ai.go                    # Package documentation and composition overview
│   ├── rag.go                   # QueryRAGForTask() with graceful degradation
│   └── metrics.go               # Event, Record(), ReadEvents(), Summary()
│
└── mcp/                         # MCP server, built-in tools, and transports
    ├── mcp.go                   # Service struct, New(), functional options,
    │                            # registerTools() for file/dir/lang (10 tools)
    ├── subsystem.go             # Subsystem and SubsystemWithShutdown interfaces,
    │                            # WithSubsystem() option
    ├── tools_rag.go             # rag_query, rag_ingest, rag_collections
    ├── tools_ml.go              # MLSubsystem: ml_generate, ml_score, ml_probe,
    │                            # ml_status, ml_backends
    ├── tools_metrics.go         # metrics_record, metrics_query; parseDuration()
    ├── tools_process.go         # process_start/stop/kill/list/output/input
    ├── tools_ws.go              # ws_start, ws_info; ProcessEventCallback
    ├── tools_webview.go         # webview_connect/disconnect/navigate/click/type/
    │                            # query/console/eval/screenshot/wait
    ├── transport_stdio.go       # ServeStdio() — default subprocess transport
    ├── transport_tcp.go         # ServeTCP(), connTransport, connConnection;
    │                            # per-connection server instances; security warnings
    ├── transport_unix.go        # ServeUnix() — domain socket with stale-file cleanup
    └── ide/                     # IDE subsystem (plugin implementing Subsystem)
        ├── ide.go               # Subsystem struct, New(), RegisterTools(),
        │                        # Shutdown(), StartBridge()
        ├── config.go            # Config, DefaultConfig(), functional option helpers
        ├── bridge.go            # Bridge: WebSocket connection to Laravel,
        │                        # connectLoop(), exponential backoff, Send(), dispatch()
        ├── tools_chat.go        # ide_chat_send/history, ide_session_list/create,
        │                        # ide_plan_status; BridgeMessage wire types
        ├── tools_build.go       # ide_build_status/list/logs; BuildInfo wire types
        └── tools_dashboard.go   # ide_dashboard_overview/activity/metrics;
                                 # DashboardOverview, ActivityEvent wire types
```

---

## 10. Dependencies

### Direct

| Module | Role |
|---|---|
| `forge.lthn.ai/core/go` | Core framework: `pkg/io` (sandboxed filesystem), `pkg/log` (security-level logging), `pkg/process` (process lifecycle), `pkg/ws` (WebSocket hub), `pkg/webview` (CDP client) |
| `forge.lthn.ai/core/go-ml` | ML scoring engine: heuristic scores, LLM judge backend, capability probes, InfluxDB pipeline status |
| `forge.lthn.ai/core/go-rag` | RAG layer: Qdrant vector DB client, Ollama embeddings, markdown chunking, `FormatResultsContext` |
| `forge.lthn.ai/core/go-inference` | Shared `TextModel`/`Backend` interfaces; `inference.List()`, `inference.Get()`, `inference.Default()` registry |
| `github.com/modelcontextprotocol/go-sdk` | MCP Go SDK: `Server`, `StdioTransport`, `AddTool`, JSON-RPC framing |
| `github.com/gorilla/websocket` | WebSocket client used by the IDE bridge |
| `github.com/stretchr/testify` | Test assertions |

### Indirect (via `go-ml` and `go-rag`)

`go-mlx`, `go-rocm`, `go-duckdb`, `parquet-go`, `ollama`, `qdrant/go-client`, and the Arrow ecosystem are transitive dependencies. They are not imported directly by `go-ai`.

All `forge.lthn.ai/core/*` dependencies use `replace` directives in `go.mod` pointing to local sibling directories during development:

```
replace forge.lthn.ai/core/go       => ../go
replace forge.lthn.ai/core/go-mlx   => ../go-mlx
replace forge.lthn.ai/core/go-ml    => ../go-ml
replace forge.lthn.ai/core/go-rag   => ../go-rag
replace forge.lthn.ai/core/go-inference => ../go-inference
```

---

## 11. Integration Points

| System | Protocol | Direction | Notes |
|---|---|---|---|
| Core CLI (`forge.lthn.ai/core/go`) | Go import | Inbound — CLI bootstraps this module | No `main` package in `go-ai`; always embedded |
| AI clients (Claude, Cursor, etc.) | MCP JSON-RPC | Inbound | Stdio (subprocess), TCP, or Unix socket |
| Laravel `core-agentic` | WebSocket | Outbound — IDE bridge | `ws://localhost:9876/ws` by default; Bearer token auth |
| Qdrant | gRPC | Outbound — RAG tools | `localhost:6334` default; via `go-rag` |
| Ollama | HTTP | Outbound — RAG embeddings | `localhost:11434` default; via `go-rag` |
| Chrome / CDP | HTTP (DevTools Protocol) | Outbound — webview tools | Via `pkg/webview` in `core/go` |
| InfluxDB | HTTP | Outbound — `ml_status` tool | `localhost:8086` default; via `go-ml` |
| Filesystem | OS | Bidirectional — file/dir tools | Sandboxed to workspace root |
| `~/.core/ai/metrics/` | JSONL files | Write (record) / Read (query) | Daily rotation, append-only |
