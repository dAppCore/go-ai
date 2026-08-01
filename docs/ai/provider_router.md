<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/provider_router.go — fallback router across multiple providers

**Package**: `dappco.re/go/ai/ai`
**File**: `go/ai/provider_router.go`

## What this is

The **fallback policy layer** for chat requests across multiple providers — local mlx, external OpenAI-compat, Anthropic-compat, etc. Each route is one `inference.TextModel` with a name + model id + labels; the router tries each in order until one succeeds.

The point: **provider policy lives in go-ai**, not in go-inference. The contract package treats every backend as equal; go-ai is where "use Vi first, fall back to GPT-4 if Vi errors, fall back to a heuristic if both fail" lives.

## ProviderRoute

```go
type ProviderRoute struct {
    Name    string              // "vi-local" | "openai" | "anthropic" | …
    ModelID string              // model id within that provider
    Model   inference.TextModel // the actual runtime
    Labels  map[string]string   // for filtering / audit
}
```

## ProviderRouter

```go
router, _ := ai.NewProviderRouter(
    ProviderRoute{Name: "vi-local",  ModelID: "vi-2b",     Model: localModel},
    ProviderRoute{Name: "openai",    ModelID: "gpt-4o",    Model: openaiBackend},
    ProviderRoute{Name: "anthropic", ModelID: "claude-3.5", Model: anthropicBackend},
).Value.(*ai.ProviderRouter)
```

## NewProviderRouterWithOptions

```go
router, _ := ai.NewProviderRouterWithOptions(
    ai.ProviderRouterOptions{
        ContextAssembler: ragContextAssembler,
        ContextRole:      "system",
        ContextPrefix:    "Relevant docs:\n",
    },
    routes...,
)
```

Options apply across all fallback attempts — same context assembler runs for every provider in the chain.

## ProviderChatRequest

```go
type ProviderChatRequest struct {
    Messages []inference.Message
    Prompt   string            // alt — single user turn

    MaxTokens   int
    Temperature float32
    TopK        int
    TopP        float32
    Options     []inference.GenerateOption  // raw escape hatch

    ContextAssembler ProviderContextAssembler  // per-request override
    ContextRole      string                    // default "system"
    ContextPrefix    string
    DisableContext   bool                      // skip context injection for this request

    Labels map[string]string
}
```

`Messages` wins over `Prompt`. The Prompt convenience builds a `[{Role: user, Content: prompt}]` message.

## ProviderContextAssembler

```go
type ProviderContextAssembler interface {
    AssembleContext(ctx, []Message) (string, error)
}
```

Injects retrieval / context-pack material before the chosen provider sees the prompt. Implementations:

- `BookStateContextAssembler` (in `book_state_demo.go`) — injects a BookState
- RAG assemblers (`ai/rag.go`) — fetch Qdrant docs
- caller-supplied `ProviderContextAssemblerFunc` — closure adapter

## Chat

```go
result := router.Chat(ctx, request)
if !result.OK { return result }
resp := result.Value.(ProviderChatResponse)
fmt.Println(resp.Text)               // the chosen provider's answer
fmt.Println(resp.Provider)           // which provider won
fmt.Println(resp.Attempts)           // per-provider attempt log
fmt.Println(resp.ContextInjected)    // did context get added?
fmt.Println(resp.ContextBytes)
```

## Fallback algorithm

```
for each route in order:
    if ctx cancelled: return cancelled
    try chatProvider(route, messages, options)
    if success: return response with attempts log
    record failure in attempts, continue

return "all providers failed: <last error>"
```

The first success wins. The full attempts log is always returned — even on success — so audit / scoring can see which providers were tried.

## ProviderAttempt + ProviderChatResponse

```go
type ProviderAttempt struct {
    Provider string  // route name
    ModelID  string
    OK       bool    // true for the winner, false for failed attempts before it
    Error    string  // empty when OK
}

type ProviderChatResponse struct {
    Text     string
    Provider string                       // winner name
    ModelID  string
    Metrics  inference.GenerateMetrics    // winner's metrics
    Attempts []ProviderAttempt            // chronological — winner is always last when OK=true
    Labels   map[string]string

    ContextInjected bool
    ContextBytes    int                   // size of injected context (UTF-8 bytes)
}
```

## Why audit-rich attempts

When you ship "local-first, cloud-fallback" you need to know:

- Did Vi handle this prompt locally? (`Attempts[0].Provider == "vi-local"` + OK)
- Or did Vi fail with what error? (`Attempts[0].Error`)
- Which provider eventually answered?
- Was retrieval context injected? Of what size?

The full attempts log gives this without scraping logs.

## Why not in go-inference

`go-inference` is the **contract**. Provider fallback is **policy**. Putting policy in the contract package would:

- Force every backend to know about other backends
- Couple the wire shapes to the routing strategy
- Make external providers (OpenAI, Anthropic) special-case in the contract

Keeping it in go-ai lets the policy layer evolve (add quota, add per-user routing, add ML-based routing) without touching any backend or any wire format.

## Used by

- `ai/book_state_demo.go` — teacher + student each get a `*ProviderRouter`
- `mcp/tools_external.go` — external MCP tools that route through providers
- `cmd/lem-desktop/agent_runner.go` — agent loop with fallback chain
- Future: CoreAgent inference pipeline (Vi-first, OSS-cloud-fallback)

## Related

- [book_state_demo.md](book_state_demo.md) — primary consumer
- [../providers/openai.md](../providers/openai.md) — outbound OpenAI provider as a route source
- [context.md](context.md) — context assembler base
- [rag.md](rag.md) — RAG context assembler
- `../../../go-inference/docs/inference/inference.md` — TextModel each route wraps
