<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/transport_tcp.go — TCP transport

**Package**: `dappco.re/go/ai/mcp`
**File**: `go/mcp/transport_tcp.go`

## What this is

The **TCP transport** for the MCP server. Same JSON-RPC envelope, newline-delimited framing, but over a TCP socket. Used when:

- The MCP server is a long-running daemon and clients dial in
- Multiple clients want concurrent connections to one server
- The server runs on a different machine from the client

## API

```go
err := svc.ServeTCP(ctx, ":7777")
err := svc.ServeTCP(ctx, "127.0.0.1:7777")  // localhost-only (production default)
```

Returns when ctx is cancelled. Accepts multiple concurrent connections; each gets its own goroutine handling the JSON-RPC loop.

## Concurrency model

```
ServeTCP listener goroutine
  ├── accept conn 1 → conn-handler goroutine 1
  ├── accept conn 2 → conn-handler goroutine 2
  └── …
```

Each conn-handler runs the same frame-decode + dispatch loop as stdio. The dispatch handlers themselves are responsible for any cross-conn state safety (the file-tool handlers, for instance, use `core.Mutex` for write protection).

## Security

TCP is **less safe by default** than Unix sockets — no filesystem permission gate, the network is in scope, malicious clients can dial in if the bind is wrong.

The default bind in deployments is `127.0.0.1:7777` — localhost-only. Binding to `0.0.0.0` requires explicit `--bind` configuration and a warning surface.

Production deployments add a proxy in front (Traefik / nginx) for TLS + auth when the MCP server needs to be remote-accessible. The transport itself stays plain TCP — wrap it externally.

## Why both TCP and Unix

| Use case | Choose |
|----------|--------|
| Same-machine, single user | stdio or Unix |
| Same-machine, multi-process | Unix (filesystem permissions) |
| Network remote | TCP + TLS proxy |
| Container-to-container | TCP (sockets across containers are awkward) |

## Used by

- `core/ide` (planned) — TCP backend mode for cross-machine dev
- Cladius's homelab integration — TCP over Tailscale for cross-machine MCP

## Related

- [service.md](service.md) — server core
- [transport_stdio.md](transport_stdio.md) — stdio sibling
- [transport_unix.md](transport_unix.md) — Unix sibling
- [jsonrpc.md](jsonrpc.md) — wire envelope
