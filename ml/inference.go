// Package ml provides ML inference, scoring, and model management for CoreGo.
//
// It supports multiple inference backends (HTTP, llama-server, Ollama) through
// a common Backend interface, and includes an ethics-aware scoring engine with
// both heuristic and LLM-judge capabilities.
//
// Register as a CoreGo service:
//
//	core.New(
//	    core.WithService(ml.NewService),
//	)
package ml

import "context"

// Backend generates text from prompts. Implementations include HTTPBackend
// (OpenAI-compatible API), LlamaBackend (managed llama-server process), and
// OllamaBackend (Ollama native API).
type Backend interface {
	// Generate sends a single user prompt and returns the response.
	Generate(ctx context.Context, prompt string, opts GenOpts) (string, error)

	// Chat sends a multi-turn conversation and returns the response.
	Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error)

	// Name returns the backend identifier (e.g. "http", "llama", "ollama").
	Name() string

	// Available reports whether the backend is ready to accept requests.
	Available() bool
}

// GenOpts configures a generation request.
type GenOpts struct {
	Temperature float64
	MaxTokens   int
	Model       string // override model for this request
}

// Message is a single chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TokenCallback receives each generated token as text. Return a non-nil
// error to stop generation early (e.g. client disconnect).
type TokenCallback func(token string) error

// StreamingBackend extends Backend with token-by-token streaming.
// Backends that generate tokens incrementally (e.g. MLX) should implement
// this interface. The serve handler uses SSE when the client sends
// "stream": true and the active backend satisfies StreamingBackend.
type StreamingBackend interface {
	Backend

	// GenerateStream streams tokens from a single prompt via the callback.
	GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error

	// ChatStream streams tokens from a chat conversation via the callback.
	ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error
}

// DefaultGenOpts returns sensible defaults for generation.
func DefaultGenOpts() GenOpts {
	return GenOpts{
		Temperature: 0.1,
	}
}
