---
title: Agentic Client
description: Security scanning, metrics recording, and CLI command integrations.
---

# Agentic Client

go-ai provides several CLI command packages that integrate AI-adjacent functionality into the `core` binary. These cover security scanning across multi-repo environments, AI metrics viewing, and homelab monitoring.

## Security Scanning

The `cmd/security/` package provides a comprehensive security scanning suite that queries GitHub's security APIs across repositories defined in a `repos.yaml` registry.

### Command Structure

```
core security
  +-- alerts     # Unified view of all security alert types
  +-- deps       # Dependabot dependency vulnerability alerts
  +-- scan       # Code scanning alerts (CodeQL, etc.)
  +-- secrets    # Secret scanning alerts
  +-- jobs       # Create GitHub issues from scan findings
```

### Common Flags

| Flag | Description |
|------|-------------|
| `--registry` | Path to `repos.yaml` (auto-detected if omitted) |
| `--repo` | Filter to a single repository |
| `--severity` | Filter by severity: `critical`, `high`, `medium`, `low` (comma-separated) |
| `--json` | Output as JSON instead of formatted table |
| `--target` | External repo target (e.g. `wailsapp/wails`) -- bypasses registry |

### Alerts

The `alerts` command provides a unified view combining Dependabot, code scanning, and secret scanning alerts:

```bash
core security alerts                          # All repos in registry
core security alerts --repo core-php          # Single repo
core security alerts --severity critical,high # Filter by severity
core security alerts --target wailsapp/wails  # External repo
core security alerts --json                   # JSON output
```

### Dependencies

Query Dependabot vulnerability alerts with upgrade suggestions:

```bash
core security deps
core security deps --severity high
```

Output includes the vulnerable version range and the first patched version when available.

### Code Scanning

Query code scanning alerts from tools like CodeQL:

```bash
core security scan
core security scan --tool codeql    # Filter by scanning tool
```

Each scan execution records a metrics event via `ai.Record()` for tracking scan activity over time.

### Secret Scanning

Check for exposed secrets across repositories:

```bash
core security secrets
core security secrets --json
```

Secrets are always treated as high severity. The output flags whether push protection was bypassed.

### Jobs

Create GitHub issues from security findings for agent-driven remediation:

```bash
core security jobs --targets wailsapp/wails
core security jobs --targets wailsapp/wails --issue-repo host-uk/core
core security jobs --targets wailsapp/wails --dry-run
core security jobs --targets a/b,c/d --copies 3
```

| Flag | Default | Description |
|------|---------|-------------|
| `--targets` | (required) | Comma-separated `owner/repo` targets |
| `--issue-repo` | `host-uk/core` | Repository where issues are created |
| `--dry-run` | `false` | Preview without creating issues |
| `--copies` | `1` | Number of issue copies per target |

Each created issue includes a findings summary, checklist, and instructions. A metrics event is recorded for each issue created.

## Metrics

The `ai/` package provides a JSONL-based metrics recording system. Events are stored at:

```
~/.core/ai/metrics/YYYY-MM-DD.jsonl
```

### Event Structure

```go
type Event struct {
    Type      string         `json:"type"`
    Timestamp time.Time      `json:"timestamp"`
    AgentID   string         `json:"agent_id,omitempty"`
    Repo      string         `json:"repo,omitempty"`
    Duration  time.Duration  `json:"duration,omitempty"`
    Data      map[string]any `json:"data,omitempty"`
}
```

### Recording Events

```go
ai.Record(ai.Event{
    Type:      "security.scan",
    Timestamp: time.Now(),
    Repo:      "wailsapp/wails",
    Data: map[string]any{
        "total":    summary.Total,
        "critical": summary.Critical,
    },
})
```

Writing uses `O_APPEND` with a mutex for concurrent safety. Missing directories are created automatically.

### Reading and Querying

```go
events, err := ai.ReadEvents(since)  // Read events from a time range
summary := ai.Summary(events)        // Aggregate by type, repo, agent
```

`ReadEvents` iterates calendar days from `since` to today, opening each daily file. Missing files are silently skipped. Malformed JSONL lines are skipped without error.

`Summary` returns a `map[string]any` with:
- `total` -- total event count
- `by_type` -- sorted slice of `{key, count}` maps
- `by_repo` -- sorted slice of `{key, count}` maps
- `by_agent` -- sorted slice of `{key, count}` maps

### CLI Command

```bash
core ai metrics                  # Last 7 days (default)
core ai metrics --since 30d     # Last 30 days
core ai metrics --since 24h     # Last 24 hours
core ai metrics --json          # JSON output
```

Duration format: `Nd` (days), `Nh` (hours), `Nm` (minutes).

### MCP Tools

The metrics system is also exposed via two MCP tools:

| Tool | Description |
|------|-------------|
| `metrics_record` | Record an event to the JSONL store |
| `metrics_query` | Query and summarise events for a time period |

## Lab Dashboard

The `cmd/lab/` package provides a homelab monitoring dashboard with real-time data collection:

```bash
core lab serve                    # Start on :8080
core lab serve --bind :9090       # Custom port
```

### Collectors

The dashboard aggregates data from multiple sources:

| Collector | Interval | Source |
|-----------|----------|--------|
| System | 60s | Local machine stats |
| Prometheus | Configurable | Prometheus endpoint |
| HuggingFace | Configurable | HF model metadata |
| Docker | Configurable | Docker container status |
| Forgejo | Configurable | Forge CI/CD status |
| Training | Configurable | ML training run status |
| Services | 60s | Service health checks |
| InfluxDB | Configurable | Time-series metrics |

### Routes

**Web pages:** `/`, `/models`, `/training`, `/dataset`, `/agents`, `/services`

**JSON API:** `/api/status`, `/api/models`, `/api/training`, `/api/dataset`, `/api/runs`, `/api/agents`, `/api/services`

**Live updates:** `/events` (Server-Sent Events)

**Health:** `/health`

## RAG CLI

The `cmd/rag/` package re-exports `go-rag`'s CLI commands for use within the `core` binary:

```go
var AddRAGSubcommands = ragcmd.AddRAGSubcommands
```

This makes RAG operations (ingest, query, collection management) available as `core rag` subcommands without duplicating the implementation.
