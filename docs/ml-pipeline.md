---
title: ML Pipeline
description: ML scoring, model management, and inference backend integration.
---

# ML Pipeline

The ML pipeline in go-ai connects the MCP server to the scoring and inference capabilities provided by `go-ml` and `go-inference`. Five MCP tools expose generation, scoring, capability probes, and backend discovery.

## Architecture

```
MCP Client
    |  tools/call ml_generate
    v
MLSubsystem (go-ai/mcp/tools_ml.go)
    |
    +-- ml.Service (go-ml)
    |       +-- InferenceAdapter --> inference.TextModel (go-inference)
    |       +-- ScoringEngine (heuristic scores)
    |       +-- JudgeBackend (LLM-as-judge)
    |
    +-- inference.List() / inference.Get() / inference.Default()
            +-- go-mlx (Metal GPU, macOS)
            +-- go-rocm (AMD ROCm, Linux)
            +-- Ollama (HTTP subprocess)
```

## ML Tools

### `ml_generate`

Generate text using the active inference backend.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` | `string` | Yes | The text prompt |
| `model` | `string` | No | Model name (uses default if omitted) |
| `max_tokens` | `int` | No | Maximum tokens to generate |
| `temperature` | `float64` | No | Sampling temperature |

Returns the generated text and metadata about which backend and model were used.

### `ml_score`

Score content using the heuristic scoring engine. Supports three scoring modes:

- **Heuristic** -- Pattern-based scoring across multiple dimensions (emotional register, sycophancy detection, vocabulary diversity, etc.)
- **Semantic** -- LLM-as-judge evaluation using a secondary model
- **Content** -- Combined scoring pipeline

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | `string` | Yes | Text to score |
| `mode` | `string` | No | `heuristic`, `semantic`, or `content` |

Returns dimension-level scores and an aggregate verdict.

### `ml_probe`

Run capability probes against the active model. Probes test specific model capabilities (instruction following, reasoning, factual recall, etc.). There are 23 built-in probes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `probe` | `string` | No | Specific probe name (runs all if omitted) |

### `ml_status`

Query the ML pipeline status, including active backends, loaded models, and InfluxDB pipeline health.

### `ml_backends`

List all registered inference backends and their availability status. Calls `inference.List()`, `inference.Get()`, and `inference.Default()` from the `go-inference` registry.

Returns an array of backends with their names, availability flags, and which is currently the default.

## Inference Backend Registry

The `go-inference` module provides a global registry for inference backends. Backends are registered at startup and can be queried at runtime:

```go
// Querying available backends (used by ml_backends tool)
backends := inference.List()     // All registered backends
backend := inference.Get("mlx")  // Specific backend by name
def := inference.Default()       // Currently active default
```

### Supported Backends

| Backend | Module | Platform | Description |
|---------|--------|----------|-------------|
| MLX | `go-mlx` | macOS (Apple Silicon) | Native Metal GPU inference |
| ROCm | `go-rocm` | Linux (AMD GPU) | AMD ROCm GPU inference via llama-server |
| Ollama | `go-ml` | Any | HTTP-based inference via Ollama subprocess |

## Scoring Engine

The scoring engine in `go-ml` provides heuristic analysis across multiple dimensions. Each dimension produces a normalised score (0.0 to 1.0) and a qualitative verdict.

Scoring dimensions include:
- Emotional register (positive and negative pattern detection)
- Sycophancy detection
- Vocabulary diversity
- Sentence complexity
- Repetition analysis
- Format adherence

The `ml_score` tool delegates directly to `go-ml`'s `ml.Service` rather than routing through `go-inference`, since the scoring engine is specific to go-ml and not an abstract backend capability.

## Integration with the MCP Server

The ML subsystem is registered as a plugin during MCP server construction:

```go
svc, err := mcp.New(
    mcp.WithSubsystem(mcp.NewMLSubsystem(mlSvc)),
)
```

`MLSubsystem` implements the `Subsystem` interface and registers all five ML tools when `RegisterTools` is called.

## Testing

ML tools can be tested with mock backends that satisfy the `ml.Backend` and `inference.Backend` interfaces:

```go
type mockMLBackend struct {
    name         string
    available    bool
    generateResp string
    generateErr  error
}

func (m *mockMLBackend) Name() string    { return m.name }
func (m *mockMLBackend) Available() bool { return m.available }
```

Register lightweight mocks for CI environments where GPU backends and model weights are not available:

```go
inference.Register(&mockInferenceBackend{name: "test-ci-mock", available: true})
```

Note that `inference.Register` is global state -- use unique names to avoid conflicts between parallel test runs.
