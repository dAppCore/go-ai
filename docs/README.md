<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# go-ai — documentation index

**Module**: `dappco.re/go/ai`
**Role**: AI facade layer. Provider routing, BookState demos, RAG facade, event metrics, MCP server. The consumer-facing surface that sits **above** the contract package (`go-inference`) and **above** the runtimes (`go-mlx`, `providers/openai`).

## Tetrad position

```
                    ┌──────────────────────────────┐
                    │      dappco.re/go (core)     │
                    └──────────────┬───────────────┘
                                   │
                    ┌──────────────┴────────────────┐
                    │     go-inference  (contract)  │
                    └──┬─────────────┬──────────────┘
                       │             │ register via init()
              ┌────────┴───┐  ┌──────┴────────┐
              │  go-mlx    │  │  go-rocm /    │
              └─────┬──────┘  └───────────────┘
                    │ consumed by
        ┌───────────┴────────┬─────────────────┐
        │  go-ml             │   you are here →  go-ai
        │  scoring/agent     │   router/demos/MCP
        └────────────────────┘ └─────────────────┘
```

## Doc tree

```
docs/
├── README.md                  ← you are here
├── ai/                        ← the facade package
│   ├── README.md              — package overview
│   ├── ai.md                  — pkg doc + canonical example
│   ├── provider_router.md     — fallback router (the load-bearing piece)
│   ├── book_state_demo.md     — teacher/student BookState demo
│   ├── book_state_demo_http.md — JSON HTTP handler
│   ├── context.md             — RAG context assembler
│   ├── rag.md                 — Qdrant facade
│   └── metrics.md             — append-only event log
│
├── providers/
│   └── openai.md              — outbound OpenAI-compatible Backend
│
├── mcp/                       ← MCP server
│   ├── README.md              — package overview
│   ├── service.md             — server core
│   ├── tools_core.md          — built-in file/dir/language tools
│   ├── tools_external.md      — Subsystem tool registration
│   ├── jsonrpc.md             — wire envelope
│   ├── transport_stdio.md     — Claude Code's default
│   ├── transport_tcp.md       — network transport
│   └── transport_unix.md      — same-machine multi-process
│
└── cmd/
    └── book-state-demo.md     — Go entry binary for the demo
```

## Where to start

- **"What's new this sprint?"** → [`ai/book_state_demo.md`](ai/book_state_demo.md) + [`cmd/book-state-demo.md`](cmd/book-state-demo.md)
- **"How does provider fallback work?"** → [`ai/provider_router.md`](ai/provider_router.md)
- **"How do I register an external OpenAI-compatible endpoint as a backend?"** → [`providers/openai.md`](providers/openai.md)
- **"What MCP tools does this server expose?"** → [`mcp/tools_core.md`](mcp/tools_core.md)
- **"How does Claude Code talk to this?"** → [`mcp/transport_stdio.md`](mcp/transport_stdio.md)
- **"How do agents inject RAG context before calling models?"** → [`ai/context.md`](ai/context.md) + [`ai/rag.md`](ai/rag.md)

## What's in this module

Three top-level subpackages:

| Path | Purpose |
|------|---------|
| `ai/` | Provider routing, BookState demo, RAG facade, metrics, MCP context assemblers |
| `providers/openai/` | Outbound OpenAI-compatible Backend |
| `mcp/` | MCP server (file/dir/language tools, JSON-RPC over stdio/TCP/Unix) |
| `cmd/` | Entry binaries (book-state-demo, daemon, lem-desktop, security CLI, …) |
| `pkg/api/` | HTTP API surface |

## Legacy docs

The flat docs (`architecture.md`, `agentic.md`, `angular-testing.md`, `development.md`, `ide-bridge.md`, `index.md`, `mcp-server.md`, `ml-pipeline.md`, `rag.md`, `tools.md`, `history.md`, `RFC-CORE-008-AGENT-EXPERIENCE.md`) pre-date this per-file pass and reference the pre-BookState shape. Treat as historical; the per-file docs above are the current source.

## Standards

- UK English in code, comments, docs
- SPDX header: `// SPDX-Licence-Identifier: EUPL-1.2`
- Error wrapping via `core.E(scope, msg, cause)` — never `fmt.Errorf`
- Test triplets: `_Good` / `_Bad` / `_Ugly`
- Conventional commits scoped to `ai`, `providers`, `mcp`, `cmd`
- Co-Author: `Co-Authored-By: Virgil <virgil@lethean.io>`
