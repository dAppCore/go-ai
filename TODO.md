# TODO — go-ai

Virgil dispatches tasks. Mark `[x]` when done, note commit hash.

---

## Phase 1: Post-Split Cleanup

- [ ] **Remove `test-mlx.go`** — Standalone test script in module root. Not part of the library. Delete it.
- [ ] **Verify `go build ./...` passes** — With replace directives pointing at local clones (via go.work or go.mod). Fix any stale import paths that reference old monolith structure.
- [ ] **Verify `go vet ./...` passes** — Fix any vet warnings.
- [ ] **Run full test suite** — `go test ./...` should pass. 84 tests documented in TEST-RESULTS.md. Confirm they still pass after the split.

## Phase 2: go-inference Migration

go-ml is migrating to use `go-inference` shared interfaces. Once that's done, go-ai's ML subsystem should use go-inference too.

- [ ] **Update `tools_ml.go` MLSubsystem** — Currently imports `go-ml.Service` directly. After go-ml Phase 1, update to use `inference.LoadModel()` + `inference.TextModel` for generation. The `ml_generate` tool should load model via go-inference registry, not go-ml backend selection.
- [ ] **Update `ml_backends` tool** — Enumerate backends via `inference.List()` instead of go-ml service registry.
- [ ] **Update `ml_score` and `ml_probe`** — These use `go-ml.Engine` and `go-ml.Probes`. Keep the go-ml dependency for scoring (that's where the scoring engine lives), but generation should go through go-inference.
- [ ] **Add go-inference to go.mod** — `require forge.lthn.ai/core/go-inference v0.0.0` with appropriate replace directive.

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
