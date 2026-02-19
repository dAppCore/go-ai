# FINDINGS — go-ai

## Split History

Extracted from the go-ai monolith on 19 Feb 2026. The original `forge.lthn.ai/core/go-ai` contained all AI/ML packages in a single module. The split extracted:

- `go-ml` (scoring engine, heuristics, judge, probes, backends) -> `forge.lthn.ai/core/go-ml`
- `go-rag` (Qdrant client, Ollama embeddings, markdown chunking) -> `forge.lthn.ai/core/go-rag`
- `go-agentic` (task queue, context builder, allowances) -> `forge.lthn.ai/core/go-agentic`
- `go-mlx` (native Metal GPU inference) -> `forge.lthn.ai/core/go-mlx`

What remains in go-ai is the MCP server hub, the `ai/` facade, and the IDE subsystem.

Commit history showing the extraction:
```
0af152e refactor: extract ml/ to standalone core/go-ml module
2886ffa refactor: extract rag/ to standalone core/go-rag module
f99ca10 refactor: extract agentic/ to standalone core/go-agentic module
34d0f9c refactor: extract mlx/ to standalone core/go-mlx module
906a535 chore: update module paths and add gitignore
```

## Current State

- **84 tests passing** (pre-split count; post-split the mcp/, mcp/ide/, and ai/ tests remain here)
- Clean split: `go build ./...` and `go test ./...` pass with local replace directives
- No circular dependencies between the extracted modules

## Architecture Notes

### Subsystem Plugin Model

The MCP server uses a plugin architecture via the `Subsystem` interface. This allows tool groups to be composed at startup without the server knowing about their internals:

1. **Built-in tools** (10): File, directory, and language detection — always registered
2. **Conditional tools**: Process (6) and WebSocket (2) tools register only if their services are provided
3. **Subsystem tools**: ML (5), IDE Chat (5), IDE Build (3), IDE Dashboard (3) register via `WithSubsystem()`
4. **Direct registration**: RAG (3), Metrics (2), Webview (10) are registered by the Service itself

The IDE subsystem implements `SubsystemWithShutdown` to gracefully close the Laravel WebSocket bridge on server shutdown.

### Tool Registration Pattern

Every tool follows the same pattern:
- Typed `XxxInput` struct with JSON tags for MCP parameter binding
- Typed `XxxOutput` struct for the response
- Handler function with signature `func(ctx, *CallToolRequest, Input) (*CallToolResult, Output, error)`
- Security-sensitive tools log via `logger.Security()`, others via `logger.Info()`

### IDE Bridge

The `mcp/ide/` package maintains a persistent WebSocket connection to Laravel's core-agentic backend. Messages are forwarded bidirectionally:

- **Outbound** (Go -> Laravel): Tool handlers call `bridge.Send()` with typed `BridgeMessage`
- **Inbound** (Laravel -> Go): `readLoop()` receives messages and dispatches them to `ws.Hub` channels

The bridge uses exponential backoff for reconnection (2s base, 30s max).

### Metrics Storage

The `ai/` package stores metrics as daily JSONL files at `~/.core/ai/metrics/YYYY-MM-DD.jsonl`. Each line is a JSON-encoded `Event` with type, timestamp, agent ID, repo, and arbitrary data. The `Summary()` function aggregates by type, repo, and agent.

## Dependencies

- **Framework**: `forge.lthn.ai/core/go` provides `pkg/io` (sandboxed filesystem), `pkg/log` (structured logging), `pkg/process` (process management), `pkg/ws` (WebSocket hub), `pkg/webview` (CDP client)
- **Module path**: `forge.lthn.ai/core/go-ai`
- **Replace directives** point to `../core`, `../go-mlx`, `../go-ml`, `../go-rag` for local development

## Known Issues

1. **tools_ml.go still uses old go-ml Backend interface**: The `MLSubsystem` directly imports `forge.lthn.ai/core/go-ml` and uses `ml.Service`, `ml.GenOpts`, `ml.HeuristicScores`, etc. This should be migrated to a `go-inference` abstraction layer to decouple from specific backends.

2. **IDE bridge only tested locally**: The bridge unit tests mock the WebSocket connection but have not been tested against a real Laravel core-agentic instance. Reconnection behaviour under network partition is unverified.

3. **Dashboard and build tools are stubs**: The IDE dashboard and build tools forward requests to Laravel via the bridge but return empty placeholder data. Real data arrives asynchronously via WebSocket subscription, which the current tool response model does not surface.

4. **Webview tools use package-level state**: `webviewInstance` is a package-level `var`, meaning only one Chrome connection is supported at a time. This is acceptable for single-agent use but would need refactoring for concurrent sessions.

5. **RAG tools require external services**: `rag_query`, `rag_ingest`, and `rag_collections` require Qdrant and Ollama to be running. No mock or fallback exists for CI environments.

6. **Unix transport has no test coverage**: `transport_unix.go` implements Unix domain socket serving but has no corresponding test file.
