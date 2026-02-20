# go-ai Development Guide

go-ai is the MCP (Model Context Protocol) hub for the Lethean AI stack. It exposes 49 tools across
file operations, RAG vector search, ML inference and scoring, process management, WebSocket streaming,
browser automation via CDP, metrics, and IDE integration. This guide covers everything needed to build,
test, extend, and contribute to the repository.

Module path: `forge.lthn.ai/core/go-ai`
Licence: EUPL-1.2
Language: Go 1.25

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Building](#building)
3. [Testing](#testing)
4. [Test Patterns](#test-patterns)
5. [Dependencies](#dependencies)
6. [Integration Points](#integration-points)
7. [Adding a New Tool](#adding-a-new-tool)
8. [Adding a New Subsystem](#adding-a-new-subsystem)
9. [Coding Standards](#coding-standards)

---

## Prerequisites

### Go toolchain

Go 1.25 or later is required. The module uses a Go workspace (`go.work`) that spans multiple sibling
repositories. Ensure `go` is on your `PATH` and at the correct version:

```
go version go1.25.x darwin/arm64
```

### Sibling repositories

All `forge.lthn.ai/core/*` dependencies are resolved via `replace` directives that point to local
sibling directories. The expected layout on disk is:

```
~/Code/
├── go/             # forge.lthn.ai/core/go       — Core framework
├── go-inference/   # forge.lthn.ai/core/go-inference — shared inference interfaces
├── go-ml/          # forge.lthn.ai/core/go-ml    — ML scoring engine
├── go-mlx/         # forge.lthn.ai/core/go-mlx   — Native Metal GPU inference
├── go-rag/         # forge.lthn.ai/core/go-rag   — Qdrant + Ollama RAG
└── go-ai/          # forge.lthn.ai/core/go-ai    — this repository
```

If your checkouts live under a different root, update the `replace` directives in `go.mod`
accordingly before running any commands.

### Replace directives

The following directives in `go.mod` wire the local clones at build and test time:

```
replace forge.lthn.ai/core/go          => ../go
replace forge.lthn.ai/core/go-mlx      => ../go-mlx
replace forge.lthn.ai/core/go-ml       => ../go-ml
replace forge.lthn.ai/core/go-rag      => ../go-rag
replace forge.lthn.ai/core/go-inference => ../go-inference
```

After cloning a new sibling repo or after `go work sync`, run `go mod tidy` to keep the lock file
consistent.

---

## Building

go-ai is a library module. There is no `main` package; the MCP server is started by the Core CLI
(`core mcp serve`) which imports `forge.lthn.ai/core/go-ai/mcp`. Build the library to verify that
all packages compile cleanly:

```bash
go build ./...
```

To vet for suspicious constructs:

```bash
go vet ./...
```

Neither command produces a binary. If you need to run the server locally for manual testing, build
and invoke the Core CLI from the sibling `go` repository:

```bash
# From ~/Code/go (the Core CLI repository)
task cli:build
./bin/core mcp serve
```

By default this starts the MCP server on stdio. Set `MCP_ADDR=:9100` to bind a TCP listener instead,
which is useful when testing with an MCP client over the network.

---

## Testing

### Run all tests

```bash
go test ./...
```

### Run a single test by name

```bash
go test -run TestName ./mcp/...
```

The `-run` flag accepts a regex. To target a specific subsystem package:

```bash
go test -run TestBridge ./mcp/ide/...
```

### Verbose output

```bash
go test -v ./...
```

### Race detector

Always run with `-race` before opening a pull request, as the server handles concurrent connections
and the subsystem infrastructure uses goroutines:

```bash
go test -race ./...
```

### Short mode (CI)

Tests that require external services — Chrome via CDP, a live Qdrant instance, or a running Ollama
server — guard themselves with a `skipIfShort()` helper. Pass `-short` to skip those tests and run
only the unit and transport tests that are safe in CI:

```bash
go test -short ./...
```

The pattern for a CI guard inside a test function is:

```go
func TestWebviewNavigate_Good_RealBrowser(t *testing.T) {
    skipIfShort(t)
    // ... test using Chrome CDP
}
```

`skipIfShort` calls `t.Skip()` when `testing.Short()` returns true. It does not skip the test when
the flag is absent, so full end-to-end coverage is available locally.

---

## Test Patterns

### Naming convention

All test functions follow the `_Good`, `_Bad`, `_Ugly` suffix pattern:

| Suffix  | Purpose |
|---------|---------|
| `_Good` | Happy path — the input is valid and the operation should succeed |
| `_Bad`  | Expected error conditions — invalid input, missing prerequisites, wrong state |
| `_Ugly` | Panics and extreme edge cases — nil receivers, concurrent mutation, resource exhaustion |

Examples from the codebase:

```go
func TestMLGenerate_Good_WithMockBackend(t *testing.T) { ... }
func TestMLGenerate_Bad_EmptyPrompt(t *testing.T)       { ... }
func TestMLGenerate_Bad_NoBackend(t *testing.T)         { ... }
```

### Mock subsystems

When testing a tool handler in isolation, build a mock backend or service and wire it into the real
subsystem constructor. The ML tools demonstrate this pattern:

```go
// mockMLBackend implements ml.Backend without requiring Ollama or Metal GPU.
type mockMLBackend struct {
    name         string
    available    bool
    generateResp string
    generateErr  error
}

func (m *mockMLBackend) Name() string      { return m.name }
func (m *mockMLBackend) Available() bool   { return m.available }
func (m *mockMLBackend) Generate(_ context.Context, _ string, _ ml.GenOpts) (string, error) {
    return m.generateResp, m.generateErr
}
func (m *mockMLBackend) Chat(_ context.Context, _ []ml.Message, _ ml.GenOpts) (string, error) {
    return m.generateResp, m.generateErr
}

// Wire the mock into a real MLSubsystem via the framework:
func newTestMLSubsystem(t *testing.T, backends ...ml.Backend) *MLSubsystem {
    t.Helper()
    c, err := framework.New(
        framework.WithName("ml", ml.NewService(ml.Options{})),
    )
    if err != nil {
        t.Fatalf("Failed to create framework core: %v", err)
    }
    svc, _ := framework.ServiceFor[*ml.Service](c, "ml")
    for _, b := range backends {
        svc.RegisterBackend(b.Name(), b)
    }
    return &MLSubsystem{service: svc, logger: log.Default()}
}
```

### Mock inference backends

The global inference registry (`inference.Register`) accepts any `inference.Backend`. Register a
lightweight mock to test tool handlers that enumerate available backends without loading model
weights:

```go
type mockInferenceBackend struct {
    name      string
    available bool
}

func (m *mockInferenceBackend) Name() string      { return m.name }
func (m *mockInferenceBackend) Available() bool   { return m.available }
func (m *mockInferenceBackend) LoadModel(_ string, _ ...inference.LoadOption) (inference.TextModel, error) {
    return nil, fmt.Errorf("mock backend: LoadModel not implemented")
}

// Register before the test and the backend appears in ml_backends output:
inference.Register(&mockInferenceBackend{name: "test-ci-mock", available: true})
```

Note that `inference.Register` is global state. If you register a mock in a test, it will persist
for the lifetime of the test binary. Use unique names to avoid conflicts between parallel test runs.

### Real services in CI

For process management tests, construct a real `process.Service` backed by the framework. These
tests run safely in CI because they only execute standard UNIX utilities (`echo`, `sleep`, `cat`):

```go
func newTestProcessService(t *testing.T) *process.Service {
    t.Helper()
    c, err := framework.New(
        framework.WithName("process", process.NewService(process.Options{})),
    )
    if err != nil {
        t.Fatalf("Failed to create framework core: %v", err)
    }
    svc, _ := framework.ServiceFor[*process.Service](c, "process")
    _ = c.ServiceStartup(context.Background(), nil)
    t.Cleanup(func() { _ = c.ServiceShutdown(context.Background()) })
    return svc
}
```

### Transport end-to-end tests

TCP and Unix socket transport tests speak raw JSON-RPC 2.0 over a live server goroutine. They verify
the full call path from wire format through to handler response without requiring any external process.
The pattern is:

1. Find a free port (or create a temporary socket path).
2. Start the server in a goroutine, cancel via `context.WithCancel`.
3. Dial, exchange `initialize` / `notifications/initialized`, then call `tools/list` or `tools/call`.
4. Cancel context and drain the error channel to verify graceful shutdown.

The helper `readJSONRPCResponse` handles server-initiated pings transparently, so tests do not need
to account for interleaved protocol messages.

Unix socket paths on macOS are limited to 104 bytes. Use the `shortSocketPath` helper to generate
paths under `/tmp` rather than relying on `t.TempDir()`, which produces paths that are often too
long:

```go
func shortSocketPath(t *testing.T, suffix string) string {
    t.Helper()
    path := fmt.Sprintf("/tmp/mcp-test-%s-%d.sock", suffix, os.Getpid())
    t.Cleanup(func() { os.Remove(path) })
    return path
}
```

### IDE bridge tests

The bridge tests in `mcp/ide/bridge_test.go` use `net/http/httptest` to stand up a real WebSocket
server in-process. This keeps tests hermetic while exercising the reconnection logic, exponential
backoff, authentication headers, and message dispatch. The `waitConnected` helper polls
`bridge.Connected()` with a deadline rather than using fixed sleeps.

---

## Dependencies

### Direct dependencies

| Module | Role |
|--------|------|
| `forge.lthn.ai/core/go` | Core framework: `pkg/io` (sandboxed filesystem), `pkg/log`, `pkg/process`, `pkg/ws`, `pkg/webview` |
| `forge.lthn.ai/core/go-ml` | ML scoring engine: heuristic scores, judge backend, capability probes, InfluxDB status |
| `forge.lthn.ai/core/go-rag` | RAG: Qdrant vector database client, Ollama embeddings, Markdown chunking |
| `forge.lthn.ai/core/go-inference` | Shared `TextModel`, `Backend`, and `Token` interfaces — zero external dependencies |
| `github.com/modelcontextprotocol/go-sdk` | MCP Go SDK: server, transports, JSON-RPC framing |
| `github.com/gorilla/websocket` | WebSocket client used by the IDE bridge to connect to Laravel |
| `github.com/stretchr/testify` | Test assertions and require helpers |

### Indirect dependencies

The following packages are pulled in transitively through `go-ml` and `go-rag`. They are not
imported directly by go-ai but are present in `go.sum`:

- `forge.lthn.ai/core/go-mlx` — Native Metal GPU inference (via go-ml)
- `github.com/qdrant/go-client` — Qdrant gRPC client (via go-rag)
- `github.com/ollama/ollama` — Ollama API client (via go-rag)
- `github.com/marcboeker/go-duckdb` — DuckDB driver (via go-rag)
- `github.com/parquet-go/parquet-go` — Parquet file format (via go-rag)
- `github.com/apache/arrow-go/v18` — Arrow columnar format (via go-rag)

### Replace directives for local development

During development, all `forge.lthn.ai/core/*` modules resolve to local directories via `replace`
directives. This means changes in a sibling repo are immediately visible without publishing to Forge.
After modifying a sibling, run:

```bash
go build ./...   # verify compilation
go test ./...    # verify tests
```

When preparing a release, the `replace` directives are removed and proper tagged versions are
referenced.

---

## Integration Points

### Core CLI bootstrap

The MCP server has no `main` package. It is bootstrapped exclusively by the Core CLI
(`forge.lthn.ai/core/go`) via a call such as:

```go
svc, err := mcp.New(
    mcp.WithWorkspaceRoot("/path/to/workspace"),
    mcp.WithProcessService(ps),
    mcp.WithWSHub(hub),
    mcp.WithSubsystem(ide.New(hub)),
    mcp.WithSubsystem(mcp.NewMLSubsystem(mlSvc)),
)
if err != nil {
    return err
}
svc.Run(ctx)  // selects stdio or TCP based on MCP_ADDR
```

`Run` selects the transport automatically: if the `MCP_ADDR` environment variable is set, it binds
a TCP listener on that address; otherwise it uses stdio. A Unix socket can be started explicitly via
`ServeUnix`.

### Laravel core-agentic WebSocket bridge

The IDE subsystem (`mcp/ide/`) maintains a persistent WebSocket connection to the Laravel
`core-agentic` application. The default endpoint is `ws://localhost:9876/ws`. When the connection
drops, the bridge performs exponential backoff reconnection up to a configurable maximum interval.

Incoming messages from Laravel are dispatched to the local WebSocket hub (`pkg/ws`), making them
available to any connected MCP client that has subscribed to the relevant channel. Outgoing messages
(such as `ide_chat_send`) are forwarded over this bridge to Laravel.

To configure a non-default URL or an authentication token:

```go
cfg := ide.DefaultConfig()
cfg.LaravelWSURL = "ws://localhost:9876/ws"
cfg.Token = "your-bearer-token"
sub := ide.New(hub, ide.WithToken("your-bearer-token"))
```

### Qdrant and Ollama for RAG

The `rag_query`, `rag_ingest`, and `rag_collections` tools delegate to `go-rag`, which connects to:

- **Qdrant** at `http://localhost:6333` by default — the vector database storing embedded chunks
- **Ollama** at `http://localhost:11434` by default — generates embeddings from text

Both services must be running and reachable for RAG tools to function. In CI, tests that touch RAG
tools are guarded with `skipIfShort(t)` so the build does not fail when these services are absent.

The `ai` package provides a higher-level facade (`QueryRAGForTask`) that degrades gracefully: if
Qdrant is unreachable, it returns an empty result set rather than an error, allowing tools to
continue operating.

### Chrome for webview tools

The ten `webview_*` tools automate a running Chrome browser via the Chrome DevTools Protocol. Chrome
must be launched with the remote debugging port open:

```bash
google-chrome --remote-debugging-port=9222
# or on macOS:
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222
```

Pass the debug URL when constructing the webview tool if not using the default port. Tests that
require a live Chrome instance are guarded with `skipIfShort(t)`.

### JSONL metrics

The `metrics_record` and `metrics_query` tools persist events to newline-delimited JSON files at:

```
~/.core/ai/metrics/YYYY-MM-DD.jsonl
```

Each line is a JSON object with at least a `timestamp` field. The `ai/metrics.go` package provides
`Record()` and `ReadEvents()` functions that go-ai tools delegate to. No external database is
required; the files are append-only and readable with standard JSON tooling.

---

## Adding a New Tool

Tools are registered in the MCP server during construction. Follow these steps to add a new tool to
an existing group.

### Step 1: Define input and output structs

Create typed structs for the tool's parameters and return value. Place them in the same file as the
handler or in a dedicated file for the group.

```go
// FileChecksumInput is the input for the file_checksum tool.
type FileChecksumInput struct {
    Path      string `json:"path"      description:"Path to the file, relative to the workspace root"`
    Algorithm string `json:"algorithm" description:"Hash algorithm: md5, sha1, sha256 (default: sha256)"`
}

// FileChecksumOutput is the output of the file_checksum tool.
type FileChecksumOutput struct {
    Path      string `json:"path"`
    Algorithm string `json:"algorithm"`
    Checksum  string `json:"checksum"`
}
```

### Step 2: Write the handler function

The handler receives a `context.Context`, a `*mcp.CallToolRequest` (may be nil in unit tests), and
the typed input struct. It returns a `*mcp.CallToolResult`, the typed output struct, and an error.

```go
func (s *Service) fileChecksum(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input FileChecksumInput,
) (*mcp.CallToolResult, FileChecksumOutput, error) {
    if input.Path == "" {
        return nil, FileChecksumOutput{}, fmt.Errorf("path cannot be empty")
    }
    if input.Algorithm == "" {
        input.Algorithm = "sha256"
    }

    content, err := s.medium.Read(input.Path)
    if err != nil {
        return nil, FileChecksumOutput{}, fmt.Errorf("file_checksum: read %q: %w", input.Path, err)
    }

    sum, err := computeChecksum(input.Algorithm, content)
    if err != nil {
        return nil, FileChecksumOutput{}, fmt.Errorf("file_checksum: compute %s: %w", input.Algorithm, err)
    }

    out := FileChecksumOutput{
        Path:      input.Path,
        Algorithm: input.Algorithm,
        Checksum:  sum,
    }
    result := mcp.NewToolResultText(fmt.Sprintf("%s  %s", out.Checksum, out.Path))
    return result, out, nil
}
```

Errors must always be wrapped with context using `fmt.Errorf("tool_name: action: %w", err)`. Never
panic in a handler; return the error instead and let the MCP SDK translate it into a JSON-RPC error
response.

### Step 3: Register the tool

Open `mcp/mcp.go` (for core file/dir/language tools) or the relevant `tools_*.go` file for the
group. Add the registration in `registerTools` or in the subsystem's `RegisterTools` method:

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "file_checksum",
    Description: "Compute a checksum of a file within the workspace",
}, s.fileChecksum)
```

The third argument must match the handler's signature exactly.

### Step 4: Add tests

Create a test file (e.g. `mcp/tools_file_checksum_test.go`) following the `_Good`/`_Bad`/`_Ugly`
naming convention:

```go
func TestFileChecksum_Good_SHA256(t *testing.T) {
    tmpDir := t.TempDir()
    _ = os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("hello"), 0644)

    s, _ := New(WithWorkspaceRoot(tmpDir))
    _, out, err := s.fileChecksum(context.Background(), nil, FileChecksumInput{
        Path: "data.txt",
    })
    if err != nil {
        t.Fatalf("fileChecksum failed: %v", err)
    }
    if out.Algorithm != "sha256" {
        t.Errorf("expected algorithm 'sha256', got %q", out.Algorithm)
    }
    if out.Checksum == "" {
        t.Error("expected non-empty checksum")
    }
}

func TestFileChecksum_Bad_EmptyPath(t *testing.T) {
    s, _ := New(WithWorkspaceRoot(t.TempDir()))
    _, _, err := s.fileChecksum(context.Background(), nil, FileChecksumInput{})
    if err == nil {
        t.Fatal("expected error for empty path")
    }
}

func TestFileChecksum_Bad_NonexistentFile(t *testing.T) {
    s, _ := New(WithWorkspaceRoot(t.TempDir()))
    _, _, err := s.fileChecksum(context.Background(), nil, FileChecksumInput{Path: "missing.txt"})
    if err == nil {
        t.Fatal("expected error for nonexistent file")
    }
}
```

Verify the new tests pass before opening a pull request:

```bash
go test -run TestFileChecksum ./mcp/...
```

---

## Adding a New Subsystem

Subsystems extend the MCP server with additional tool groups. They are decoupled from the core
`Service` and registered at construction time via `WithSubsystem()`.

### Step 1: Implement the Subsystem interface

```go
// Subsystem interface (defined in mcp/subsystem.go):
type Subsystem interface {
    Name() string
    RegisterTools(server *mcp.Server)
}
```

Create a new package under `mcp/` or alongside the relevant sibling repository integration:

```go
// mcp/metrics2/metrics2.go
package metrics2

import (
    "context"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Subsystem struct {
    // ... fields
}

func New() *Subsystem {
    return &Subsystem{}
}

func (s *Subsystem) Name() string { return "metrics2" }

func (s *Subsystem) RegisterTools(server *mcp.Server) {
    mcp.AddTool(server, &mcp.Tool{
        Name:        "metrics2_summary",
        Description: "Return a summary of recorded metrics",
    }, s.summary)
}
```

### Step 2: Optionally implement SubsystemWithShutdown

If the subsystem holds resources (connections, goroutines, file handles), implement the shutdown
interface so the MCP server can clean up gracefully when its context is cancelled:

```go
// SubsystemWithShutdown interface (defined in mcp/subsystem.go):
type SubsystemWithShutdown interface {
    Subsystem
    Shutdown(ctx context.Context) error
}

func (s *Subsystem) Shutdown(ctx context.Context) error {
    // close connections, signal goroutines to stop, etc.
    return nil
}
```

The MCP `Service.Shutdown` method iterates over registered subsystems, checks whether each
implements `SubsystemWithShutdown`, and calls `Shutdown` if so.

### Step 3: Register via WithSubsystem

Pass the subsystem to `mcp.New` in the Core CLI bootstrap code:

```go
import "forge.lthn.ai/core/go-ai/mcp/metrics2"

svc, err := mcp.New(
    mcp.WithWorkspaceRoot(root),
    mcp.WithSubsystem(metrics2.New()),
)
```

### Step 4: Add tests

Follow the same patterns as for individual tools. For subsystems that own connections, use
`net/http/httptest` or in-process stubs to avoid external service dependencies in CI. Guard any
tests that need real external services with `skipIfShort(t)`.

---

## Coding Standards

### Language

Use **UK English** in all comments, documentation, log messages, and user-facing strings:
`colour`, `organisation`, `centre`, `initialise`, `licence` (noun), `license` (verb).

### Error handling

- Always return errors rather than panicking.
- Wrap errors with context: `fmt.Errorf("subsystem.Operation: what went wrong: %w", err)`.
- Do not discard errors with `_` unless the operation is genuinely fire-and-forget and the error is
  documented as ignorable.
- Log errors at the point of handling, not at the point of wrapping, to avoid duplicate log entries.

### Test naming

- Function names: `Test{Type}_{Suffix}_{Description}` where `{Suffix}` is `Good`, `Bad`, or `Ugly`.
- Helper constructors: `newTest{Type}(t *testing.T, ...) *Type`.
- Call `t.Helper()` at the top of every test helper function.

### Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(mcp): add file_checksum tool with sha256 default

Adds a sandboxed file checksum tool supporting md5, sha1, and sha256.
Defaults to sha256 when the algorithm field is omitted.

Co-Authored-By: Virgil <virgil@lethean.io>
```

Types in use across the repository: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`.

### Formatting

The codebase uses `gofmt` defaults. Run before committing:

```bash
gofmt -l -w .
```

There is no Pint or equivalent; standard `gofmt` is sufficient.

### Licence header

Every new Go source file must carry the EUPL-1.2 SPDX identifier in a comment block at the top:

```go
// SPDX-License-Identifier: EUPL-1.2
// Copyright (c) Lethean contributors
```

Do not add licence headers to test files unless the project convention changes.
