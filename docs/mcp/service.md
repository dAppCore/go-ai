<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/service.go — MCP server core

**Package**: `dappco.re/go/ai/mcp`
**File**: `go/mcp/service.go`

## What this is

The MCP (Model Context Protocol) **server** that go-ai mounts to give agents access to file ops, directory ops, language detection, and registered subsystems. Wraps JSON-RPC over multiple transports (stdio, TCP, Unix socket).

This is the file that turns "go-ai is a library" into "go-ai is also an MCP server other tools can dial".

## Constants

```go
serverName        = "core-cli"
serverVersion     = "0.1.0"
maxMCPMessageSize = 10 * 1024 * 1024   // 10MB per JSON-RPC frame
```

The 10MB cap protects against malformed frames trying to OOM the server.

## Options

```go
type Options struct {
    WorkspaceRoot  string         // root for path-relative file ops
    Unrestricted   bool           // disable path/cmd restrictions (dev mode)
    ProcessService any            // optional core process service for proc tools
    WSHub          any            // optional websocket hub for live tools
    Subsystems     []Subsystem    // extra tool sets to register
}
```

`Unrestricted: true` lifts the path-allowlist gate. Dangerous; off by default; opt-in for trusted local-only invocations.

## Subsystem

```go
type Subsystem interface {
    RegisterTools(s *Service) core.Result
}
```

The extension point. Each subsystem registers its own tools at startup. Examples (planned / shipped):

- `core/agent` subsystem — agent introspection
- `core/lab` subsystem — lab dashboard tools
- `core/ide` subsystem — IDE-side tools
- third-party subsystems — auto-discovered or explicit

## Lifecycle

```go
svc, _ := mcp.New(mcp.Options{
    WorkspaceRoot: "/Users/me/project",
    Subsystems:    []Subsystem{labSubsystem, agentSubsystem},
}).Value.(*mcp.Service)

svc.ServeStdio(ctx)
// or
svc.ServeTCP(ctx, ":7777")
// or
svc.ServeUnix(ctx, "/tmp/core-mcp.sock")
```

Each Serve* runs until ctx is cancelled. Returns `core.Result` for clean error propagation.

## Built-in tools

Registered by `registerBuiltInTools` in `tools_core.go`:

| Tool name | Group | Purpose |
|-----------|-------|---------|
| `file_read` | file | read file content |
| `file_write` | file | write file content |
| `file_delete` | file | delete file or empty dir |
| `file_rename` | file | rename / move |
| `file_exists` | file | existence check |
| `file_edit` | file | replace text in file |
| `dir_list` | dir | list contents |
| `dir_create` | dir | mkdir |
| `lang_detect` | language | identify language from path |

Plus tools registered by Subsystems at startup.

## Path discipline

All file/dir tools resolve relative paths against `WorkspaceRoot`. Absolute paths outside the root are rejected unless `Unrestricted` is set. This is the **load-bearing security boundary** — without it, an MCP-connected agent could `file_write /etc/passwd`.

## Error handling

Tool failures return JSON-RPC error objects with stable error codes. The server doesn't crash on bad input — only on misconfiguration (no workspace root + no Unrestricted).

## Used by

- Claude Code (via stdio MCP transport)
- core/ide (via Unix socket)
- Cladius's own session (via TCP when configured)
- Future: third-party MCP clients

## Related

- [tools_core.md](tools_core.md) — built-in tool implementations
- [tools_external.md](tools_external.md) — external tool registration
- [jsonrpc.md](jsonrpc.md) — JSON-RPC envelope
- [transport_stdio.md](transport_stdio.md) — stdio transport
- [transport_tcp.md](transport_tcp.md) — TCP transport
- [transport_unix.md](transport_unix.md) — Unix socket transport
- MCP spec: https://spec.modelcontextprotocol.io
