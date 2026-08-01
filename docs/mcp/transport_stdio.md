<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/transport_stdio.go — stdio transport

**Package**: `dappco.re/go/ai/mcp`
**File**: `go/mcp/transport_stdio.go`

## What this is

The **stdio transport** for the MCP server. Newline-delimited JSON-RPC frames over the process's stdin / stdout. Used by:

- Claude Code (the canonical MCP client) — spawns `core-cli mcp` as a subprocess and talks stdio
- Test fixtures driving the server in-process
- Any tool that prefers piped I/O to opening a socket

## API

```go
err := svc.ServeStdio(ctx)
```

Reads from `os.Stdin`, writes to `os.Stdout`, runs until ctx is cancelled or stdin returns EOF (which Claude Code does when the user exits).

## Framing

Each line is one complete JSON-RPC frame. No length prefixes; no SSE; just `\n` between frames. The server uses `bufio.Scanner` with `maxMCPMessageSize` cap.

A malformed line returns a `-32700 parse error` JSON-RPC response on stdout; the connection survives.

## Why stdio

Two reasons stdio is the canonical MCP transport:

1. **Process model.** MCP clients (Claude Code) spawn servers as child processes. Stdio is the universal IPC between parent and child — no port to allocate, no socket file to clean up, no auth.
2. **Lifecycle.** When the parent exits, stdin EOF naturally signals shutdown. No keep-alive ping needed.

## Stdout discipline

The transport writes JSON-RPC frames to stdout — therefore **nothing else may write to stdout**. Log messages, debug prints, panics — all go to stderr. A stray `fmt.Println` from a tool handler corrupts the wire frame and confuses the client.

The core framework's logging respects this: `core.Stdout()` and `core.Stderr()` are explicit; logging defaults to stderr. Tool handlers that need to surface user-facing text return it through the response, not via stdout.

## Used by

- `cmd/core-cli mcp` (when wired) — primary deployment
- Claude Code as MCP client
- test harness in `service_test.go`

## Related

- [service.md](service.md) — what dispatches received frames
- [jsonrpc.md](jsonrpc.md) — frame shape
- [transport_tcp.md](transport_tcp.md) — same shape, network transport
- [transport_unix.md](transport_unix.md) — same shape, Unix socket
