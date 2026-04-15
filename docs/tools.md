# go-ai MCP Tool Reference

**Module**: `forge.lthn.ai/core/go-ai`

This document is the authoritative reference for every MCP tool exposed by the go-ai hub. It is generated from the source definitions in `mcp/` and `mcp/ide/`.

---

## Overview

go-ai exposes **49 tools** across 13 functional groups via the [Model Context Protocol](https://modelcontextprotocol.io/). The server can be started in two modes:

- **Stdio** (default): `core mcp serve` — standard MCP transport for use with Claude, Cursor, and other MCP clients.
- **TCP**: `MCP_ADDR=:9000 core mcp serve` — listens on a TCP socket for programmatic access.

### Registration model

Tools are registered in two tiers:

1. **Built-in tools** (registered unconditionally in `mcp.New()`): file operations, directory operations, language detection, RAG, metrics, and webview tools. These are always present regardless of configuration.

2. **Subsystem tools** (registered via `WithSubsystem()`): ML, process management, WebSocket, and IDE tools. These require the corresponding backend service to be provided via an `Option` at construction time. Process and WebSocket tools are silently omitted if their respective services are not supplied. IDE tools are provided by the `ide.Subsystem` subsystem.

### Workspace sandboxing

All file and directory tools operate within a sandboxed filesystem medium. By default the sandbox root is the current working directory at the time `mcp.New()` is called. The root can be changed with `WithWorkspaceRoot(path)`. All paths supplied to file tools are validated to remain within this root — path traversal attempts are rejected.

### Audit logging

Every tool execution is logged. Tools that perform writes, process control, or browser automation are logged at `Security` level. Read-only tools are logged at `Info` level.

---

## Tool Groups

| Group | Tools | File |
|---|---|---|
| File Operations | 6 | `mcp/mcp.go` |
| Directory Operations | 2 | `mcp/mcp.go` |
| Language Detection | 2 | `mcp/mcp.go` |
| RAG — Vector Search | 3 | `mcp/tools_rag.go` |
| ML — Inference & Scoring | 5 | `mcp/tools_ml.go` |
| Metrics | 2 | `mcp/tools_metrics.go` |
| Process Management | 6 | `mcp/tools_process.go` |
| WebSocket | 2 | `mcp/tools_ws.go` |
| Browser Automation | 10 | `mcp/tools_webview.go` |
| IDE Chat | 5 | `mcp/ide/tools_chat.go` |
| IDE Build | 3 | `mcp/ide/tools_build.go` |
| IDE Dashboard | 3 | `mcp/ide/tools_dashboard.go` |

---

## File Operations

Six tools for reading, writing, editing, renaming, deleting, and checking files. All paths are resolved relative to the configured workspace root and must not escape it.

---

### `file_read`

Read the contents of a file.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | Path to the file, relative to workspace root |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Content | `string` | `content` | Full file contents |
| Language | `string` | `language` | Detected language ID (see `lang_detect`) |
| Path | `string` | `path` | Echoed input path |

**Notes**

The language field is populated by extension-based detection and will be `"plaintext"` for unrecognised extensions. The call fails if the path does not exist or points outside the workspace root.

---

### `file_write`

Write content to a file, creating it and any intermediate directories if they do not exist.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | Destination path, relative to workspace root |
| Content | `string` | `content` | Yes | Content to write |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the write succeeded |
| Path | `string` | `path` | Echoed input path |

**Notes**

Overwrites any existing file at the given path without warning. Parent directories are created automatically. Logged at `Security` level.

---

### `file_edit`

Edit a file by replacing one occurrence (or all occurrences) of `old_string` with `new_string`. The file must already exist.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | Path to the file |
| OldString | `string` | `old_string` | Yes | Exact string to find; must not be empty |
| NewString | `string` | `new_string` | Yes | Replacement string (may be empty to delete) |
| ReplaceAll | `bool` | `replace_all` | No | Replace every occurrence; default is first only |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Path | `string` | `path` | Echoed input path |
| Success | `bool` | `success` | Whether the edit succeeded |
| Replacements | `int` | `replacements` | Number of occurrences replaced |

**Notes**

Returns an error if `old_string` is empty or is not found in the file. With `replace_all=false` (default) only the first occurrence is replaced and `replacements` will be `1`. With `replace_all=true` all occurrences are replaced and `replacements` reflects the total count.

---

### `file_delete`

Delete a file or empty directory.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | Path to delete |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the deletion succeeded |
| Path | `string` | `path` | Echoed input path |

**Notes**

Non-empty directories are not deleted; use multiple calls or a process tool to remove them recursively. Logged at `Security` level.

---

### `file_rename`

Rename or move a file within the workspace.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| OldPath | `string` | `oldPath` | Yes | Current path |
| NewPath | `string` | `newPath` | Yes | Destination path |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the rename succeeded |
| OldPath | `string` | `oldPath` | Echoed source path |
| NewPath | `string` | `newPath` | Echoed destination path |

**Notes**

Both paths must remain within the workspace root. Can be used to move files between subdirectories. Logged at `Security` level.

---

### `file_exists`

Check whether a path exists and whether it is a file or directory.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | Path to check |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Exists | `bool` | `exists` | Whether the path exists |
| IsDir | `bool` | `isDir` | Whether the path is a directory |
| Path | `string` | `path` | Echoed input path |

**Notes**

Returns `exists=false` for paths that do not exist or are inaccessible. Does not distinguish between a missing path and a permission error.

---

## Directory Operations

Two tools for listing and creating directories within the workspace.

---

### `dir_list`

List the contents of a directory.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | Directory path, relative to workspace root |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Entries | `[]DirectoryEntry` | `entries` | List of entries |
| Path | `string` | `path` | Echoed input path |

**DirectoryEntry fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Name | `string` | `name` | Entry name (filename only) |
| Path | `string` | `path` | Entry path (joined with input path) |
| IsDir | `bool` | `isDir` | Whether the entry is a directory |
| Size | `int64` | `size` | File size in bytes; 0 for directories |

**Notes**

Lists only the immediate children; not recursive. The `path` field in each entry preserves the relative form of the input path.

---

### `dir_create`

Create a new directory, including any necessary parent directories.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | Directory path to create |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the directory was created |
| Path | `string` | `path` | Echoed input path |

**Notes**

Behaves like `mkdir -p`; does not fail if the directory already exists.

---

## Language Detection

Two tools for working with the built-in language registry.

---

### `lang_detect`

Detect the programming language of a file based on its extension or name.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | File path (the file need not exist; only the name is inspected) |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Language | `string` | `language` | Detected language ID |
| Path | `string` | `path` | Echoed input path |

**Notes**

Detection is purely extension-based. `Dockerfile` (exact name, no extension) is detected as `"dockerfile"`. Unknown extensions return `"plaintext"`.

---

### `lang_list`

Get the list of all supported programming languages.

**Parameters**

None.

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Languages | `[]LanguageInfo` | `languages` | All supported language entries |

**LanguageInfo fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Language identifier used throughout the API |
| Name | `string` | `name` | Human-readable display name |
| Extensions | `[]string` | `extensions` | File extensions that map to this language |

**Supported languages**

`typescript`, `javascript`, `go`, `python`, `rust`, `java`, `php`, `ruby`, `html`, `css`, `json`, `yaml`, `markdown`, `sql`, `shell`

The full detection set (including `c`, `cpp`, `csharp`, `scss`, `xml`, `swift`, `kotlin`, `dockerfile`) is available via `lang_detect` but is not enumerated by `lang_list`.

---

## RAG — Vector Search

Three tools for semantic document retrieval and ingestion. These tools require:

- **Qdrant** running at `localhost:6334` (default gRPC port)
- **Ollama** running at `localhost:11434` for embedding generation

The default collection name is `hostuk-docs`.

---

### `rag_query`

Query the RAG vector database for relevant documentation. Returns semantically similar content based on the query.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Question | `string` | `question` | Yes | The natural-language query |
| Collection | `string` | `collection` | No | Collection name; default `hostuk-docs` |
| TopK | `int` | `topK` | No | Number of results to return; default `5` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Results | `[]RAGQueryResult` | `results` | Ordered list of matching chunks |
| Query | `string` | `query` | Echoed question |
| Collection | `string` | `collection` | Collection used |
| Context | `string` | `context` | Pre-formatted context string suitable for injection into a prompt |

**RAGQueryResult fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Content | `string` | `content` | The chunk text |
| Source | `string` | `source` | Original source file or URL |
| Section | `string` | `section` | Section heading within the source (if available) |
| Category | `string` | `category` | Category tag (if available) |
| ChunkIndex | `int` | `chunkIndex` | Chunk position within the source document |
| Score | `float32` | `score` | Cosine similarity score |

**Notes**

Returns an error if `question` is empty. Results are ordered by descending similarity score. The `context` field is ready for direct use in a system prompt.

---

### `rag_ingest`

Ingest documents into the RAG vector database. Supports both single files and entire directories.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Path | `string` | `path` | Yes | File or directory path to ingest |
| Collection | `string` | `collection` | No | Target collection; default `hostuk-docs` |
| Recreate | `bool` | `recreate` | No | Drop and recreate the collection before ingestion |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether ingestion succeeded |
| Path | `string` | `path` | Echoed input path |
| Collection | `string` | `collection` | Collection used |
| Chunks | `int` | `chunks` | Number of chunks indexed (file only; `0` for directory) |
| Message | `string` | `message` | Human-readable summary |

**Notes**

The path is validated against the workspace medium (sandboxed). For directory ingestion the chunk count is not returned. Setting `recreate=true` destroys all existing vectors in the collection before ingesting. Logged at `Security` level.

---

### `rag_collections`

List all available collections in the Qdrant vector database.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| ShowStats | `bool` | `show_stats` | No | Include point count and status for each collection |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Collections | `[]CollectionInfo` | `collections` | List of collections |

**CollectionInfo fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Name | `string` | `name` | Collection name |
| PointsCount | `uint64` | `points_count` | Total vectors stored (only present when `show_stats=true`) |
| Status | `string` | `status` | Qdrant collection status (only present when `show_stats=true`) |

**Notes**

Opens a short-lived Qdrant client connection per call. Stats are fetched per-collection and failures for individual collections are logged and skipped rather than aborting the entire response.

---

## ML — Inference & Scoring

Five tools for text generation, response scoring, capability probing, pipeline status, and backend enumeration. These tools are provided by the `MLSubsystem` and require a configured `go-ml` service instance (`ml.Service`).

The generation path is: `go-ai` → `go-ml.Service.Generate` → `InferenceAdapter` → `inference.TextModel` (from `go-inference`).

---

### `ml_generate`

Generate text via a configured ML inference backend.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Prompt | `string` | `prompt` | Yes | The input prompt |
| Backend | `string` | `backend` | No | Backend name; uses service default if omitted |
| Model | `string` | `model` | No | Model override for backends that support multiple models |
| Temperature | `float64` | `temperature` | No | Sampling temperature |
| MaxTokens | `int` | `max_tokens` | No | Maximum tokens to generate |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Response | `string` | `response` | Generated text |
| Backend | `string` | `backend` | Backend used |
| Model | `string` | `model` | Model used (if applicable) |

**Notes**

Returns an error if `prompt` is empty. Backend selection falls through to the service default when `backend` is not specified. Available backends can be queried with `ml_backends`.

---

### `ml_score`

Score a prompt/response pair using one or more scoring suites.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Prompt | `string` | `prompt` | Yes | The original prompt |
| Response | `string` | `response` | Yes | The model response to evaluate |
| Suites | `string` | `suites` | No | Comma-separated suite names; default `heuristic` |

Available suites: `heuristic`, `semantic`

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Heuristic | `*ml.HeuristicScores` | `heuristic` | Length, structure, and format heuristics (present when requested) |
| Semantic | `*ml.SemanticScores` | `semantic` | LLM judge scores (present when requested) |
| Content | `*ml.ContentScores` | `content` | Content scores (not available via this tool; use `ml_probe`) |

**Notes**

`heuristic` scoring requires no backend. `semantic` scoring requires a judge backend to be configured in the ML service; the call returns an error if none is present. The `content` suite is not supported by this tool — use `ml_probe` instead. Both `prompt` and `response` must be non-empty.

---

### `ml_probe`

Run capability probes against an inference backend. Each probe sends a structured prompt to the backend and records the response.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Backend | `string` | `backend` | No | Backend name; uses service default if omitted |
| Categories | `string` | `categories` | No | Comma-separated category names to filter probes; all categories run if omitted |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Total | `int` | `total` | Number of probes executed |
| Results | `[]MLProbeResultItem` | `results` | Per-probe results |

**MLProbeResultItem fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Probe identifier |
| Category | `string` | `category` | Probe category |
| Response | `string` | `response` | Backend response, or an error string prefixed with `"error: "` |

**Notes**

Probes are drawn from `ml.CapabilityProbes` defined in `go-ml`. Individual probe failures are captured as error strings in the response rather than aborting the entire run. Temperature is fixed at `0.7` and `MaxTokens` at `2048` for all probes.

---

### `ml_status`

Show training and generation progress from InfluxDB.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| InfluxURL | `string` | `influx_url` | No | InfluxDB URL; default `http://localhost:8086` |
| InfluxDB | `string` | `influx_db` | No | Database name; default `lem` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Status | `string` | `status` | Formatted status report as a multi-line string |

**Notes**

Connects to InfluxDB on each call. Returns an error if the InfluxDB instance is unreachable.

---

### `ml_backends`

List available inference backends and their status.

**Parameters**

None.

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Backends | `[]MLBackendInfo` | `backends` | All registered backends |
| Default | `string` | `default` | Name of the default backend |

**MLBackendInfo fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Name | `string` | `name` | Backend identifier |
| Available | `bool` | `available` | Whether the backend is currently available |

**Notes**

Queries the `go-inference` global registry directly, bypassing `go-ml.Service`. This is the canonical source of truth for which backends are compiled in and available at runtime.

---

## Metrics

Two tools for recording and querying agent activity events. Events are stored in daily JSONL files on the local filesystem via the `go-ai/ai` package.

---

### `metrics_record`

Record a metrics event for AI or security tracking.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Type | `string` | `type` | Yes | Event type identifier |
| AgentID | `string` | `agent_id` | No | Agent identifier |
| Repo | `string` | `repo` | No | Repository name associated with the event |
| Data | `map[string]any` | `data` | No | Arbitrary additional event data |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the event was recorded |
| Timestamp | `time.Time` | `timestamp` | Server-assigned event timestamp |

**Notes**

The timestamp is assigned by the server at the moment of recording; any timestamp in `data` is treated as user data only. Returns an error if `type` is empty.

---

### `metrics_query`

Query metrics events and get aggregated statistics by type, repository, and agent.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Since | `string` | `since` | No | Lookback window; default `7d` |

Supported duration formats: `Nd` (days), `Nh` (hours), `Nm` (minutes). Examples: `"7d"`, `"24h"`, `"30m"`.

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ByType | `map[string]int` | `by_type` | Event counts grouped by type |
| ByRepo | `map[string]int` | `by_repo` | Event counts grouped by repository |
| ByAgent | `map[string]int` | `by_agent` | Event counts grouped by agent ID |
| Recent | `[]Event` | `recent` | The 10 most recent events in chronological order |

**Notes**

Returns an error if the `since` value cannot be parsed. The duration unit must be a single character: `d`, `h`, or `m`. Durations must be positive integers.

---

## Process Management

Six tools for starting, stopping, and interacting with external processes. These tools are only registered when a `process.Service` instance is provided via `WithProcessService()` at construction time.

All process-control operations (`process_start`, `process_stop`, `process_kill`, `process_input`) are logged at `Security` level.

---

### `process_start`

Start a new external process. Returns a process ID for tracking.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Command | `string` | `command` | Yes | The executable to run |
| Args | `[]string` | `args` | No | Command-line arguments |
| Dir | `string` | `dir` | No | Working directory for the process |
| Env | `[]string` | `env` | No | Additional environment variables in `KEY=VALUE` format |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Unique process identifier used in subsequent calls |
| PID | `int` | `pid` | Operating system process ID |
| Command | `string` | `command` | Echoed command |
| Args | `[]string` | `args` | Echoed arguments |
| StartedAt | `time.Time` | `startedAt` | Process start time |

**Notes**

Returns an error if `command` is empty. The process is managed by the `process.Service`; its output is captured and available via `process_output`.

---

### `process_stop`

Gracefully stop a running process by ID.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| ID | `string` | `id` | Yes | Process ID returned by `process_start` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Echoed process ID |
| Success | `bool` | `success` | Whether the stop signal was sent |
| Message | `string` | `message` | Status message |

**Notes**

The current implementation sends `SIGKILL`. A SIGTERM-first approach may be introduced in a future version. If the process does not respond, use `process_kill`.

---

### `process_kill`

Force kill a process by ID.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| ID | `string` | `id` | Yes | Process ID returned by `process_start` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Echoed process ID |
| Success | `bool` | `success` | Whether the kill succeeded |
| Message | `string` | `message` | Status message |

**Notes**

Sends an unconditional kill signal via `process.Service.Kill()`. Use when `process_stop` does not succeed within an acceptable time.

---

### `process_list`

List all managed processes.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| RunningOnly | `bool` | `running_only` | No | Return only actively running processes; default `false` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Processes | `[]ProcessInfo` | `processes` | List of process records |
| Total | `int` | `total` | Number of records returned |

**ProcessInfo fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Process identifier |
| Command | `string` | `command` | Executable name |
| Args | `[]string` | `args` | Arguments |
| Dir | `string` | `dir` | Working directory |
| Status | `string` | `status` | Process status string |
| PID | `int` | `pid` | OS process ID |
| ExitCode | `int` | `exitCode` | Exit code (meaningful only after termination) |
| StartedAt | `time.Time` | `startedAt` | Process start time |
| Duration | `time.Duration` | `duration` | Elapsed or total runtime |

---

### `process_output`

Get the captured stdout/stderr output of a process by ID.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| ID | `string` | `id` | Yes | Process ID |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Echoed process ID |
| Output | `string` | `output` | Captured output as a single string |

**Notes**

Output is buffered by `process.Service`. For streaming output in real time, connect a WebSocket subscriber via `ws_start` and subscribe to the process channel.

---

### `process_input`

Send text to the stdin of a running process.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| ID | `string` | `id` | Yes | Process ID |
| Input | `string` | `input` | Yes | Text to write to stdin |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Echoed process ID |
| Success | `bool` | `success` | Whether the input was delivered |
| Message | `string` | `message` | Status message |

**Notes**

Returns an error if the process is not found or if `input` is empty. Does not append a newline automatically; include `"\n"` in `input` if required.

---

## WebSocket

Two tools for starting and inspecting the WebSocket hub used for real-time process output streaming. These tools are only registered when a `ws.Hub` instance is provided via `WithWSHub()` at construction time.

The WebSocket endpoint is served at `/ws` on the configured address.

---

### `ws_start`

Start the WebSocket server for real-time process output streaming.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Addr | `string` | `addr` | No | Listen address in `host:port` format; default `":8080"` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the server started (or was already running) |
| Addr | `string` | `addr` | Actual address the server is bound to |
| Message | `string` | `message` | Status message including the full WebSocket URL |

**Notes**

If the server is already running, returns the existing address without error. The WebSocket URL takes the form `ws://<addr>/ws`. Logged at `Security` level because it opens a network listener.

---

### `ws_info`

Get WebSocket hub statistics.

**Parameters**

None.

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Clients | `int` | `clients` | Number of currently connected WebSocket clients |
| Channels | `int` | `channels` | Number of active subscription channels |

---

## Browser Automation

Ten tools for controlling a Chrome browser via the Chrome DevTools Protocol (CDP). All browser tools share a single global `webview.Webview` instance; `webview_connect` must be called before any other browser tool in a session.

**Prerequisite**: start Chrome with remote debugging enabled:

```
google-chrome --remote-debugging-port=9222
```

`webview_connect`, `webview_eval`, and `webview_navigate` are logged at `Security` level due to their capability to exfiltrate data or execute arbitrary code.

---

### `webview_connect`

Connect to a running Chrome instance via the Chrome DevTools Protocol.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| DebugURL | `string` | `debug_url` | Yes | Chrome DevTools URL, e.g. `http://localhost:9222` |
| Timeout | `int` | `timeout` | No | Default operation timeout in seconds; default `30` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the connection was established |
| Message | `string` | `message` | Status message |

**Notes**

Closes any existing connection before establishing a new one. Returns an error if `debug_url` is empty or if Chrome is not reachable at the given address.

---

### `webview_disconnect`

Disconnect from Chrome DevTools and release the connection.

**Parameters**

None.

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the disconnection succeeded |
| Message | `string` | `message` | Status message |

**Notes**

Returns `success=true` with message `"No active connection"` if no connection was open. Safe to call multiple times.

---

### `webview_navigate`

Navigate the browser to a URL.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| URL | `string` | `url` | Yes | Fully qualified URL to navigate to |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether navigation was initiated |
| URL | `string` | `url` | Echoed URL |

**Notes**

Returns an error if not connected. Navigation is initiated but this tool does not wait for the page to finish loading; use `webview_wait` to synchronise on a page element after navigation.

---

### `webview_click`

Click on an element identified by a CSS selector.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Selector | `string` | `selector` | Yes | CSS selector for the target element |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the click was performed |

**Notes**

Returns an error if not connected or if `selector` is empty. The selector must match at least one element; the first match is clicked.

---

### `webview_type`

Type text into an element identified by a CSS selector.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Selector | `string` | `selector` | Yes | CSS selector for the target element |
| Text | `string` | `text` | Yes | Text to type into the element |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the text was typed |

**Notes**

Returns an error if not connected or if `selector` is empty. Appends to any existing content in the element rather than replacing it; clear the field first if a fresh value is required.

---

### `webview_query`

Query DOM elements by CSS selector.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Selector | `string` | `selector` | Yes | CSS selector |
| All | `bool` | `all` | No | Return all matching elements; default returns first match only |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Found | `bool` | `found` | Whether any elements matched |
| Count | `int` | `count` | Number of elements matched |
| Elements | `[]WebviewElementInfo` | `elements` | Element details |

**WebviewElementInfo fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| NodeID | `int` | `nodeId` | CDP node identifier |
| TagName | `string` | `tagName` | HTML tag name |
| Attributes | `map[string]string` | `attributes` | Element attributes |
| BoundingBox | `*webview.BoundingBox` | `boundingBox` | Element position and dimensions |

**Notes**

With `all=false` (default), returns at most one element and `found=false` when no match exists. With `all=true`, returns all matches. A zero-result `all=true` query returns `found=false` and an empty slice rather than an error.

---

### `webview_console`

Get browser console messages captured since the last call.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Clear | `bool` | `clear` | No | Clear the console buffer after reading; default `false` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Messages | `[]WebviewConsoleMessage` | `messages` | Captured console messages |
| Count | `int` | `count` | Number of messages |

**WebviewConsoleMessage fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Type | `string` | `type` | Message level: `log`, `warn`, `error`, etc. |
| Text | `string` | `text` | Message content |
| Timestamp | `string` | `timestamp` | RFC 3339 timestamp |
| URL | `string` | `url` | Source URL (if available) |
| Line | `int` | `line` | Source line number (if available) |

**Notes**

Returns an error if not connected. Messages accumulate in a buffer; use `clear=true` to reset the buffer after reading to avoid receiving duplicate messages on subsequent calls.

---

### `webview_eval`

Evaluate a JavaScript expression in the browser context.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Script | `string` | `script` | Yes | JavaScript expression or statement to evaluate |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether evaluation succeeded |
| Result | `any` | `result` | Return value of the expression (JSON-serialisable) |
| Error | `string` | `error` | Error message if evaluation failed |

**Notes**

Returns an error if not connected or if `script` is empty. JavaScript errors are captured in the `error` field with `success=false` rather than being returned as tool-level errors. Logged at `Security` level.

---

### `webview_screenshot`

Capture a screenshot of the current browser viewport.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Format | `string` | `format` | No | Image format: `"png"` or `"jpeg"`; default `"png"` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the screenshot was captured |
| Data | `string` | `data` | Base64-encoded image data |
| Format | `string` | `format` | Format used |

**Notes**

Returns an error if not connected. The `data` field is standard base64 (not URL-safe); decode with standard base64 to obtain the raw image bytes.

---

### `webview_wait`

Wait for an element matching a CSS selector to appear in the DOM.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Selector | `string` | `selector` | Yes | CSS selector to wait for |
| Timeout | `int` | `timeout` | No | Maximum wait time in seconds |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Success | `bool` | `success` | Whether the element appeared within the timeout |
| Message | `string` | `message` | Status message including the matched selector |

**Notes**

Returns an error if not connected or if `selector` is empty. If the element does not appear before the timeout, an error is returned. The default timeout is inherited from the connection timeout set during `webview_connect`.

---

## IDE Chat

Five tools for interacting with agent chat sessions via the Laravel WebSocket bridge. These tools are provided by the `ide.Subsystem` and require a `ws.Hub` instance for real-time forwarding.

**Implementation status**: The Laravel backend integration is currently a stub. Tool calls emit bridge messages and return placeholder responses. Live data is delivered asynchronously via WebSocket subscriptions.

---

### `ide_chat_send`

Send a message to an agent chat session.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| SessionID | `string` | `sessionId` | Yes | Target session identifier |
| Message | `string` | `message` | Yes | Message content to send |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Sent | `bool` | `sent` | Whether the message was forwarded to the bridge |
| SessionID | `string` | `sessionId` | Echoed session ID |
| Timestamp | `time.Time` | `timestamp` | Server-assigned send timestamp |

**Notes**

The bridge forwards the message on the `chat:<sessionId>` channel. The agent's reply arrives asynchronously via the WebSocket connection. Returns an error if the bridge is not available.

---

### `ide_chat_history`

Retrieve message history for a chat session.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| SessionID | `string` | `sessionId` | Yes | Session identifier |
| Limit | `int` | `limit` | No | Maximum number of messages to return |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| SessionID | `string` | `sessionId` | Echoed session ID |
| Messages | `[]ChatMessage` | `messages` | Message list (currently empty; data arrives via WebSocket) |

**ChatMessage fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Role | `string` | `role` | `"user"` or `"assistant"` |
| Content | `string` | `content` | Message text |
| Timestamp | `time.Time` | `timestamp` | Message timestamp |

**Notes**

Currently returns an empty `messages` array. The history request is forwarded to the Laravel backend via the bridge; data is returned asynchronously.

---

### `ide_session_list`

List active agent sessions.

**Parameters**

None.

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Sessions | `[]Session` | `sessions` | Active sessions (currently empty; data arrives via WebSocket) |

**Session fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Session identifier |
| Name | `string` | `name` | Human-readable session name |
| Status | `string` | `status` | Session status string |
| CreatedAt | `time.Time` | `createdAt` | Session creation time |

---

### `ide_session_create`

Create a new agent session.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Name | `string` | `name` | Yes | Human-readable name for the new session |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Session | `Session` | `session` | The new session record |

**Notes**

Currently returns a placeholder session with `status="creating"` and no `id`. The actual session is created asynchronously by the Laravel backend; the final record arrives via WebSocket.

---

### `ide_plan_status`

Get the current plan status for an agent session.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| SessionID | `string` | `sessionId` | Yes | Session identifier |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| SessionID | `string` | `sessionId` | Echoed session ID |
| Status | `string` | `status` | Plan status; currently `"unknown"` |
| Steps | `[]PlanStep` | `steps` | Plan steps (currently empty) |

**PlanStep fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Name | `string` | `name` | Step name |
| Status | `string` | `status` | Step status |

---

## IDE Build

Three tools for querying CI build status and logs. Provided by `ide.Subsystem`; data is sourced from the Laravel backend via the WebSocket bridge.

**Implementation status**: Stub. Tool calls emit bridge messages and return placeholder responses.

---

### `ide_build_status`

Get the status of a specific build.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| BuildID | `string` | `buildId` | Yes | Build identifier |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Build | `BuildInfo` | `build` | Build record |

**BuildInfo fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| ID | `string` | `id` | Build identifier |
| Repo | `string` | `repo` | Repository name |
| Branch | `string` | `branch` | Git branch |
| Status | `string` | `status` | Build status; currently `"unknown"` |
| Duration | `string` | `duration` | Build duration (if complete) |
| StartedAt | `time.Time` | `startedAt` | Build start time |

---

### `ide_build_list`

List recent builds, optionally filtered by repository.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Repo | `string` | `repo` | No | Filter by repository name |
| Limit | `int` | `limit` | No | Maximum number of builds to return |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Builds | `[]BuildInfo` | `builds` | Build records (currently empty) |

---

### `ide_build_logs`

Retrieve log output for a build.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| BuildID | `string` | `buildId` | Yes | Build identifier |
| Tail | `int` | `tail` | No | Return only the last N lines |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| BuildID | `string` | `buildId` | Echoed build identifier |
| Lines | `[]string` | `lines` | Log lines (currently empty) |

---

## IDE Dashboard

Three tools for querying platform-wide overview, activity, and aggregate metrics. Provided by `ide.Subsystem`; data is sourced from the Laravel backend via the WebSocket bridge.

**Implementation status**: `ide_dashboard_overview` returns live bridge connectivity status. The other two tools are stubs that forward requests and return placeholder responses.

---

### `ide_dashboard_overview`

Get a high-level overview of the platform.

**Parameters**

None.

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Overview | `DashboardOverview` | `overview` | Platform summary |

**DashboardOverview fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Repos | `int` | `repos` | Number of repositories (stub; currently `0`) |
| Services | `int` | `services` | Number of services (stub; currently `0`) |
| ActiveSessions | `int` | `activeSessions` | Active agent sessions (stub; currently `0`) |
| RecentBuilds | `int` | `recentBuilds` | Recent build count (stub; currently `0`) |
| BridgeOnline | `bool` | `bridgeOnline` | Whether the Laravel WebSocket bridge is currently connected |

**Notes**

`BridgeOnline` is the only live field. All other fields return `0` until the Laravel backend integration is complete.

---

### `ide_dashboard_activity`

Get the recent activity feed.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Limit | `int` | `limit` | No | Maximum number of events to return |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Events | `[]ActivityEvent` | `events` | Activity events (currently empty) |

**ActivityEvent fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Type | `string` | `type` | Event category |
| Message | `string` | `message` | Human-readable description |
| Timestamp | `time.Time` | `timestamp` | Event timestamp |

---

### `ide_dashboard_metrics`

Get aggregate build and agent metrics for a time period.

**Parameters**

| Field | Type | JSON key | Required | Description |
|---|---|---|---|---|
| Period | `string` | `period` | No | Time period: `"1h"`, `"24h"`, or `"7d"`; default `"24h"` |

**Response**

| Field | Type | JSON key | Description |
|---|---|---|---|
| Period | `string` | `period` | Echoed period |
| Metrics | `DashboardMetrics` | `metrics` | Aggregate metrics |

**DashboardMetrics fields**

| Field | Type | JSON key | Description |
|---|---|---|---|
| BuildsTotal | `int` | `buildsTotal` | Total builds in period |
| BuildsSuccess | `int` | `buildsSuccess` | Successful builds |
| BuildsFailed | `int` | `buildsFailed` | Failed builds |
| AvgBuildTime | `string` | `avgBuildTime` | Average build duration |
| AgentSessions | `int` | `agentSessions` | Agent sessions created |
| MessagesTotal | `int` | `messagesTotal` | Total chat messages |
| SuccessRate | `float64` | `successRate` | Build success ratio (0.0–1.0) |

**Notes**

All fields currently return zero values. Data will be populated once the Laravel backend integration is complete.
