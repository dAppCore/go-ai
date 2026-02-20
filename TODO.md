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

- [x] **Stdio transport e2e** — SDK's StdioTransport binds os.Stdin/Stdout directly; documented skip with rationale. Protocol covered by TCP/Unix e2e tests. `a6a7fb8`
- [x] **TCP transport e2e** — Full JSON-RPC round-trip: initialize → tools/list → tools/call file_read + file_write. Plus tools discovery and error handling tests. `a6a7fb8`
- [x] **Unix transport** — Full e2e via unix domain socket: initialize → tools/list → file_read + dir_list. Short socket paths for macOS sun_path limit. `a6a7fb8`
- [x] **Webview tools CI guard** — Added `skipIfShort()` guard + `TestWebviewToolHandlers_RequiresChrome` test (skipped with `-short`). Existing struct tests are CI-safe. `a6a7fb8`

## Phase 4: IDE Subsystem Hardening

- [x] **Bridge reconnection test** — Fixed data race (atomic.Int32), added exponential backoff test (HTTP 403 path), server shutdown detection test. `8c0ef43`
- [x] **Add auth to bridge** — Token field in Config, WithToken option, Bearer header in dial(). Tests for presence + absence. `8c0ef43`
- [x] **Dashboard/chat/build tool tests** — 49 tests: all 11 tool handlers with nil bridge (error) and connected mock (success), JSON round-trips for all types, stub doc comments. `8c0ef43`

## Phase 5: Testing Gaps

- [x] **Process tools CI tests** — Full handler tests using real process.Service with echo/sleep/cat/pwd/env. Validation, lifecycle, stdin/stdout round-trip. `2c745a6`
- [x] **RAG tools mock** — Handler validation (empty question/path), default application, graceful Qdrant/Ollama errors. Struct round-trips. `2c745a6`
- [x] **ML tools mock** — Mock ml.Backend + inference.Backend for CI. Generate, score (heuristic/semantic/content), probes (23), backends registry. `2c745a6`
- [x] **Metrics benchmark** — 6 benchmarks (Record, Parallel, Query 10K/50K, Summary, full cycle). 10K unit test. `2c745a6`

---

## Workflow

1. Virgil in core/go writes tasks here after research
2. This repo's session picks up tasks in phase order
3. Mark `[x]` when done, note commit hash
4. New discoveries → add tasks, flag in FINDINGS.md
