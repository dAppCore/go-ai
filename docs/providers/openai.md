<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# providers/openai/openai.go — outbound OpenAI-compatible Backend

**Package**: `dappco.re/go/ai/providers/openai`
**File**: `go/providers/openai/openai.go`

## What this is

The **client-side** OpenAI-compatible provider. Mirror-image of `go-inference/openai/openai.go` (which is the **server-side** handler): both share the same wire DTOs from `dappco.re/go/inference/openai`, but this side **calls out** to an OpenAI-compatible endpoint and adapts the response into an `inference.TextModel`.

This is how go-ai routes a chat request through GPT-4 / Claude / a local Ollama-as-OpenAI / a future Lethean-distributed shard — anything that speaks the OpenAI wire on the other end becomes a `TextModel` here.

Once loaded, the resulting Model registers in `inference`'s global Backend registry — at which point it's indistinguishable to consumers from a native Metal model. That's the trick: external providers and local backends share the same downstream API.

## Config

```go
type Config struct {
    Name             string                  // route name, e.g. "openai" | "anthropic-bridge"
    BaseURL          string                  // https://api.openai.com/v1
    APIKey           string
    Organisation     string                  // OpenAI org id (optional)
    Project          string                  // OpenAI project id (optional)
    DefaultModel     string                  // model id when path = ""
    HTTPClient       *http.Client            // override for custom transport
    Limiter          Limiter                 // quota / rate gate
    ContextAssembler ContextAssembler        // pre-request context injection
    EstimateTokens   func(...) int           // for quota accounting
}
```

`Limiter` is an interface, not a concrete type — satisfied by `*ratelimit.RateLimiter` but doesn't force the import. Keeps this package free of policy.

## Backend

```go
type Backend struct { cfg Config }

var _ inference.Backend           = (*Backend)(nil)
var _ inference.CapabilityReporter = (*Backend)(nil)

func NewBackend(cfg Config) *Backend
func Register(cfg Config) *Backend  // construct + register globally
```

`Backend.Available()` returns true iff `BaseURL` and `DefaultModel` are set — outbound providers can't dial without both.

`Backend.LoadModel(path)` interprets `path` as the model id (not a filesystem path), defaulting to `DefaultModel` when empty. Returns a `*Model` handle.

## Model

```go
type Model struct {
    backend *Backend
    modelID string
    client  *http.Client
    // …
}

var _ inference.TextModel          = (*Model)(nil)
var _ inference.CapabilityReporter = (*Model)(nil)
```

Implements:

- `Generate` — converts to single-turn Chat
- `Chat` — POSTs `/v1/chat/completions` (via the wire DTOs from `dappco.re/go/inference/openai`); yields one buffered token containing the full response text (no streaming on this path yet)
- `Classify` — N/A (returns error) — external providers don't expose prefill-only path
- `BatchGenerate` — sequential Chat for each prompt
- `Info` / `ModelType` — best-effort, populated from the response model field
- `Metrics` — populated from the response usage block
- `Err` — last error
- `Close` — no-op

## Capabilities reported

The backend reports a constrained capability set:

```go
SupportedCapability(CapabilityModelLoad, CapabilityGroupRuntime)
SupportedCapability(CapabilityGenerate,  CapabilityGroupModel)
SupportedCapability(CapabilityChat,      CapabilityGroupModel)
```

Notably **not** supported: classify, batch, agent memory, attention probe, training. External providers are inference-only; consumers branch via capability check.

`RuntimeIdentity.Backend = b.Name()`, `Device = "external"`, `NativeRuntime = false`, labels carry `provider: openai-compatible` + `base_url`.

## Why register as an inference.Backend

Three reasons not to keep this as a go-ai-only abstraction:

1. **One backend list everywhere.** `inference.List()` returns local + external uniformly. UI model pickers show both without special-casing.
2. **LoadModel is the entry.** `inference.LoadModel(modelID, WithBackend("openai-compatible"))` works because the backend is registered. No "use this other API for external".
3. **CapabilityReport is uniform.** Audit / monitoring consume one shape; external providers are differentiated by their reported caps, not by their type.

## Streaming

Today: buffered (one yield with full text). Streaming is the obvious next addition — the wire DTOs in `inference/openai/openai.go` already model `ChatCompletionChunk`, so the SSE parse path is small.

## Tokens / quota

`EstimateTokens` callback runs pre-request; `Limiter.WaitForCapacity` gates; `Limiter.RecordUsage` runs on response. This lets a higher-level rate-limiter live anywhere (in-memory, Redis, distributed) without this package knowing.

## Used by

- `ProviderRouter` routes (one of N fallback options)
- `cmd/lem-desktop/agent_runner.go` — external provider chain
- `ai/book_state_demo.go` — book-state teacher route when configured with an external endpoint
- Future: Cladius "use Vi locally, fall back to OpenAI for hard prompts"

## Related

- [../ai/provider_router.md](../ai/provider_router.md) — consumer that chains routes
- `../../../go-inference/docs/openai/openai.md` — server side using the same wire DTOs
- `../../../go-inference/docs/inference/inference.md` — Backend + TextModel contracts
- `../../../go-inference/docs/inference/capability.md` — CapabilityReport shape
