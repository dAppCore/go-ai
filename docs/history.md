# go-ai — Project History and Known Issues

Module: `forge.lthn.ai/core/go-ai`

---

## Project History

### Origins

`go-ai` began as a monolith of approximately 23,000 lines of Go, housing all AI and machine-learning concerns for the Lethean platform in a single module. The module covered ML scoring and heuristics, retrieval-augmented generation, native Metal GPU inference, agentic task queuing, an MCP server hub, an `ai/` facade for metrics and shared utilities, and an IDE subsystem bridging the Go MCP layer to a Laravel backend.

The monolithic structure was pragmatic during early development but created coupling that made independent versioning, focused testing, and cross-team ownership impractical.

### The Split — 19 February 2026

On 19 February 2026 the monolith was broken into focused, independently versioned modules. The extraction commits, in order, were:

```
0af152e  refactor: extract ml/ to standalone core/go-ml module
2886ffa  refactor: extract rag/ to standalone core/go-rag module
f99ca10  refactor: extract agentic/ to standalone core/go-agentic module
34d0f9c  refactor: extract mlx/ to standalone core/go-mlx module
906a535  chore: update module paths and add gitignore
```

The four modules extracted were:

| New module | Contents | Path |
|---|---|---|
| `go-ml` | Scoring engine, heuristics, judge, probes, backends | `forge.lthn.ai/core/go-ml` |
| `go-rag` | Qdrant client, Ollama embeddings, markdown chunking | `forge.lthn.ai/core/go-rag` |
| `go-agentic` | Task queue, context builder, allowances | `forge.lthn.ai/core/go-agentic` |
| `go-mlx` | Native Metal GPU inference | `forge.lthn.ai/core/go-mlx` |

What remains in `go-ai` after the split:

- **MCP server hub** (~5,600 LOC) — 30+ tool handlers across file I/O, process management, RAG, ML, metrics, webview, and IDE subsystems
- **`ai/` facade** — metrics recording, JSONL storage, agent summary aggregation
- **IDE subsystem** (`mcp/ide/`) — persistent WebSocket bridge to the Laravel `core-agentic` backend, with dashboard, chat, and build tool handlers

The split produced no circular dependencies. `go build ./...` and `go test ./...` both passed against local `replace` directives immediately after extraction.

---

## Development Phases

### Phase 1 — Post-Split Cleanup

Immediately following the extraction, the module needed housekeeping to verify the remaining code was self-consistent.

- Deleted the standalone `test-mlx.go` script from the module root.
- Confirmed `go build ./...` produced a clean build with no stale import paths.
- Confirmed `go vet ./...` produced no warnings.
- Ran the full test suite to baseline. Two tests required fixes: `TestSandboxing_Symlinks_Blocked` (renamed, assertion corrected) and `TestNewTCPTransport_Warning` (security warning added to `NewTCPTransport`).

### Phase 2 — go-inference Migration

Commit `4d73fa2`

`go-ml` adopted `go-inference` shared interfaces (`forge.lthn.ai/core/go-inference`) to allow backends to be swapped without altering call sites. `go-ai`'s ML tool layer was updated accordingly:

- `tools_ml.go` — `MLSubsystem` flow documented; generation already routed through `InferenceAdapter -> inference.TextModel` without behavioural change.
- `ml_backends` tool — rewritten to call `inference.List()`, `inference.Get()`, and `inference.Default()` instead of the lower-level `ml.Service` methods.
- `ml_score` and `ml_probe` — retained direct `go-ml` dependency (the scoring engine lives there); generation paths route through `go-inference`.
- `go.mod` — `go-inference` promoted from indirect to direct dependency.

### Phase 3 — MCP Transport Testing

Commit `a6a7fb8`

The MCP server supports three transports: stdio, TCP, and Unix domain sockets. This phase added end-to-end tests covering the wire protocol for each.

- **Stdio** — `StdioTransport` binds `os.Stdin`/`os.Stdout` directly; a CI test was documented as skipped with rationale. Protocol correctness is covered by TCP and Unix tests.
- **TCP** — Full JSON-RPC round-trip: `initialize` → `tools/list` → `tools/call file_read` → `tools/call file_write`. Additional tests cover tool discovery and error handling.
- **Unix** — Full end-to-end via Unix domain socket: `initialize` → `tools/list` → `file_read` → `dir_list`. Socket paths are kept short to respect macOS `sun_path` limits.
- **Webview CI guard** — `skipIfShort()` guard added; `TestWebviewToolHandlers_RequiresChrome` is marked to skip under `-short`. Struct-level webview tests remain CI-safe.

### Phase 4 — IDE Subsystem Hardening

Commit `8c0ef43`

The IDE subsystem (`mcp/ide/`) maintains a persistent WebSocket connection to Laravel's `core-agentic` backend. Before this phase it had no dedicated tests. This phase added:

- **Bridge reconnection** — Fixed a data race (converted shared counter to `atomic.Int32`). Added tests for exponential backoff (HTTP 403 path) and server shutdown detection.
- **Auth support** — `Token` field added to `Config`, `WithToken` option exposed, Bearer header injected in `dial()`. Tests verify header presence and absence.
- **Dashboard, chat, and build tool tests** — 49 tests in total, covering all 11 tool handlers under two conditions: nil bridge (error path) and a connected mock bridge (success path). JSON round-trips verified for all input and output types. Stub documentation comments added to each handler.

### Phase 5 — Testing Gaps

Commit `2c745a6`

A survey of coverage after Phase 4 identified four areas with insufficient test depth:

- **Process tools** — Full handler tests using a real `process.Service`. Tests cover `echo`, `sleep`, `cat`, `pwd`, and `env` subprocesses, including validation, lifecycle transitions, and stdin/stdout round-trips.
- **RAG tools** — Handler validation for empty `question` and `path` fields, default application behaviour, and graceful degradation when Qdrant or Ollama are unavailable. Struct round-trips verified.
- **ML tools** — Mock implementations of `ml.Backend` and `inference.Backend` for CI use. Tests cover generate, score (heuristic, semantic, content), the 23 built-in probes, and the backends registry.
- **Metrics benchmarks** — Six benchmarks: `Record`, `Record` (parallel), `Query` at 10K and 50K events, `Summary`, and a full record-query-summary cycle. Unit test exercises 10,000 events.

---

## Known Limitations

The following issues are recorded in `FINDINGS.md`. None are blockers for current use but should be addressed before the module is considered production-hardened.

### 1. ML tools not fully abstracted via go-inference

`tools_ml.go` imports `forge.lthn.ai/core/go-ml` directly and uses `ml.Service`, `ml.GenOpts`, and `ml.HeuristicScores`. The intention noted in Phase 2 was to route all backend interaction through `go-inference` interfaces. Generation flows through `InferenceAdapter` but scoring and probes retain the direct `go-ml` dependency. Full abstraction would allow the ML backend to be swapped (for example, substituting a remote service or a different local runner) without modifying the MCP tool layer.

### 2. IDE bridge not tested against a real Laravel instance

The bridge tests use a mock WebSocket server. The bridge has not been exercised against a live `core-agentic` Laravel instance. In particular, reconnection behaviour under network partition — where the bridge is mid-conversation and the remote drops — has not been observed empirically. The exponential backoff logic (2 s base, 30 s ceiling) is unit-tested but not integration-tested.

### 3. Dashboard and build tools return placeholder data

The IDE dashboard (`ide_overview`, `ide_status`) and build (`build_start`, `build_status`, `build_logs`) tools forward requests to Laravel via the bridge and return typed response structs. In practice the structs contain empty or zero values at the time of the tool response; real data arrives asynchronously via WebSocket subscription. The current synchronous MCP tool-response model does not surface that asynchronous stream, so callers receive stubs. Resolving this would likely require a polling mechanism or a push-notification extension to the MCP protocol.

### 4. Webview tools support only a single Chrome connection

`webviewInstance` is a package-level variable in `tools_webview.go`. Consequently only one CDP connection to Chrome can be active at a time. For a single AI agent session this is adequate. Concurrent agent sessions requiring independent browser contexts would require the state to be moved into a per-session struct, which implies a more significant refactor of the tool registration pattern.

### 5. RAG tools require external services with no CI fallback

`rag_query`, `rag_ingest`, and `rag_collections` connect to Qdrant (vector store) and Ollama (embedding model). No mock or in-process fallback exists. In CI environments where these services are not running, the tests in Phase 5 cover only handler validation and graceful error paths; the actual RAG round-trip is not exercised. A lightweight mock Qdrant client would close this gap.

### 6. Unix transport has limited test coverage

`transport_unix.go` implements Unix domain socket serving and was included in the Phase 3 TCP and Unix e2e test suite at the integration level. However it has no unit-level test file of its own, and edge cases such as socket file cleanup on abnormal shutdown or `EADDRINUSE` handling on restart are not covered.

---

## Future Considerations

- **go-inference full adoption** — Once `go-ml` completes its own migration, the remaining direct `go-ml` imports in `tools_ml.go` should be replaced with `go-inference` calls. This would make the ML tool layer backend-agnostic and consistent with the `ml_backends` tool that was already migrated in Phase 2.

- **Asynchronous tool responses** — The MCP protocol as currently used returns a single synchronous result per tool call. The IDE build and dashboard tools would benefit from a streaming or subscription model. Monitoring the MCP specification for server-sent notifications or progress extensions is worthwhile.

- **Multi-session webview** — If `go-ai` is deployed in contexts where multiple agents run concurrently, the package-level webview state will become a contention point. Refactoring `webviewInstance` into a session-scoped registry is the natural next step.

- **RAG service abstraction** — Introducing a `Retriever` interface (analogous to `inference.TextModel`) would allow RAG tools to be tested in CI without live Qdrant and Ollama instances, and would make the vector store backend swappable.

- **IDE bridge integration tests** — Standing up a minimal `core-agentic` Laravel instance in CI (or a purpose-built stub server) would allow the full bridge lifecycle — connect, send, receive, reconnect — to be verified against realistic message formats.
