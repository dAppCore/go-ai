<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/transport_unix.go — Unix socket transport

**Package**: `dappco.re/go/ai/mcp`
**File**: `go/mcp/transport_unix.go`

## What this is

The **Unix domain socket transport** for the MCP server. Same JSON-RPC + newline-delimited framing as stdio and TCP, but over a Unix socket. The production default for same-machine, multi-process MCP — fast, no network exposure, filesystem-permissions for access control.

## API

```go
const DefaultUnixSocket = "/tmp/core-mcp.sock"

err := svc.ServeUnix(ctx, "")                          // uses default
err := svc.ServeUnix(ctx, "/var/run/core/mcp.sock")    // explicit
```

Empty path uses the default. Returns when ctx is cancelled. The socket file is created on Serve, removed on shutdown (best-effort).

## Concurrency model

Same as TCP — listener goroutine accepts, each conn gets its own handler goroutine. Multiple processes can hold concurrent connections.

## Security

Unix sockets gate via **filesystem permissions**:

```
chmod 0600 /tmp/core-mcp.sock    # owner only
chmod 0660 + group              # owner + group
```

The default permission is 0600 (owner only). Multi-user setups can override via the socket's directory chmod and the user's umask.

No network exposure — even with the socket mode wide-open, only local processes can connect.

## Cleanup

Stale sockets from a crashed previous run would block `bind()`. The server's start path:

1. Tries to dial the socket — if dial succeeds, another server is running → error
2. If dial fails (no listener), remove the stale socket file
3. bind() fresh

## Why Unix is the production default

Three reasons over TCP:

1. **No port allocation.** Filesystem path > TCP port for static configuration.
2. **No network risk.** Wrong bind → not exposed beyond the box.
3. **Cleaner credentials.** Filesystem ownership is the auth model.

Three reasons over stdio:

1. **Long-running daemon.** Stdio is per-process; Unix is daemonised.
2. **Multi-client.** Stdio is 1:1; Unix accepts many.
3. **Reconnect.** A crashed client can re-dial; stdio doesn't recover.

## Used by

- `core/ide` (production) — primary IDE-to-Cladius MCP transport
- `cmd/violet` (the go-mlx sidecar) — uses Unix socket for its own protocol (not MCP, but same shape)
- Daemonised `core-cli mcp` (planned) — long-running MCP server

## Related

- [service.md](service.md) — server core
- [transport_stdio.md](transport_stdio.md) — stdio sibling
- [transport_tcp.md](transport_tcp.md) — TCP sibling
- [jsonrpc.md](jsonrpc.md) — wire envelope
- `../../../go-mlx/docs/cmd/violet.md` — adjacent Unix-socket sidecar
