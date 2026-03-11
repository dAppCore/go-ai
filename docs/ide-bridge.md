---
title: IDE Bridge
description: IDE integration bridge connecting the MCP server to Laravel core-agentic via WebSocket.
---

# IDE Bridge

The `mcp/ide` package implements the IDE subsystem. It bridges the desktop MCP server to the Laravel `core-agentic` backend via a persistent WebSocket connection, enabling AI agents to interact with agent sessions, builds, and the platform dashboard.

## Architecture

```
MCP Client (Claude, Cursor, etc.)
    |
    v  MCP JSON-RPC
go-ai MCP Server
    |
    v  ide_* tool calls
IDE Subsystem (mcp/ide/)
    |
    +-- Bridge -----------> Laravel core-agentic
    |   (WebSocket)         ws://localhost:9876/ws
    |
    +-- ws.Hub <------------ Bridge dispatch
         |
         v  real-time updates
    Local WebSocket subscribers (browser UIs)
```

## Subsystem Structure

```go
type Subsystem struct {
    cfg    Config
    bridge *Bridge    // nil in headless mode
    hub    *ws.Hub    // local WebSocket hub for real-time forwarding
}
```

When a `ws.Hub` is provided, the subsystem creates a `Bridge` that actively connects to Laravel. Without a hub (`hub == nil`), the subsystem runs in **headless mode**: tools are still registered and return stub responses, but no real-time forwarding occurs.

## Configuration

```go
type Config struct {
    LaravelWSURL         string        // WebSocket endpoint (default: ws://localhost:9876/ws)
    WorkspaceRoot        string        // local path for workspace context
    Token                string        // Bearer token for Authorization header
    ReconnectInterval    time.Duration // base backoff (default: 2s)
    MaxReconnectInterval time.Duration // cap for exponential backoff (default: 30s)
}
```

All fields are overridable via functional options:

```go
sub := ide.New(hub,
    ide.WithLaravelURL("ws://custom:9876/ws"),
    ide.WithToken("my-bearer-token"),
    ide.WithWorkspaceRoot("/path/to/project"),
    ide.WithReconnectInterval(5 * time.Second),
)
```

## WebSocket Bridge

The `Bridge` maintains a persistent WebSocket connection to Laravel and forwards inbound messages to the local `ws.Hub`.

### Connection Lifecycle

```
StartBridge(ctx)
    +-- go connectLoop(ctx)
           +-- dial(ctx)              <-- WebSocket upgrade with Bearer token
           |      sets b.connected = true
           +-- readLoop(ctx)          <-- blocks reading frames
                  +-- dispatch(msg)   <-- routes to ws.Hub channel
                  [on read error]
                  sets b.connected = false, returns to connectLoop
```

### Exponential Backoff

When the connection drops or fails to establish, the bridge uses exponential backoff:

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

Backoff sequence with defaults: 2s, 4s, 8s, 16s, 30s, 30s, ... The delay resets to 2s on every successful connection.

### Authentication

```go
var header http.Header
if b.cfg.Token != "" {
    header = http.Header{}
    header.Set("Authorization", "Bearer "+b.cfg.Token)
}
conn, _, err := dialer.DialContext(ctx, b.cfg.LaravelWSURL, header)
```

When `Token` is empty, no `Authorization` header is sent. This is appropriate for development environments running without authentication.

### Message Dispatch

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

The `dispatch` method routes messages to the local `ws.Hub`:

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

### Outbound Messages

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

A mutex ensures `Send` and the `readLoop` do not race on `b.conn`. If the bridge is disconnected, `Send` returns an error which propagates to the MCP client as a JSON-RPC error.

## IDE Tool Groups

The subsystem registers 11 tools across three groups.

### Chat Tools (`tools_chat.go`)

| Tool | Description |
|------|-------------|
| `ide_chat_send` | Send a message to an agent chat session |
| `ide_chat_history` | Retrieve message history for a session |
| `ide_session_list` | List active agent sessions |
| `ide_session_create` | Create a new agent session |
| `ide_plan_status` | Get current plan status for a session |

### Build Tools (`tools_build.go`)

| Tool | Description |
|------|-------------|
| `ide_build_status` | Get the status of a specific build |
| `ide_build_list` | List recent builds, optionally filtered by repository |
| `ide_build_logs` | Retrieve log output for a build |

### Dashboard Tools (`tools_dashboard.go`)

| Tool | Description |
|------|-------------|
| `ide_dashboard_overview` | High-level platform overview (repos, services, sessions, builds, bridge status) |
| `ide_dashboard_activity` | Recent activity feed |
| `ide_dashboard_metrics` | Aggregate build and agent metrics for a time period |

### Tool Behaviour

All IDE tools follow a **fire-and-forward** pattern: the tool sends a `BridgeMessage` to Laravel and returns an immediate acknowledgement or stub response. Real data arrives asynchronously via the WebSocket read loop and is forwarded to `ws.Hub` subscribers.

The `ide_dashboard_overview` tool is the one exception -- it reads `bridge.Connected()` synchronously to populate the `BridgeOnline` field.

## Registration

The IDE subsystem is registered during MCP server construction:

```go
svc, err := mcp.New(
    mcp.WithWSHub(hub),
    mcp.WithSubsystem(ide.New(hub, ide.WithToken(token))),
)
```

The subsystem implements `SubsystemWithShutdown`, closing the bridge connection gracefully when the MCP server shuts down.

## Testing

Bridge tests use `net/http/httptest` to stand up a real WebSocket server in-process. This keeps tests hermetic while exercising:

- Reconnection logic and exponential backoff
- Authentication header injection
- Message dispatch routing
- Server shutdown detection

The `waitConnected` helper polls `bridge.Connected()` with a deadline rather than using fixed sleeps.

All 11 tool handlers are tested under two conditions:
- **nil bridge** -- verifies the error path
- **connected mock bridge** -- verifies the success path with JSON round-trip validation
