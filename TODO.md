# TODO — go-ai

Virgil dispatches tasks. Mark `[x]` when done.

## Phase 1: Post-Split Cleanup

- [ ] Verify all 49 tool registrations work after go-ml/go-rag/go-agentic extraction
- [ ] Update any stale import paths that still reference old monolith paths
- [ ] Test MCP Stdio transport end-to-end (`core mcp serve` over stdin/stdout)
- [ ] Test MCP TCP transport end-to-end (`MCP_ADDR=:9100 core mcp serve`)
- [ ] Test MCP Unix transport end-to-end (explicit socket path)
- [ ] Confirm `go build ./...` and `go vet ./...` pass cleanly with all replace directives
- [ ] Remove `test-mlx.go` from module root (standalone script, not part of the library)

## Phase 2: go-inference Migration

- [ ] Define `go-inference` Backend interface that abstracts over go-ml, go-mlx, and Ollama
- [ ] Update `tools_ml.go` MLSubsystem to use `go-inference` interfaces instead of direct `go-ml.Service`
- [ ] Decouple `ml_generate` from specific backend selection — let go-inference route
- [ ] Decouple `ml_score` and `ml_probe` from direct `go-ml` types
- [ ] Update `ml_backends` to enumerate backends via go-inference registry
- [ ] Remove direct `forge.lthn.ai/core/go-ml` import from `tools_ml.go` once migration is complete

## Phase 3: IDE Subsystem

- [ ] Bridge reconnection stress testing (kill Laravel, verify exponential backoff + reconnect)
- [ ] Add session management (track active sessions locally, not just fire-and-forget to Laravel)
- [ ] Implement dashboard tools beyond stubs (currently return empty data, rely on bridge forwarding)
- [ ] Add authentication to bridge WebSocket connection (token or shared secret)
- [ ] Test bridge with real Laravel core-agentic instance (not just unit mocks)
- [ ] Add heartbeat/ping-pong to detect stale connections faster

## Phase 4: Testing Gaps

- [ ] Add integration tests with actual MCP client (connect via TCP, call tools, verify responses)
- [ ] Webview tools are untested in CI (require Chrome with `--remote-debugging-port`)
- [ ] Process tools need CI-safe tests (start/stop/kill lightweight processes)
- [ ] RAG tools need Qdrant + Ollama test fixtures or mocks for CI
- [ ] ML tools need mock backend for CI (no real inference in tests)
- [ ] Add benchmark tests for metrics JSONL read/write at scale
- [ ] Unix transport has no tests

## Workflow

```
Virgil assigns → agent picks up → branch → implement → test → PR → merge
```

Standing rule: all tasks go through `core dev commit` with conventional commits.
