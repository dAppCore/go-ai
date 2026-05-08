// SPDX-License-Identifier: EUPL-1.2

package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	core "dappco.re/go"
	"dappco.re/go/inference"
	openaicompat "dappco.re/go/inference/openai"
	"dappco.re/go/ratelimit"
)

func TestOpenAI_Chat_Good_PostsRequestAndRecordsUsage(t *testing.T) {
	var waited atomic.Bool
	var recorded atomic.Bool

	limiter, err := ratelimit.NewWithConfig(ratelimit.Config{
		FilePath:  core.JoinPath(t.TempDir(), "ratelimits.yaml"),
		Providers: []ratelimit.Provider{ratelimit.ProviderOpenAI},
	})
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !waited.Load() {
			t.Fatal("provider called HTTP before waiting for rate-limit capacity")
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != openaicompat.DefaultChatCompletionsPath {
			t.Fatalf("path = %s, want %s", r.URL.Path, openaicompat.DefaultChatCompletionsPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}

		req, err := openaicompat.DecodeRequest(r.Body)
		if err != nil {
			t.Fatalf("DecodeRequest() error = %v", err)
		}
		if req.Model != "gpt-test" {
			t.Fatalf("model = %q, want gpt-test", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "hello" {
			t.Fatalf("messages = %+v, want single user prompt", req.Messages)
		}
		if req.MaxTokens == nil || *req.MaxTokens != 8 {
			t.Fatalf("max_tokens = %v, want 8", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(core.JSONMarshalString(openaicompat.ChatCompletionResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-test",
			Choices: []openaicompat.ChatChoice{{
				Index:        0,
				Message:      openaicompat.ChatMessage{Role: "assistant", Content: "hello back"},
				FinishReason: "stop",
			}},
			Usage: openaicompat.ChatUsage{
				PromptTokens:     5,
				CompletionTokens: 2,
				TotalTokens:      7,
			},
		})))
	}))
	defer server.Close()

	backend := NewBackend(Config{
		Name:         "openai-test",
		BaseURL:      server.URL,
		APIKey:       "sk-test",
		DefaultModel: "gpt-test",
		HTTPClient:   server.Client(),
		Limiter: waitRecordLimiter{
			inner:    limiter,
			waited:   &waited,
			recorded: &recorded,
		},
	})

	model, err := backend.LoadModel("", inference.WithBackend("ignored"))
	if err != nil {
		t.Fatalf("LoadModel() error = %v", err)
	}
	defer model.Close()

	var got string
	for token := range model.Chat(context.Background(), []inference.Message{{Role: "user", Content: "hello"}}, inference.WithMaxTokens(8)) {
		got += token.Text
	}
	if err := model.Err(); err != nil {
		t.Fatalf("Chat() Err() = %v", err)
	}
	if got != "hello back" {
		t.Fatalf("Chat() = %q, want hello back", got)
	}
	if !recorded.Load() {
		t.Fatal("provider did not record usage after successful response")
	}
	metrics := model.Metrics()
	if metrics.PromptTokens != 5 || metrics.GeneratedTokens != 2 {
		t.Fatalf("Metrics() = %+v, want prompt=5 generated=2", metrics)
	}
}

func TestOpenAI_Chat_Good_PrependsContextAssemblerOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := openaicompat.DecodeRequest(r.Body)
		if err != nil {
			t.Fatalf("DecodeRequest() error = %v", err)
		}
		if len(req.Messages) != 2 {
			t.Fatalf("messages len = %d, want context + user", len(req.Messages))
		}
		if req.Messages[0].Role != "system" || !core.Contains(req.Messages[0].Content, "retrieved context") {
			t.Fatalf("context message = %+v, want system context", req.Messages[0])
		}
		_, _ = w.Write([]byte(core.JSONMarshalString(openaicompat.ChatCompletionResponse{
			Model: "gpt-test",
			Choices: []openaicompat.ChatChoice{{
				Message: openaicompat.ChatMessage{Role: "assistant", Content: "context answer"},
			}},
		})))
	}))
	defer server.Close()

	backend := NewBackend(Config{
		Name:         "openai-test",
		BaseURL:      server.URL,
		DefaultModel: "gpt-test",
		HTTPClient:   server.Client(),
		ContextAssembler: ContextAssemblerFunc(func(context.Context, []inference.Message) (string, error) {
			return "retrieved context", nil
		}),
	})
	model, err := backend.LoadModel("")
	if err != nil {
		t.Fatalf("LoadModel() error = %v", err)
	}

	var got string
	for token := range model.Chat(context.Background(), []inference.Message{{Role: "user", Content: "question"}}) {
		got += token.Text
	}
	if err := model.Err(); err != nil {
		t.Fatalf("Chat() Err() = %v", err)
	}
	if got != "context answer" {
		t.Fatalf("Chat() = %q, want context answer", got)
	}
}

func TestOpenAI_Chat_Bad_ProviderErrorDoesNotRecordUsage(t *testing.T) {
	var recorded atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(core.JSONMarshalString(openaicompat.ErrorResponse{
			Error: openaicompat.ErrorObject{
				Message: "rate limited",
				Type:    "rate_limit_error",
				Code:    "rate_limit_error",
			},
		})))
	}))
	defer server.Close()

	backend := NewBackend(Config{
		Name:         "openai-test",
		BaseURL:      server.URL,
		DefaultModel: "gpt-test",
		HTTPClient:   server.Client(),
		Limiter: waitRecordLimiter{
			recorded: &recorded,
		},
	})
	model, err := backend.LoadModel("")
	if err != nil {
		t.Fatalf("LoadModel() error = %v", err)
	}

	for range model.Generate(context.Background(), "hello") {
	}
	if model.Err() == nil {
		t.Fatal("Generate() Err() = nil, want provider error")
	}
	if recorded.Load() {
		t.Fatal("provider recorded usage for failed response")
	}
}

func TestOpenAI_Capabilities_Good_ReportProviderIdentity(t *testing.T) {
	backend := NewBackend(Config{
		Name:         "openai-test",
		BaseURL:      "https://api.example.test",
		DefaultModel: "gpt-test",
	})
	if backend.Name() != "openai-test" {
		t.Fatalf("Name() = %q, want openai-test", backend.Name())
	}
	if !backend.Available() {
		t.Fatal("Available() = false, want true for configured provider")
	}
	backendReport := backend.Capabilities()
	if !backendReport.Supports(inference.CapabilityGenerate) || !backendReport.Supports(inference.CapabilityChat) {
		t.Fatalf("Backend Capabilities() = %+v, want generate and chat", backendReport.Capabilities)
	}

	model, err := backend.LoadModel("")
	if err != nil {
		t.Fatalf("LoadModel() error = %v", err)
	}
	report := model.(inference.CapabilityReporter).Capabilities()
	if report.Runtime.Backend != "openai-test" {
		t.Fatalf("Runtime.Backend = %q, want openai-test", report.Runtime.Backend)
	}
	if report.Runtime.NativeRuntime {
		t.Fatal("Runtime.NativeRuntime = true, want external provider")
	}
	if report.Model.ID != "gpt-test" {
		t.Fatalf("Model.ID = %q, want gpt-test", report.Model.ID)
	}
	if !report.Supports(inference.CapabilityGenerate) || !report.Supports(inference.CapabilityChat) {
		t.Fatalf("Capabilities() = %+v, want generate and chat", report.Capabilities)
	}
}

func TestOpenAI_Register_Good_AddsBackendToInferenceRegistry(t *testing.T) {
	name := "openai-register-" + t.Name()
	backend := Register(Config{
		Name:         name,
		BaseURL:      "https://api.example.test",
		DefaultModel: "gpt-test",
	})
	if backend == nil {
		t.Fatal("Register() returned nil")
	}

	got, ok := inference.Get(name)
	if !ok {
		t.Fatalf("inference.Get(%q) not found", name)
	}
	if got != backend {
		t.Fatalf("inference.Get(%q) = %T, want registered backend", name, got)
	}
}

type waitRecordLimiter struct {
	inner interface {
		WaitForCapacity(context.Context, string, int) error
		RecordUsage(string, int, int)
	}
	waited   *atomic.Bool
	recorded *atomic.Bool
}

func (l waitRecordLimiter) WaitForCapacity(ctx context.Context, model string, tokens int) error {
	if l.waited != nil {
		l.waited.Store(true)
	}
	if l.inner != nil {
		return l.inner.WaitForCapacity(ctx, model, tokens)
	}
	return nil
}

func (l waitRecordLimiter) RecordUsage(model string, promptTokens, outputTokens int) {
	if l.recorded != nil {
		l.recorded.Store(true)
	}
	if l.inner != nil {
		l.inner.RecordUsage(model, promptTokens, outputTokens)
	}
}
