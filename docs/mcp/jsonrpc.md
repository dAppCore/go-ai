<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/jsonrpc.go — JSON-RPC envelope

**Package**: `dappco.re/go/ai/mcp`
**File**: `go/mcp/jsonrpc.go`

## What this is

The JSON-RPC 2.0 envelope shapes the MCP server uses across all transports — newline-delimited frames carrying `tool/call`, `tool/list`, `initialize`, and the rest of the MCP protocol surface.

## Request

```go
type jsonrpcRequest struct {
    JSONRPC string          `json:"jsonrpc"`   // always "2.0"
    ID      json.RawMessage `json:"id,omitempty"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}
```

`ID` is `RawMessage` because JSON-RPC allows ids as either string or number; we don't care about the type, just need to echo it.

## Response

```go
type jsonrpcResponse struct {
    JSONRPC string            `json:"jsonrpc"`
    ID      json.RawMessage   `json:"id,omitempty"`
    Result  any               `json:"result,omitempty"`
    Error   *jsonrpcError     `json:"error,omitempty"`
}

type jsonrpcError struct {
    Code    int     `json:"code"`
    Message string  `json:"message"`
    Data    any     `json:"data,omitempty"`
}
```

Standard JSON-RPC 2.0 shape. Exactly one of `Result` / `Error` is set per response.

## Error codes

MCP uses both the standard JSON-RPC codes (-32700 parse error, -32600 invalid request, -32601 method not found, etc.) and MCP-specific codes for tool-level errors.

## Notification

```go
type jsonrpcNotification struct {
    JSONRPC string          `json:"jsonrpc"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}
```

JSON-RPC notifications have no ID — fire-and-forget. MCP uses these for server-pushed updates (e.g., `notifications/cancelled`).

## Why a thin file

This file deliberately doesn't try to be a JSON-RPC library. The shapes are tiny; the codec is `encoding/json` (via `core.JSONUnmarshalString`). All the complexity lives in the transport layer (framing) and the service layer (method dispatch); this file just defines the wire shape.

## Related

- [service.md](service.md) — dispatch logic that consumes these shapes
- [transport_stdio.md](transport_stdio.md) — newline-delimited framing
- [transport_tcp.md](transport_tcp.md) — same wire, different transport
- JSON-RPC 2.0 spec: https://www.jsonrpc.org/specification
