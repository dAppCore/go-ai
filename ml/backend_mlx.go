//go:build darwin && arm64

package ml

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"

	"forge.lthn.ai/core/go-mlx"
	"forge.lthn.ai/core/go-mlx/cache"
	"forge.lthn.ai/core/go-mlx/model"
	"forge.lthn.ai/core/go-mlx/sample"
	"forge.lthn.ai/core/go-mlx/tokenizer"
)

// MLXBackend implements Backend and StreamingBackend for native Metal inference.
type MLXBackend struct {
	model      model.Model
	tok        *tokenizer.Tokenizer
	caches     []cache.Cache
	sampler    sample.Sampler
	mu         sync.Mutex
	modelBytes uint64 // model size at load time, for memory budget
}

// Compile-time check that MLXBackend satisfies StreamingBackend.
var _ StreamingBackend = (*MLXBackend)(nil)

// NewMLXBackend loads a model from a safetensors directory and creates
// a native Metal inference backend.
func NewMLXBackend(modelPath string) (*MLXBackend, error) {
	if !mlx.MetalAvailable() {
		return nil, fmt.Errorf("mlx: Metal GPU not available")
	}

	slog.Info("mlx: loading model", "path", modelPath)
	m, err := model.LoadModel(modelPath)
	if err != nil {
		return nil, fmt.Errorf("mlx: load model: %w", err)
	}

	// Cap Metal memory: cache limit for allocator reuse, memory limit as hard ceiling.
	mlx.SetCacheLimit(16 * 1024 * 1024 * 1024)  // 16 GB allocator cache
	mlx.SetMemoryLimit(24 * 1024 * 1024 * 1024)  // 24 GB hard cap

	modelMB := mlx.GetActiveMemory() / 1024 / 1024
	slog.Info("mlx: model loaded",
		"layers", m.NumLayers(),
		"memory_mb", modelMB,
	)

	return &MLXBackend{
		model:      m,
		tok:        m.Tokenizer(),
		caches:     m.NewCache(),
		sampler:    sample.New(0.1, 0, 0, 0),
		modelBytes: mlx.GetActiveMemory(),
	}, nil
}

// generate is the core token generation loop. If cb is non-nil, each token's
// text is sent to it (streaming mode). Returns the full output text.
func (b *MLXBackend) generate(ctx context.Context, tokens []int32, opts GenOpts, cb TokenCallback) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, c := range b.caches {
		c.Reset()
	}

	temp := float32(opts.Temperature)
	if temp == 0 {
		temp = 0.1
	}
	sampler := sample.New(temp, 0, 0, 0)

	input := mlx.FromValues(tokens, 1, len(tokens))

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	var output []int32
	firstToken := true
	for i := 0; i < maxTokens; i++ {
		select {
		case <-ctx.Done():
			runtime.GC()
			mlx.ClearCache()
			return b.tok.Decode(output), ctx.Err()
		default:
		}

		logits := b.model.Forward(input, b.caches)
		logits = lastPosition(logits)
		next := sampler.Sample(logits)
		mlx.Materialize(next)

		nextToken := int32(next.Int())
		if nextToken == b.tok.EOSToken() {
			break
		}
		output = append(output, nextToken)
		input = mlx.FromValues([]int32{nextToken}, 1, 1)

		// Stream the token text to the callback
		if cb != nil {
			tokenText := b.tok.DecodeToken(nextToken)
			// Strip the SentencePiece leading space only on the first token
			if firstToken {
				tokenText = strings.TrimLeft(tokenText, " ")
				firstToken = false
			}
			if err := cb(tokenText); err != nil {
				runtime.GC()
				mlx.ClearCache()
				return b.tok.Decode(output), err
			}
		}

		if i%4 == 3 {
			runtime.GC()
			mlx.ClearCache()
		}
	}

	runtime.GC()
	mlx.ClearCache()
	b.checkMemory()
	return b.tok.Decode(output), nil
}

// Generate produces text from a prompt using native Metal inference.
func (b *MLXBackend) Generate(ctx context.Context, prompt string, opts GenOpts) (string, error) {
	formatted := formatPrompt(b.model.ModelType(), prompt)
	tokens := b.tok.Encode(formatted)
	return b.generate(ctx, tokens, opts, nil)
}

// Chat formats messages and generates a response.
func (b *MLXBackend) Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error) {
	prompt := formatChat(b.model.ModelType(), messages)
	tokens := b.tok.Encode(prompt)
	return b.generate(ctx, tokens, opts, nil)
}

// GenerateStream streams tokens from a single prompt via the callback.
func (b *MLXBackend) GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error {
	formatted := formatPrompt(b.model.ModelType(), prompt)
	tokens := b.tok.Encode(formatted)
	_, err := b.generate(ctx, tokens, opts, cb)
	return err
}

// ChatStream streams tokens from a chat conversation via the callback.
func (b *MLXBackend) ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error {
	prompt := formatChat(b.model.ModelType(), messages)
	tokens := b.tok.Encode(prompt)
	_, err := b.generate(ctx, tokens, opts, cb)
	return err
}

// lastPosition extracts the last sequence position from [B, L, V] logits → [B, V].
func lastPosition(logits *mlx.Array) *mlx.Array {
	shape := logits.Shape()
	if len(shape) == 3 && shape[1] > 1 {
		L := shape[1]
		logits = mlx.Slice(logits, []int32{0, L - 1, 0}, []int32{shape[0], L, shape[2]})
		logits = mlx.Reshape(logits, shape[0], shape[2])
	} else if len(shape) == 3 && shape[1] == 1 {
		logits = mlx.Reshape(logits, shape[0], shape[2])
	}
	return logits
}

// checkMemory logs Metal memory usage and forces cleanup if it exceeds budget.
func (b *MLXBackend) checkMemory() {
	active := mlx.GetActiveMemory()
	budget := b.modelBytes * 3
	if active > budget {
		slog.Warn("mlx: memory over budget, forcing cleanup",
			"active_mb", active/1024/1024,
			"model_mb", b.modelBytes/1024/1024,
			"peak_mb", mlx.GetPeakMemory()/1024/1024,
		)
		runtime.GC()
		runtime.GC()
		mlx.ClearCache()
	}
}

// Name returns the backend identifier.
func (b *MLXBackend) Name() string { return "mlx" }

// Available reports whether Metal GPU is ready.
func (b *MLXBackend) Available() bool { return mlx.MetalAvailable() }

// formatPrompt wraps a raw prompt in the model's chat template for single-turn generation.
func formatPrompt(modelType, prompt string) string {
	switch modelType {
	case "qwen3":
		return fmt.Sprintf("<|im_start|>user\n%s<|im_end|>\n<|im_start|>assistant\n", prompt)
	default:
		return tokenizer.FormatGemmaPrompt(prompt)
	}
}

// formatChat builds a multi-turn chat prompt from messages using the model's template.
func formatChat(modelType string, messages []Message) string {
	switch modelType {
	case "qwen3":
		return formatQwen3Chat(messages)
	default:
		return formatGemmaChat(messages)
	}
}

func formatGemmaChat(messages []Message) string {
	var prompt string
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			prompt += fmt.Sprintf("<start_of_turn>user\n%s<end_of_turn>\n", msg.Content)
		case "assistant":
			prompt += fmt.Sprintf("<start_of_turn>model\n%s<end_of_turn>\n", msg.Content)
		case "system":
			prompt += fmt.Sprintf("<start_of_turn>user\n[System: %s]<end_of_turn>\n", msg.Content)
		}
	}
	prompt += "<start_of_turn>model\n"
	return prompt
}

func formatQwen3Chat(messages []Message) string {
	var prompt string
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			prompt += fmt.Sprintf("<|im_start|>system\n%s<|im_end|>\n", msg.Content)
		case "user":
			prompt += fmt.Sprintf("<|im_start|>user\n%s<|im_end|>\n", msg.Content)
		case "assistant":
			prompt += fmt.Sprintf("<|im_start|>assistant\n%s<|im_end|>\n", msg.Content)
		}
	}
	prompt += "<|im_start|>assistant\n"
	return prompt
}
