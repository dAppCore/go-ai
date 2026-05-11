<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/metrics.go — append-only JSONL event log

**Package**: `dappco.re/go/ai/ai`
**File**: `go/ai/metrics.go`

## What this is

The **AI event metrics layer** — thread-safe, append-only JSONL recording of AI-related events at `~/.core/ai/metrics/YYYY-MM-DD.jsonl`. Used as the audit substrate for what the AI surface did, when, and why.

Three public functions:

- `Record(Event)` — append an event
- `ReadEvents(since)` — read events since a cutoff timestamp
- `Summary([]Event)` — aggregate event counts by type / repo / source

## Event shape

```go
type Event struct {
    Time   time.Time         // when (UTC)
    Type   string            // event type (e.g. "security.scan", "rag.query", "router.fallback")
    Source string            // emitting subsystem ("cli", "agent", "mcp")
    Repo   string            // repo context if applicable
    Status string            // "ok" | "error" | …
    Detail string            // free-form
    Labels map[string]string // filtering tags
}
```

JSON-encoded one per line. The timestamp prefix makes file rotation per-day natural — separate file per UTC day, no rotation policy needed.

## Storage

`~/.core/ai/metrics/YYYY-MM-DD.jsonl`. Append-only — never rewritten. A crash mid-write produces a torn line at EOF; the reader catches torn lines and skips them.

`metricsWriteLock` (a `core.New().Lock("ai.metrics.write")`) is the cross-goroutine append guard. Cross-process append safety relies on POSIX `O_APPEND` semantics — usually fine on local filesystems, less so on networked ones.

## Record

```go
ai.Record(ai.Event{
    Type:   "router.fallback",
    Source: "agent",
    Detail: "vi-local failed, used openai",
    Labels: map[string]string{"reason": "timeout"},
})
```

Time defaults to now if zero. Errors are reported via the return value; callers in non-critical paths (like the router itself) typically ignore record errors.

## ReadEvents

```go
events, err := ai.ReadEvents(time.Now().Add(-24 * time.Hour))
```

Walks today's + recent days' files; filters by timestamp. The `recentEventLimit = 10` constant caps the in-memory recent window — older events stay on disk.

## Summary

```go
summary := ai.Summary(events)
// summary.ByType, summary.BySource, summary.ByRepo — counts
```

Aggregate counts for dashboarding / quick-scan.

## Used by

- `cmd/security/*` — records security scan results
- `cmd/metrics/cmd.go` — surface for the `core ai metrics` CLI
- `mcp/tools_core.go` — agent tool that records its actions
- `cmd/lem-desktop/agent_runner.go` — agent loop emits per-decision events

## Why JSONL not DuckDB / SQLite

Three reasons:

1. **Append-only fits.** AI events are write-mostly; no joins, no updates.
2. **Crash-tolerant.** A crashed write loses one record; SQLite would risk DB corruption.
3. **Easy to ingest.** Other tools (eval pipeline, audit, monitoring) just read JSONL — no schema migration.

The downside is no fast queries — `ReadEvents` is a full file scan. Fine for the volumes seen (~1000s of events / day), would not scale to 1M/day. At that scale, DuckDB ingest on top of the JSONL is the next step.

## Related

- `cmd/metrics/cmd.go` — CLI exposing this data
- [../mcp/tools_core.md](../mcp/tools_core.md) (planned) — MCP tools that record events
- `dappco.re/go/io` — file primitives
