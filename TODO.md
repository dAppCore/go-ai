# TODO — go-ai

Virgil dispatches tasks. Mark `[x]` when done, note commit hash.

---

## Phase 1: Post-Split Cleanup

- [x] **Remove `test-mlx.go`** — Deleted standalone test script from module root.
- [x] **Verify `go build ./...` passes** — Clean build, no stale import paths.
- [x] **Verify `go vet ./...` passes** — No vet warnings.
- [x] **Run full test suite** — All tests pass. Fixed `TestSandboxing_Symlinks_Blocked` (renamed; asserts sandbox blocks symlink escape) and `TestNewTCPTransport_Warning` (added missing security warning to `NewTCPTransport`).

## Phase 2: go-inference Migration

go-ml is migrating to use `go-inference` shared interfaces. Once that's done, go-ai's ML subsystem should use go-inference too.

- [x] **Update `tools_ml.go` MLSubsystem** — mlGenerate/mlScore/mlProbe unchanged (work correctly via go-ml.Service → InferenceAdapter → inference.TextModel). Added flow documentation comments. `4d73fa2`
- [x] **Update `ml_backends` tool** — Rewritten to use `inference.List()/Get()/Default()` instead of `ml.Service.Backends()/Backend()/DefaultBackend()`. `4d73fa2`
- [x] **Update `ml_score` and `ml_probe`** — Kept go-ml dependency for scoring/probes (that's where the scoring engine lives). Generation flows through go-inference via InferenceAdapter. Added documentation comments. `4d73fa2`
- [x] **Add go-inference to go.mod** — Promoted from indirect to direct require. Replace directive already present. `4d73fa2`

## Phase 3: MCP Transport Testing

- [ ] **Stdio transport e2e** — Test `core mcp serve` over stdin/stdout with a mock MCP client. Verify tool discovery + file_read round-trip.
- [ ] **TCP transport e2e** — Test `MCP_ADDR=:9100 core mcp serve`. Connect, list tools, call `file_read`, verify response.
- [ ] **Unix transport** — Currently untested. Add basic connect + tool call test.
- [ ] **Webview tools CI guard** — `tools_webview.go` tools require Chrome. Add `testing.Short()` skip or build tag so CI doesn't fail.

## Phase 4: IDE Subsystem Hardening

- [ ] **Bridge reconnection test** — Kill mock Laravel WS server, verify exponential backoff + reconnect in `bridge.go`.
- [ ] **Add auth to bridge** — `bridge.go` connects unauthenticated. Add token header on WebSocket upgrade.
- [ ] **Dashboard tools beyond stubs** — `tools_dashboard.go` returns empty data. Implement real data fetching or document as stub.

## Phase 5: Testing Gaps

- [ ] **Process tools CI tests** — `tools_process.go` needs CI-safe tests (start/stop lightweight processes like `echo` or `sleep`).
- [ ] **RAG tools mock** — `tools_rag.go` needs Qdrant + Ollama mocks for CI. Test `rag_query`, `rag_ingest`, `rag_collections` without live services.
- [ ] **ML tools mock** — `tools_ml.go` needs mock backend for CI. No real inference in tests.
- [ ] **Metrics benchmark** — Benchmark `metrics_record` + `metrics_query` at scale (10K+ JSONL events).

---

## Workflow

1. Virgil in core/go writes tasks here after research
2. This repo's session picks up tasks in phase order
3. Mark `[x]` when done, note commit hash
4. New discoveries → add tasks, flag in FINDINGS.md
