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
		ContextAssembler: ContextAssemblerFunc(func(context.Context, []inference.Message) core.Result {
			return core.Ok("retrieved context")
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

func TestOpenai_ContextAssemblerFunc_AssembleContext_Good(t *testing.T) {
	assembler := ContextAssemblerFunc(func(context.Context, []inference.Message) core.Result {
		return core.Ok("retrieved context")
	})
	result := assembler.AssembleContext(context.Background(), nil)

	if !result.OK || result.Value.(string) != "retrieved context" {
		t.Fatalf("ContextAssemblerFunc.AssembleContext() = %#v, want context text", result)
	}
}

func TestOpenai_ContextAssemblerFunc_AssembleContext_Bad(t *testing.T) {
	var assembler ContextAssemblerFunc
	result := assembler.AssembleContext(context.Background(), nil)

	if !result.OK || result.Value.(string) != "" {
		t.Fatalf("ContextAssemblerFunc.AssembleContext() = %#v, want empty context", result)
	}
}

func TestOpenai_ContextAssemblerFunc_AssembleContext_Ugly(t *testing.T) {
	assembler := ContextAssemblerFunc(func(context.Context, []inference.Message) core.Result {
		return core.Fail(core.E("test.assembler", "failed", nil))
	})
	result := assembler.AssembleContext(context.Background(), nil)

	if result.OK || !core.Contains(result.Error(), "failed") {
		t.Fatalf("ContextAssemblerFunc.AssembleContext() = %#v, want failure", result)
	}
}

func TestOpenai_NewBackend_Good(t *testing.T) {
	backend := NewBackend(Config{Name: "provider", BaseURL: "https://api.example.test/", DefaultModel: "gpt"})

	if backend == nil || backend.Name() != "provider" {
		t.Fatalf("NewBackend() = %#v, want named backend", backend)
	}
	if backend.cfg.BaseURL != "https://api.example.test" {
		t.Fatalf("NewBackend() BaseURL = %q, want trimmed URL", backend.cfg.BaseURL)
	}
}

func TestOpenai_NewBackend_Bad(t *testing.T) {
	backend := NewBackend(Config{})

	if backend == nil || backend.Name() != defaultProviderName {
		t.Fatalf("NewBackend() = %#v, want default provider name", backend)
	}
	if backend.Available() {
		t.Fatal("NewBackend() Available() = true, want unavailable without URL/model")
	}
}

func TestOpenai_NewBackend_Ugly(t *testing.T) {
	backend := NewBackend(Config{Name: "  ", BaseURL: "https://api.example.test///", DefaultModel: "gpt"})

	if backend.Name() != defaultProviderName {
		t.Fatalf("NewBackend() Name() = %q, want default", backend.Name())
	}
	if backend.cfg.BaseURL != "https://api.example.test" {
		t.Fatalf("NewBackend() BaseURL = %q, want all trailing slashes removed", backend.cfg.BaseURL)
	}
}

func TestOpenai_Register_Good(t *testing.T) {
	name := "openai-register-good-" + t.Name()
	backend := Register(Config{Name: name, BaseURL: "https://api.example.test", DefaultModel: "gpt"})
	got, ok := inference.Get(name)

	if backend == nil || !ok || got != backend {
		t.Fatalf("Register() backend=%#v ok=%v got=%#v, want registered backend", backend, ok, got)
	}
}

func TestOpenai_Register_Bad(t *testing.T) {
	name := "openai-register-bad-" + t.Name()
	backend := Register(Config{Name: name})

	if backend == nil {
		t.Fatal("Register() returned nil")
	}
	if backend.Available() {
		t.Fatal("Register() backend Available() = true, want unavailable without static config")
	}
}

func TestOpenai_Register_Ugly(t *testing.T) {
	name := "openai-register-ugly-" + t.Name()
	first := Register(Config{Name: name, BaseURL: "https://first.example", DefaultModel: "first"})
	second := Register(Config{Name: name, BaseURL: "https://second.example", DefaultModel: "second"})
	got, ok := inference.Get(name)

	if first == nil || second == nil || !ok || got != second {
		t.Fatalf("Register() overwrite got=%#v ok=%v, want second backend", got, ok)
	}
}

func TestOpenai_Backend_Name_Good(t *testing.T) {
	backend := NewBackend(Config{Name: "openai-test"})

	if got := backend.Name(); got != "openai-test" {
		t.Fatalf("Backend.Name() = %q, want custom name", got)
	}
}

func TestOpenai_Backend_Name_Bad(t *testing.T) {
	var backend *Backend

	if got := backend.Name(); got != defaultProviderName {
		t.Fatalf("Backend.Name() = %q, want default for nil backend", got)
	}
}

func TestOpenai_Backend_Name_Ugly(t *testing.T) {
	backend := NewBackend(Config{Name: ""})

	if got := backend.Name(); got != defaultProviderName {
		t.Fatalf("Backend.Name() = %q, want default for blank name", got)
	}
}

func TestOpenai_Backend_Available_Good(t *testing.T) {
	backend := NewBackend(Config{BaseURL: "https://api.example.test", DefaultModel: "gpt"})

	if !backend.Available() {
		t.Fatal("Backend.Available() = false, want true for configured provider")
	}
}

func TestOpenai_Backend_Available_Bad(t *testing.T) {
	backend := NewBackend(Config{BaseURL: "https://api.example.test"})

	if backend.Available() {
		t.Fatal("Backend.Available() = true, want false without model")
	}
}

func TestOpenai_Backend_Available_Ugly(t *testing.T) {
	var backend *Backend

	if backend.Available() {
		t.Fatal("Backend.Available() = true, want false for nil backend")
	}
}

func TestOpenai_Backend_LoadModel_Good(t *testing.T) {
	backend := NewBackend(Config{BaseURL: "https://api.example.test", DefaultModel: "gpt"})
	textModel, err := backend.LoadModel("")

	if err != nil {
		t.Fatalf("Backend.LoadModel() error = %v", err)
	}
	if model := textModel.(*Model); model.modelID != "gpt" {
		t.Fatalf("Backend.LoadModel() modelID = %q, want default model", model.modelID)
	}
}

func TestOpenai_Backend_LoadModel_Bad(t *testing.T) {
	var backend *Backend
	model, err := backend.LoadModel("gpt")

	if err == nil || !core.Contains(err.Error(), "backend is nil") {
		t.Fatalf("Backend.LoadModel() = (%v, %v), want nil backend failure", model, err)
	}
}

func TestOpenai_Backend_LoadModel_Ugly(t *testing.T) {
	backend := NewBackend(Config{BaseURL: "https://api.example.test", DefaultModel: "fallback"})
	textModel, err := backend.LoadModel("override")

	if err != nil {
		t.Fatalf("Backend.LoadModel() error = %v", err)
	}
	if model := textModel.(*Model); model.modelID != "override" {
		t.Fatalf("Backend.LoadModel() modelID = %q, want explicit path", model.modelID)
	}
}

func TestOpenai_Backend_Capabilities_Good(t *testing.T) {
	backend := NewBackend(Config{Name: "cap", BaseURL: "https://api.example.test", DefaultModel: "gpt"})
	report := backend.Capabilities()

	if !report.Available || !report.Supports(inference.CapabilityGenerate) || !report.Supports(inference.CapabilityChat) {
		t.Fatalf("Backend.Capabilities() = %+v, want available generate/chat report", report)
	}
}

func TestOpenai_Backend_Capabilities_Bad(t *testing.T) {
	var backend *Backend
	report := backend.Capabilities()

	if report.Available || report.Runtime.Backend != defaultProviderName {
		t.Fatalf("Backend.Capabilities() = %+v, want unavailable default report", report)
	}
}

func TestOpenai_Backend_Capabilities_Ugly(t *testing.T) {
	backend := NewBackend(Config{Name: "labels", BaseURL: "https://api.example.test/", DefaultModel: "gpt"})
	report := backend.Capabilities()

	if report.Runtime.Labels["base_url"] != "https://api.example.test" {
		t.Fatalf("Backend.Capabilities() labels = %+v, want trimmed base_url", report.Runtime.Labels)
	}
}

func TestOpenai_Model_Generate_Good(t *testing.T) {
	model, cleanup := newTestModel(t, "generated text", http.StatusOK)
	defer cleanup()

	var got string
	for token := range model.Generate(context.Background(), "hello", inference.WithMaxTokens(8)) {
		got += token.Text
	}

	if got != "generated text" {
		t.Fatalf("Model.Generate() = %q, want generated text", got)
	}
	if err := model.Err(); err != nil {
		t.Fatalf("Model.Generate() Err() = %v", err)
	}
}

func TestOpenai_Model_Generate_Bad(t *testing.T) {
	model, cleanup := newTestModel(t, "rate limited", http.StatusTooManyRequests)
	defer cleanup()

	for range model.Generate(context.Background(), "hello") {
		t.Fatal("Model.Generate() yielded token for provider error")
	}

	if err := model.Err(); err == nil || !core.Contains(err.Error(), "HTTP") {
		t.Fatalf("Model.Generate() Err() = %v, want provider failure", err)
	}
}

func TestOpenai_Model_Generate_Ugly(t *testing.T) {
	model, cleanup := newTestModel(t, "", http.StatusOK)
	defer cleanup()

	count := 0
	for range model.Generate(context.Background(), "hello") {
		count++
	}

	if count != 0 {
		t.Fatalf("Model.Generate() yielded %d tokens, want none for empty content", count)
	}
}

func TestOpenai_Model_Chat_Good(t *testing.T) {
	model, cleanup := newTestModel(t, "chat text", http.StatusOK)
	defer cleanup()

	var got string
	for token := range model.Chat(context.Background(), []inference.Message{{Role: "user", Content: "hi"}}) {
		got += token.Text
	}

	if got != "chat text" {
		t.Fatalf("Model.Chat() = %q, want chat text", got)
	}
}

func TestOpenai_Model_Chat_Bad(t *testing.T) {
	model, cleanup := newTestModel(t, "bad", http.StatusInternalServerError)
	defer cleanup()

	for range model.Chat(context.Background(), []inference.Message{{Role: "user", Content: "hi"}}) {
		t.Fatal("Model.Chat() yielded token for failed provider")
	}
	if err := model.Err(); err == nil {
		t.Fatal("Model.Chat() Err() = nil, want failure")
	}
}

func TestOpenai_Model_Chat_Ugly(t *testing.T) {
	model, cleanup := newTestModel(t, "context chat", http.StatusOK)
	defer cleanup()
	model.backend.cfg.ContextAssembler = ContextAssemblerFunc(func(context.Context, []inference.Message) core.Result {
		return core.Ok("context")
	})

	for range model.Chat(context.Background(), []inference.Message{{Role: "user", Content: "hi"}}) {
	}

	if err := model.Err(); err != nil {
		t.Fatalf("Model.Chat() Err() = %v, want context-injected success", err)
	}
}

func TestOpenai_Model_Classify_Good(t *testing.T) {
	model := &Model{}
	results, err := model.Classify(context.Background(), []string{"prompt"})

	if err == nil || !core.Contains(err.Error(), "not supported") {
		t.Fatalf("Model.Classify() = (%v, %v), want unsupported failure", results, err)
	}
}

func TestOpenai_Model_Classify_Bad(t *testing.T) {
	var model *Model
	_, err := model.Classify(context.Background(), nil)

	if err == nil {
		t.Fatal("Model.Classify() err = nil, want unsupported failure")
	}
}

func TestOpenai_Model_Classify_Ugly(t *testing.T) {
	model := &Model{}
	_, err := model.Classify(context.Background(), []string{"a", "b"}, inference.WithMaxTokens(1))

	if err == nil || !core.Contains(err.Error(), "classification") {
		t.Fatalf("Model.Classify() error = %v, want classification context", err)
	}
}

func TestOpenai_Model_BatchGenerate_Good(t *testing.T) {
	model, cleanup := newTestModel(t, "batch", http.StatusOK)
	defer cleanup()
	batches, err := model.BatchGenerate(context.Background(), []string{"a", "b"})

	if err != nil {
		t.Fatalf("Model.BatchGenerate() error = %v", err)
	}
	if len(batches) != 2 || len(batches[0].Tokens) != 1 {
		t.Fatalf("Model.BatchGenerate() = %+v, want two token batches", batches)
	}
}

func TestOpenai_Model_BatchGenerate_Bad(t *testing.T) {
	model, cleanup := newTestModel(t, "bad", http.StatusBadGateway)
	defer cleanup()
	batches, err := model.BatchGenerate(context.Background(), []string{"a"})

	if err != nil {
		t.Fatalf("Model.BatchGenerate() outer error = %v, want per-prompt error", err)
	}
	if len(batches) != 1 || batches[0].Err == nil {
		t.Fatalf("Model.BatchGenerate() = %+v, want per-prompt error", batches)
	}
}

func TestOpenai_Model_BatchGenerate_Ugly(t *testing.T) {
	model, cleanup := newTestModel(t, "unused", http.StatusOK)
	defer cleanup()
	batches, err := model.BatchGenerate(context.Background(), nil)

	if err != nil || len(batches) != 0 {
		t.Fatalf("Model.BatchGenerate() = (%+v, %v), want empty batch success", batches, err)
	}
}

func TestOpenai_Model_ModelType_Good(t *testing.T) {
	model := &Model{}

	if got := model.ModelType(); got != "openai-compatible" {
		t.Fatalf("Model.ModelType() = %q, want openai-compatible", got)
	}
}

func TestOpenai_Model_ModelType_Bad(t *testing.T) {
	var model *Model

	if got := model.ModelType(); got == "" {
		t.Fatal("Model.ModelType() = empty, want stable type even for nil receiver")
	}
}

func TestOpenai_Model_ModelType_Ugly(t *testing.T) {
	model := &Model{modelID: "custom"}

	if got := model.ModelType(); !core.Contains(got, "openai") {
		t.Fatalf("Model.ModelType() = %q, want provider family", got)
	}
}

func TestOpenai_Model_Info_Good(t *testing.T) {
	model := &Model{}
	info := model.Info()

	if info.Architecture != "openai-compatible" {
		t.Fatalf("Model.Info() = %+v, want openai-compatible architecture", info)
	}
}

func TestOpenai_Model_Info_Bad(t *testing.T) {
	var model *Model
	info := model.Info()

	if info.Architecture == "" {
		t.Fatalf("Model.Info() = %+v, want architecture for nil receiver", info)
	}
}

func TestOpenai_Model_Info_Ugly(t *testing.T) {
	model := &Model{modelID: "gpt-test"}
	info := model.Info()

	if info.QuantBits != 0 || info.NumLayers != 0 {
		t.Fatalf("Model.Info() = %+v, want external provider metadata only", info)
	}
}

func TestOpenai_Model_Metrics_Good(t *testing.T) {
	model := &Model{metrics: inference.GenerateMetrics{PromptTokens: 3, GeneratedTokens: 2}}
	metrics := model.Metrics()

	if metrics.PromptTokens != 3 || metrics.GeneratedTokens != 2 {
		t.Fatalf("Model.Metrics() = %+v, want stored metrics", metrics)
	}
}

func TestOpenai_Model_Metrics_Bad(t *testing.T) {
	model := &Model{}
	metrics := model.Metrics()

	if metrics.PromptTokens != 0 || metrics.GeneratedTokens != 0 {
		t.Fatalf("Model.Metrics() = %+v, want zero metrics before request", metrics)
	}
}

func TestOpenai_Model_Metrics_Ugly(t *testing.T) {
	model := &Model{}
	model.setResult(inference.GenerateMetrics{GeneratedTokens: 7}, core.Ok(nil))
	metrics := model.Metrics()

	if metrics.GeneratedTokens != 7 {
		t.Fatalf("Model.Metrics() = %+v, want setResult metrics", metrics)
	}
}

func TestOpenai_Model_Err_Good(t *testing.T) {
	model := &Model{}
	err := model.Err()

	if err != nil {
		t.Fatalf("Model.Err() = %v, want nil before failure", err)
	}
}

func TestOpenai_Model_Err_Bad(t *testing.T) {
	model := &Model{lastErr: core.E("test", "failed", nil)}
	err := model.Err()

	if err == nil || !core.Contains(err.Error(), "failed") {
		t.Fatalf("Model.Err() = %v, want stored error", err)
	}
}

func TestOpenai_Model_Err_Ugly(t *testing.T) {
	model := &Model{}
	model.setResult(inference.GenerateMetrics{}, core.Fail(core.E("test", "set failure", nil)))
	err := model.Err()

	if err == nil || !core.Contains(err.Error(), "set failure") {
		t.Fatalf("Model.Err() = %v, want setResult failure", err)
	}
}

func TestOpenai_Model_Close_Good(t *testing.T) {
	model := &Model{}
	err := model.Close()

	if err != nil {
		t.Fatalf("Model.Close() = %v, want nil", err)
	}
}

func TestOpenai_Model_Close_Bad(t *testing.T) {
	var model *Model
	err := model.Close()

	if err != nil {
		t.Fatalf("Model.Close() = %v, want nil receiver close nil", err)
	}
}

func TestOpenai_Model_Close_Ugly(t *testing.T) {
	model := &Model{lastErr: core.AnError}
	err := model.Close()

	if err != nil || model.lastErr == nil {
		t.Fatalf("Model.Close() = %v lastErr=%v, want close without clearing generation error", err, model.lastErr)
	}
}

func TestOpenai_Model_Capabilities_Good(t *testing.T) {
	backend := NewBackend(Config{Name: "cap", BaseURL: "https://api.example.test", DefaultModel: "gpt"})
	model := &Model{backend: backend, modelID: "gpt"}
	report := model.Capabilities()

	if report.Model.ID != "gpt" || !report.Supports(inference.CapabilityGenerate) {
		t.Fatalf("Model.Capabilities() = %+v, want model capability report", report)
	}
}

func TestOpenai_Model_Capabilities_Bad(t *testing.T) {
	var model *Model
	report := model.Capabilities()

	if report.Runtime.Backend != defaultProviderName || report.Model.ID != "" {
		t.Fatalf("Model.Capabilities() = %+v, want default nil model report", report)
	}
}

func TestOpenai_Model_Capabilities_Ugly(t *testing.T) {
	backend := NewBackend(Config{Name: "cap", BaseURL: "https://api.example.test/", DefaultModel: "gpt"})
	model := &Model{backend: backend, modelID: "gpt"}
	report := model.Capabilities()

	if report.Runtime.Labels["base_url"] != "https://api.example.test" || report.Model.Labels["provider"] == "" {
		t.Fatalf("Model.Capabilities() labels = runtime:%+v model:%+v", report.Runtime.Labels, report.Model.Labels)
	}
}

func newTestModel(t *testing.T, content string, status int) (*Model, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(core.JSONMarshalString(openaicompat.ErrorResponse{
				Error: openaicompat.ErrorObject{Message: content},
			})))
			return
		}
		_, _ = w.Write([]byte(core.JSONMarshalString(openaicompat.ChatCompletionResponse{
			Model: "gpt-test",
			Choices: []openaicompat.ChatChoice{{
				Message: openaicompat.ChatMessage{Role: "assistant", Content: content},
			}},
			Usage: openaicompat.ChatUsage{PromptTokens: 1, CompletionTokens: 1},
		})))
	}))
	backend := NewBackend(Config{
		Name:         "test",
		BaseURL:      server.URL,
		DefaultModel: "gpt-test",
		HTTPClient:   server.Client(),
	})
	textModel, err := backend.LoadModel("")
	if err != nil {
		t.Fatalf("LoadModel() error = %v", err)
	}
	return textModel.(*Model), server.Close
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
