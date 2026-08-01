<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# mcp/tools_external.go — external tool registry

**Package**: `dappco.re/go/ai/mcp`
**File**: `go/mcp/tools_external.go`

## What this is

The hook for **registering tools defined outside the mcp package**. Subsystems passed in `Options.Subsystems` call this surface during startup to advertise their own tools.

This is the extension point that lets `core/lab`, `core/agent`, `core/ide`, and future plugins all register their tools with one MCP server.

## Tool registration

```go
type Tool struct {
    Group       string
    Name        string
    Description string
    Handler     ToolHandler
    InputSchema map[string]any   // optional JSON Schema for inputs
}

func (s *Service) RegisterTool(t Tool) core.Result
func (s *Service) RegisterTools(tools []Tool) core.Result
```

A Subsystem's `RegisterTools(s *Service) core.Result` implementation calls these to mount its tools.

## Subsystem boundary

```go
// In a hypothetical core/lab Subsystem:
func (lab *LabSubsystem) RegisterTools(s *mcp.Service) core.Result {
    return s.RegisterTools([]mcp.Tool{
        {Group: "lab", Name: "lab_runs",  Description: "List recent lab runs", Handler: lab.listRuns},
        {Group: "lab", Name: "lab_run",   Description: "Show one lab run",     Handler: lab.showRun},
    })
}
```

The subsystem keeps its handlers, types, and dependencies in its own module. The mcp package only sees the Tool descriptor + a function pointer.

## Why a registry vs hard-coded

Three reasons:

1. **Loose coupling.** core/lab doesn't need to be a dependency of go-ai/mcp. The Subsystem interface is the only crossing.
2. **Composition.** A binary can pick which subsystems to include — minimal MCP server vs full-featured.
3. **Plugins.** Future third-party MCP tools can be Subsystems loaded at startup from config.

## Naming discipline

External tools must use the `<group>_<name>` pattern (per built-in convention):

- ✓ `lab_runs`, `agent_state`, `ide_buffers`
- ✗ `runs` (no group prefix — name collision risk)
- ✗ `LabRuns` (snake_case, not CamelCase)

The server doesn't enforce this hard, but clients filter by group prefix — non-conforming tools get harder to discover.

## Input schemas

`InputSchema map[string]any` carries the JSON Schema for tool inputs. MCP clients use this for parameter validation + UI generation (e.g. Claude Code renders forms from schemas).

The built-in tools use schemas inferred from their input structs; external tools either provide their own or skip (and lose the auto-form benefit).

## Used by

- `core/lab` (planned) — lab dashboard tools
- `core/agent` (planned) — agent introspection
- `core/ide` (planned, partial) — IDE-specific tools

## Related

- [service.md](service.md) — Subsystem interface lives there
- [tools_core.md](tools_core.md) — same registration mechanism for built-ins
