<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/tools_core.go — built-in file / dir / language tools

**Package**: `dappco.re/go/ai/mcp`
**File**: `go/mcp/tools_core.go`

## What this is

The **built-in tool registration** for the MCP server. Registers 9 tools at service startup covering file ops, directory ops, and language detection — the minimum surface a code-editing agent needs.

Implemented as method handlers on `*Service`, registered via `tool(group, name, desc, handler)`.

## Tool catalogue

### File tools

| Tool | Params | Returns |
|------|--------|---------|
| `file_read` | path | string content |
| `file_write` | path, content | success |
| `file_delete` | path | success (path must be file or empty dir) |
| `file_rename` | old_path, new_path | success |
| `file_exists` | path | bool |
| `file_edit` | path, old_string, new_string | replaced count |

All paths resolve relative to `WorkspaceRoot` (unless `Unrestricted` is set).

### Directory tools

| Tool | Params | Returns |
|------|--------|---------|
| `dir_list` | path | sorted entry list with sizes + types |
| `dir_create` | path | success (recursive) |

### Language tools

| Tool | Params | Returns |
|------|--------|---------|
| `lang_detect` | path | language id ("go", "python", "typescript", …) |

## typedHandler

```go
typedHandler(s.readFile)
```

The generic wrapper that:

1. Decodes JSON-RPC params into the handler's expected struct
2. Calls the strongly typed handler method
3. Encodes the result back to JSON-RPC

Avoids per-tool boilerplate; each method takes a typed input struct and returns a typed output.

## Security model

`file_*` and `dir_*` tools resolve paths via `resolvePath(path)` which:

1. Cleans the path
2. Resolves relative paths against `WorkspaceRoot`
3. Rejects absolute paths or `..` escapes (unless `Unrestricted`)

Without this, MCP clients could read/write arbitrary files. The cost is real — agents that need to touch global paths (homedir config, system tools) require explicit Unrestricted opt-in.

## file_edit semantics

```
file_edit(path, "OLD", "NEW")
```

- Replaces **all** occurrences (no flag for "first only" yet)
- Returns count of replacements
- Errors if file doesn't exist or path is outside workspace

Used by Claude Code's Edit tool path when it dials in via MCP.

## Tool naming convention

Group prefix + underscore + verb: `file_read`, `dir_list`, `lang_detect`. The group prefix lets the MCP client filter tools by category (e.g. "show me only file tools"). Matches the pattern downstream tools follow (e.g. `tasks_list`, `mantis_view` in core/ide).

## Used by

Any MCP client dialed into the server — most commonly:

- Claude Code (via stdio)
- core/ide chat panel (via Unix socket)
- Cladius's own session

## Related

- [service.md](service.md) — server core that registers these
- [tools_external.md](tools_external.md) — registry for non-built-in tools
- [jsonrpc.md](jsonrpc.md) — envelope shape
