package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forge.lthn.ai/core/go-inference"
	"forge.lthn.ai/core/go-ml"
	"forge.lthn.ai/core/go/pkg/core"
	"forge.lthn.ai/core/go-log"
)

// --- Mock backend for inference registry ---

// mockInferenceBackend implements inference.Backend for CI testing of ml_backends.
type mockInferenceBackend struct {
	name      string
	available bool
}

func (m *mockInferenceBackend) Name() string { return m.name }
func (m *mockInferenceBackend) Available() bool { return m.available }
func (m *mockInferenceBackend) LoadModel(_ string, _ ...inference.LoadOption) (inference.TextModel, error) {
	return nil, fmt.Errorf("mock backend: LoadModel not implemented")
}

// --- Mock ml.Backend for Generate ---

// mockMLBackend implements ml.Backend for CI testing.
type mockMLBackend struct {
	name         string
	available    bool
	generateResp string
	generateErr  error
}

func (m *mockMLBackend) Name() string      { return m.name }
func (m *mockMLBackend) Available() bool    { return m.available }

func (m *mockMLBackend) Generate(_ context.Context, _ string, _ ml.GenOpts) (ml.Result, error) {
	return ml.Result{Text: m.generateResp}, m.generateErr
}

func (m *mockMLBackend) Chat(_ context.Context, _ []ml.Message, _ ml.GenOpts) (ml.Result, error) {
	return ml.Result{Text: m.generateResp}, m.generateErr
}

// newTestMLSubsystem creates an MLSubsystem with a real ml.Service for testing.
func newTestMLSubsystem(t *testing.T, backends ...ml.Backend) *MLSubsystem {
	t.Helper()
	c, err := core.New(
		core.WithName("ml", ml.NewService(ml.Options{})),
	)
	if err != nil {
		t.Fatalf("Failed to create framework core: %v", err)
	}
	svc, err := core.ServiceFor[*ml.Service](c, "ml")
	if err != nil {
		t.Fatalf("Failed to get ML service: %v", err)
	}
	// Register mock backends
	for _, b := range backends {
		svc.RegisterBackend(b.Name(), b)
	}
	return &MLSubsystem{
		service: svc,
		logger:  log.Default(),
	}
}

// --- Input/Output struct tests ---

// TestMLGenerateInput_Good verifies all fields can be set.
func TestMLGenerateInput_Good(t *testing.T) {
	input := MLGenerateInput{
		Prompt:      "Hello world",
		Backend:     "test",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   100,
	}
	if input.Prompt != "Hello world" {
		t.Errorf("Expected prompt 'Hello world', got %q", input.Prompt)
	}
	if input.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", input.Temperature)
	}
	if input.MaxTokens != 100 {
		t.Errorf("Expected max_tokens 100, got %d", input.MaxTokens)
	}
}

// TestMLScoreInput_Good verifies all fields can be set.
func TestMLScoreInput_Good(t *testing.T) {
	input := MLScoreInput{
		Prompt:   "test prompt",
		Response: "test response",
		Suites:   "heuristic,semantic",
	}
	if input.Prompt != "test prompt" {
		t.Errorf("Expected prompt 'test prompt', got %q", input.Prompt)
	}
	if input.Response != "test response" {
		t.Errorf("Expected response 'test response', got %q", input.Response)
	}
}

// TestMLProbeInput_Good verifies all fields can be set.
func TestMLProbeInput_Good(t *testing.T) {
	input := MLProbeInput{
		Backend:    "test",
		Categories: "reasoning,code",
	}
	if input.Backend != "test" {
		t.Errorf("Expected backend 'test', got %q", input.Backend)
	}
}

// TestMLStatusInput_Good verifies all fields can be set.
func TestMLStatusInput_Good(t *testing.T) {
	input := MLStatusInput{
		InfluxURL: "http://localhost:8086",
		InfluxDB:  "lem",
	}
	if input.InfluxURL != "http://localhost:8086" {
		t.Errorf("Expected InfluxURL, got %q", input.InfluxURL)
	}
}

// TestMLBackendsInput_Good verifies empty struct.
func TestMLBackendsInput_Good(t *testing.T) {
	_ = MLBackendsInput{}
}

// TestMLBackendsOutput_Good verifies struct fields.
func TestMLBackendsOutput_Good(t *testing.T) {
	output := MLBackendsOutput{
		Backends: []MLBackendInfo{
			{Name: "ollama", Available: true},
			{Name: "llama", Available: false},
		},
		Default: "ollama",
	}
	if len(output.Backends) != 2 {
		t.Fatalf("Expected 2 backends, got %d", len(output.Backends))
	}
	if output.Default != "ollama" {
		t.Errorf("Expected default 'ollama', got %q", output.Default)
	}
	if !output.Backends[0].Available {
		t.Error("Expected first backend to be available")
	}
}

// TestMLProbeOutput_Good verifies struct fields.
func TestMLProbeOutput_Good(t *testing.T) {
	output := MLProbeOutput{
		Total: 2,
		Results: []MLProbeResultItem{
			{ID: "probe-1", Category: "reasoning", Response: "test"},
			{ID: "probe-2", Category: "code", Response: "test2"},
		},
	}
	if output.Total != 2 {
		t.Errorf("Expected total 2, got %d", output.Total)
	}
	if output.Results[0].ID != "probe-1" {
		t.Errorf("Expected ID 'probe-1', got %q", output.Results[0].ID)
	}
}

// TestMLStatusOutput_Good verifies struct fields.
func TestMLStatusOutput_Good(t *testing.T) {
	output := MLStatusOutput{Status: "OK: 5 training runs"}
	if output.Status != "OK: 5 training runs" {
		t.Errorf("Unexpected status: %q", output.Status)
	}
}

// TestMLGenerateOutput_Good verifies struct fields.
func TestMLGenerateOutput_Good(t *testing.T) {
	output := MLGenerateOutput{
		Response: "Generated text here",
		Backend:  "ollama",
		Model:    "qwen3:8b",
	}
	if output.Response != "Generated text here" {
		t.Errorf("Unexpected response: %q", output.Response)
	}
}

// TestMLScoreOutput_Good verifies struct fields.
func TestMLScoreOutput_Good(t *testing.T) {
	output := MLScoreOutput{
		Heuristic: &ml.HeuristicScores{},
	}
	if output.Heuristic == nil {
		t.Error("Expected Heuristic to be set")
	}
	if output.Semantic != nil {
		t.Error("Expected Semantic to be nil")
	}
}

// --- Handler validation tests ---

// TestMLGenerate_Bad_EmptyPrompt verifies empty prompt returns error.
func TestMLGenerate_Bad_EmptyPrompt(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, _, err := m.mlGenerate(ctx, nil, MLGenerateInput{})
	if err == nil {
		t.Fatal("Expected error for empty prompt")
	}
	if !strings.Contains(err.Error(), "prompt cannot be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestMLGenerate_Good_WithMockBackend verifies generate works with a mock backend.
func TestMLGenerate_Good_WithMockBackend(t *testing.T) {
	mock := &mockMLBackend{
		name:         "test-mock",
		available:    true,
		generateResp: "mock response",
	}
	m := newTestMLSubsystem(t, mock)
	ctx := context.Background()

	_, out, err := m.mlGenerate(ctx, nil, MLGenerateInput{
		Prompt:  "test",
		Backend: "test-mock",
	})
	if err != nil {
		t.Fatalf("mlGenerate failed: %v", err)
	}
	if out.Response != "mock response" {
		t.Errorf("Expected 'mock response', got %q", out.Response)
	}
}

// TestMLGenerate_Bad_NoBackend verifies generate fails gracefully without a backend.
func TestMLGenerate_Bad_NoBackend(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, _, err := m.mlGenerate(ctx, nil, MLGenerateInput{
		Prompt:  "test",
		Backend: "nonexistent",
	})
	if err == nil {
		t.Fatal("Expected error for missing backend")
	}
	if !strings.Contains(err.Error(), "no backend available") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestMLScore_Bad_EmptyPrompt verifies empty prompt returns error.
func TestMLScore_Bad_EmptyPrompt(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, _, err := m.mlScore(ctx, nil, MLScoreInput{Response: "some"})
	if err == nil {
		t.Fatal("Expected error for empty prompt")
	}
}

// TestMLScore_Bad_EmptyResponse verifies empty response returns error.
func TestMLScore_Bad_EmptyResponse(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, _, err := m.mlScore(ctx, nil, MLScoreInput{Prompt: "some"})
	if err == nil {
		t.Fatal("Expected error for empty response")
	}
}

// TestMLScore_Good_Heuristic verifies heuristic scoring without live services.
func TestMLScore_Good_Heuristic(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, out, err := m.mlScore(ctx, nil, MLScoreInput{
		Prompt:   "What is Go?",
		Response: "Go is a statically typed, compiled programming language designed at Google.",
		Suites:   "heuristic",
	})
	if err != nil {
		t.Fatalf("mlScore failed: %v", err)
	}
	if out.Heuristic == nil {
		t.Fatal("Expected heuristic scores to be set")
	}
}

// TestMLScore_Good_DefaultSuite verifies default suite is heuristic.
func TestMLScore_Good_DefaultSuite(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, out, err := m.mlScore(ctx, nil, MLScoreInput{
		Prompt:   "What is Go?",
		Response: "Go is a statically typed, compiled programming language designed at Google.",
	})
	if err != nil {
		t.Fatalf("mlScore failed: %v", err)
	}
	if out.Heuristic == nil {
		t.Fatal("Expected heuristic scores (default suite)")
	}
}

// TestMLScore_Bad_SemanticNoJudge verifies semantic scoring fails without a judge.
func TestMLScore_Bad_SemanticNoJudge(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, _, err := m.mlScore(ctx, nil, MLScoreInput{
		Prompt:   "test",
		Response: "test",
		Suites:   "semantic",
	})
	if err == nil {
		t.Fatal("Expected error for semantic scoring without judge")
	}
	if !strings.Contains(err.Error(), "requires a judge") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestMLScore_Bad_ContentSuite verifies content suite redirects to ml_probe.
func TestMLScore_Bad_ContentSuite(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, _, err := m.mlScore(ctx, nil, MLScoreInput{
		Prompt:   "test",
		Response: "test",
		Suites:   "content",
	})
	if err == nil {
		t.Fatal("Expected error for content suite")
	}
	if !strings.Contains(err.Error(), "ContentProbe") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestMLProbe_Good_WithMockBackend verifies probes run with mock backend.
func TestMLProbe_Good_WithMockBackend(t *testing.T) {
	mock := &mockMLBackend{
		name:         "probe-mock",
		available:    true,
		generateResp: "probe response",
	}
	m := newTestMLSubsystem(t, mock)
	ctx := context.Background()

	_, out, err := m.mlProbe(ctx, nil, MLProbeInput{
		Backend:    "probe-mock",
		Categories: "reasoning",
	})
	if err != nil {
		t.Fatalf("mlProbe failed: %v", err)
	}
	// Should have run probes in the "reasoning" category
	for _, r := range out.Results {
		if r.Category != "reasoning" {
			t.Errorf("Expected category 'reasoning', got %q", r.Category)
		}
		if r.Response != "probe response" {
			t.Errorf("Expected 'probe response', got %q", r.Response)
		}
	}
	if out.Total != len(out.Results) {
		t.Errorf("Expected total %d, got %d", len(out.Results), out.Total)
	}
}

// TestMLProbe_Good_NoCategory verifies all probes run without category filter.
func TestMLProbe_Good_NoCategory(t *testing.T) {
	mock := &mockMLBackend{
		name:         "all-probe-mock",
		available:    true,
		generateResp: "ok",
	}
	m := newTestMLSubsystem(t, mock)
	ctx := context.Background()

	_, out, err := m.mlProbe(ctx, nil, MLProbeInput{Backend: "all-probe-mock"})
	if err != nil {
		t.Fatalf("mlProbe failed: %v", err)
	}
	// Should run all 23 probes
	if out.Total != len(ml.CapabilityProbes) {
		t.Errorf("Expected %d probes, got %d", len(ml.CapabilityProbes), out.Total)
	}
}

// TestMLBackends_Good_EmptyRegistry verifies empty result when no backends registered.
func TestMLBackends_Good_EmptyRegistry(t *testing.T) {
	m := newTestMLSubsystem(t)
	ctx := context.Background()

	// Note: inference.List() returns global registry state.
	// This test verifies the handler runs without panic.
	_, out, err := m.mlBackends(ctx, nil, MLBackendsInput{})
	if err != nil {
		t.Fatalf("mlBackends failed: %v", err)
	}
	// We can't guarantee what's in the global registry, but it should not panic
	_ = out
}

// TestMLBackends_Good_WithMockInferenceBackend verifies registered backend appears.
func TestMLBackends_Good_WithMockInferenceBackend(t *testing.T) {
	// Register a mock backend in the global inference registry
	mock := &mockInferenceBackend{name: "test-ci-mock", available: true}
	inference.Register(mock)

	m := newTestMLSubsystem(t)
	ctx := context.Background()

	_, out, err := m.mlBackends(ctx, nil, MLBackendsInput{})
	if err != nil {
		t.Fatalf("mlBackends failed: %v", err)
	}

	found := false
	for _, b := range out.Backends {
		if b.Name == "test-ci-mock" {
			found = true
			if !b.Available {
				t.Error("Expected mock backend to be available")
			}
		}
	}
	if !found {
		t.Error("Expected to find 'test-ci-mock' in backends list")
	}
}

// TestMLSubsystem_Good_Name verifies subsystem name.
func TestMLSubsystem_Good_Name(t *testing.T) {
	m := newTestMLSubsystem(t)
	if m.Name() != "ml" {
		t.Errorf("Expected name 'ml', got %q", m.Name())
	}
}

// TestNewMLSubsystem_Good verifies constructor.
func TestNewMLSubsystem_Good(t *testing.T) {
	c, err := core.New(
		core.WithName("ml", ml.NewService(ml.Options{})),
	)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	svc, err := core.ServiceFor[*ml.Service](c, "ml")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
	sub := NewMLSubsystem(svc)
	if sub == nil {
		t.Fatal("Expected non-nil subsystem")
	}
	if sub.service != svc {
		t.Error("Expected service to be set")
	}
	if sub.logger == nil {
		t.Error("Expected logger to be set")
	}
}
