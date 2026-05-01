# Repository Agent Guide

This repository is `dappco.re/go/ai`, the Go AI integration layer for the
Core ecosystem. It provides CLI commands, an MCP daemon, API provider
handlers, lightweight RAG and metrics adapters, and local desktop helpers.

The repository follows the Core v0.9 compliance shape:

- Import the Core facade as `core "dappco.re/go"` in production code, or use
  a dot import from `dappco.re/go` in focused tests and examples.
- Do not import the banned standard-library packages directly in repository
  Go files. Use Core wrappers for formatting, errors, JSON, filesystem,
  environment, string, path, logging, and process boundary work.
- Public functions and methods are tested in the source sibling test file with
  `Test<File>_<Symbol>_Good`, `Bad`, and `Ugly` triplets. Each triplet calls the
  symbol it names and asserts returned state or errors.
- Public symbols also need examples in the matching sibling
  `<file>_example_test.go` file. Examples print through Core `Println` and keep
  `// Output:` exact.
- Avoid generated AX-7 dumping-ground files. Tests live with the file whose
  public surface they exercise.

The MCP package is the operational center of the repository. `service.go`
owns tool registration and typed handler plumbing; `tools_core.go` owns file,
directory, and language tools; `tools_external.go` owns RAG, metrics, process,
WebSocket, browser, IDE, and dashboard tools; the transport files expose
stdio, TCP, and Unix socket serving.

Before handing work back, run the repository verification sequence from the
brief. The audit script is the source of truth for compliance status.
