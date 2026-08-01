<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/ — MCP server + transports + tool registry

**Package**: `dappco.re/go/ai/mcp`

## What this package owns

The **MCP (Model Context Protocol) server** that makes go-ai's file/dir/language tools (and any subsystem-registered tools) callable from MCP clients like Claude Code, core/ide, and remote MCP tools.

Three transports, one dispatch core, one tool registry — composes into "one MCP server, mount it where you need it".

## File map

| File | Doc | Role |
|------|-----|------|
| `service.go` | [service.md](service.md) | Server core: Options, dispatch, Subsystem interface |
| `tools_core.go` | [tools_core.md](tools_core.md) | Built-in tools (file_*, dir_*, lang_detect) |
| `tools_external.go` | [tools_external.md](tools_external.md) | External Subsystem tool registration |
| `jsonrpc.go` | [jsonrpc.md](jsonrpc.md) | Request / Response / Notification shapes |
| `transport_stdio.go` | [transport_stdio.md](transport_stdio.md) | Newline-JSON over stdio (Claude Code's default) |
| `transport_tcp.go` | [transport_tcp.md](transport_tcp.md) | Same wire, TCP transport |
| `transport_unix.go` | [transport_unix.md](transport_unix.md) | Same wire, Unix socket |

## Mental model

```
            ┌────────────────────────────────┐
            │      mcp.Service                │
            │  ┌──────────────────────────┐   │
            │  │ tool registry            │   │
            │  │  ├── built-in (tools_core)│  │
            │  │  └── external (tools_     │  │
            │  │      external + Subsystems)│ │
            │  └──────────────────────────┘   │
            │  ┌──────────────────────────┐   │
            │  │ JSON-RPC dispatch        │   │
            │  │  jsonrpc.go              │   │
            │  └──────────────────────────┘   │
            └──────┬──────┬──────────┬────────┘
                   │      │          │
                   ▼      ▼          ▼
              stdio    TCP       Unix socket
            (Claude)  (remote)   (core/ide,
                                  daemonised)
```

A binary picks one transport per Serve call. All three can be used in parallel from the same Service if needed (rare; mostly transport choice is per-deployment).

## Spec compliance

Implements the MCP server side of the protocol — tool listing, tool calling, initialise handshake. Notifications are emitted for cancellation and progress where applicable.

Reference: https://spec.modelcontextprotocol.io

## Path discipline

All file/dir tools resolve relative paths against `Options.WorkspaceRoot`. Absolute paths outside the root are rejected unless `Options.Unrestricted` is set — the load-bearing security boundary for MCP-connected agents.

## Related

- [../ai/](../ai/README.md) — the AI facade that pre-dates this (RAG, metrics, router)
- [../cmd/book-state-demo.md](../cmd/book-state-demo.md) — adjacent binary that uses a different (non-MCP) HTTP shape
- Future: core/agent and core/lab Subsystem implementations
