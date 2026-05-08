// SPDX-License-Identifier: EUPL-1.2

// Package openai provides an outbound OpenAI-compatible provider backend for
// go-ai consumers. It implements the shared inference contracts without
// importing local GPU runtimes or core/api.
package openai

import (
	"context"
	"io"
	"iter"
	"net/http"
	"sync"
	"time"

	core "dappco.re/go"
	"dappco.re/go/inference"
	openaicompat "dappco.re/go/inference/openai"
)

const (
	defaultProviderName = "openai"
	defaultHTTPTimeout  = 60 * time.Second
)

// Limiter is satisfied by *ratelimit.RateLimiter without forcing this package
// to own quota policy.
type Limiter interface {
	WaitForCapacity(context.Context, string, int) error
	RecordUsage(model string, promptTokens, outputTokens int)
}

// ContextAssembler optionally injects retrieval/context-pack material before a
// provider request. go-rag adapters can satisfy this shape without creating a
// dependency cycle.
type ContextAssembler interface {
	AssembleContext(context.Context, []inference.Message) (string, error)
}

// ContextAssemblerFunc adapts a function to ContextAssembler.
type ContextAssemblerFunc func(context.Context, []inference.Message) (string, error)

func (fn ContextAssemblerFunc) AssembleContext(ctx context.Context, messages []inference.Message) (string, error) {
	if fn == nil {
		return "", nil
	}
	return fn(ctx, messages)
}

// Config describes one OpenAI-compatible external provider.
type Config struct {
	Name             string
	BaseURL          string
	APIKey           string
	Organisation     string
	Project          string
	DefaultModel     string
	HTTPClient       *http.Client
	Limiter          Limiter
	ContextAssembler ContextAssembler
	EstimateTokens   func([]inference.Message, inference.GenerateConfig) int
}

// Backend implements inference.Backend for an external OpenAI-compatible
// provider.
type Backend struct {
	cfg Config
}

var _ inference.Backend = (*Backend)(nil)
var _ inference.CapabilityReporter = (*Backend)(nil)

// NewBackend creates an outbound OpenAI-compatible provider backend.
func NewBackend(cfg Config) *Backend {
	cfg.Name = defaultString(cfg.Name, defaultProviderName)
	cfg.BaseURL = trimTrailingSlash(cfg.BaseURL)
	return &Backend{cfg: cfg}
}

// Register creates and registers an outbound provider backend with the shared
// inference registry.
func Register(cfg Config) *Backend {
	backend := NewBackend(cfg)
	inference.Register(backend)
	return backend
}

// Name implements inference.Backend.
func (b *Backend) Name() string {
	if b == nil {
		return defaultProviderName
	}
	return defaultString(b.cfg.Name, defaultProviderName)
}

// Available reports whether the provider has enough static configuration to
// attempt requests.
func (b *Backend) Available() bool {
	return b != nil && core.Trim(b.cfg.BaseURL) != "" && core.Trim(b.cfg.DefaultModel) != ""
}

// LoadModel creates a lightweight model handle for the requested provider
// model. path is interpreted as the provider model id; an empty path uses
// Config.DefaultModel.
func (b *Backend) LoadModel(path string, _ ...inference.LoadOption) (inference.TextModel, error) {
	if b == nil {
		return nil, core.E("ai.openai.LoadModel", "backend is nil", nil)
	}
	modelID := core.Trim(path)
	if modelID == "" {
		modelID = core.Trim(b.cfg.DefaultModel)
	}
	if modelID == "" {
		return nil, core.E("ai.openai.LoadModel", "model id is required", nil)
	}
	if core.Trim(b.cfg.BaseURL) == "" {
		return nil, core.E("ai.openai.LoadModel", "base URL is required", nil)
	}
	return &Model{
		backend: b,
		modelID: modelID,
		client:  httpClient(b.cfg.HTTPClient),
	}, nil
}

// Capabilities implements inference.CapabilityReporter.
func (b *Backend) Capabilities() inference.CapabilityReport {
	baseURL := ""
	if b != nil {
		baseURL = core.Trim(b.cfg.BaseURL)
	}
	return inference.CapabilityReport{
		Runtime: inference.RuntimeIdentity{
			Backend:       b.Name(),
			Device:        "external",
			NativeRuntime: false,
			Labels: map[string]string{
				"provider": "openai-compatible",
				"base_url": baseURL,
			},
		},
		Available: b.Available(),
		Capabilities: []inference.Capability{
			inference.SupportedCapability(inference.CapabilityModelLoad, inference.CapabilityGroupRuntime),
			inference.SupportedCapability(inference.CapabilityGenerate, inference.CapabilityGroupModel),
			inference.SupportedCapability(inference.CapabilityChat, inference.CapabilityGroupModel),
		},
	}
}

// Model is a loaded external provider model handle.
type Model struct {
	backend *Backend
	modelID string
	client  *http.Client

	mu      sync.Mutex
	lastErr error
	metrics inference.GenerateMetrics
}

var _ inference.TextModel = (*Model)(nil)
var _ inference.CapabilityReporter = (*Model)(nil)

// Generate implements inference.TextModel.
func (m *Model) Generate(ctx context.Context, prompt string, opts ...inference.GenerateOption) iter.Seq[inference.Token] {
	return m.Chat(ctx, []inference.Message{{Role: "user", Content: prompt}}, opts...)
}

// Chat implements inference.TextModel.
func (m *Model) Chat(ctx context.Context, messages []inference.Message, opts ...inference.GenerateOption) iter.Seq[inference.Token] {
	return func(yield func(inference.Token) bool) {
		content, metrics, err := m.complete(ctx, messages, opts...)
		m.setResult(metrics, err)
		if err != nil || content == "" {
			return
		}
		yield(inference.Token{Text: content})
	}
}

// Classify is not exposed for external chat providers yet.
func (m *Model) Classify(context.Context, []string, ...inference.GenerateOption) ([]inference.ClassifyResult, error) {
	return nil, core.E("ai.openai.Classify", "classification is not supported by this provider backend", nil)
}

// BatchGenerate runs Generate sequentially for each prompt.
func (m *Model) BatchGenerate(ctx context.Context, prompts []string, opts ...inference.GenerateOption) ([]inference.BatchResult, error) {
	results := make([]inference.BatchResult, 0, len(prompts))
	for _, prompt := range prompts {
		var tokens []inference.Token
		for token := range m.Generate(ctx, prompt, opts...) {
			tokens = append(tokens, token)
		}
		results = append(results, inference.BatchResult{Tokens: tokens, Err: m.Err()})
	}
	return results, nil
}

// ModelType implements inference.TextModel.
func (m *Model) ModelType() string {
	return "openai-compatible"
}

// Info implements inference.TextModel.
func (m *Model) Info() inference.ModelInfo {
	return inference.ModelInfo{Architecture: "openai-compatible"}
}

// Metrics implements inference.TextModel.
func (m *Model) Metrics() inference.GenerateMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.metrics
}

// Err implements inference.TextModel.
func (m *Model) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastErr
}

// Close implements inference.TextModel.
func (m *Model) Close() error {
	return nil
}

// Capabilities implements inference.CapabilityReporter.
func (m *Model) Capabilities() inference.CapabilityReport {
	backendName := defaultProviderName
	baseURL := ""
	if m != nil && m.backend != nil {
		backendName = m.backend.Name()
		baseURL = core.Trim(m.backend.cfg.BaseURL)
	}
	modelID := ""
	if m != nil {
		modelID = m.modelID
	}
	return inference.CapabilityReport{
		Runtime: inference.RuntimeIdentity{
			Backend:       backendName,
			Device:        "external",
			NativeRuntime: false,
			Labels: map[string]string{
				"provider": "openai-compatible",
				"base_url": baseURL,
			},
		},
		Model: inference.ModelIdentity{
			ID:           modelID,
			Architecture: "openai-compatible",
			Labels: map[string]string{
				"provider": "openai-compatible",
			},
		},
		Available: true,
		Capabilities: []inference.Capability{
			inference.SupportedCapability(inference.CapabilityGenerate, inference.CapabilityGroupModel),
			inference.SupportedCapability(inference.CapabilityChat, inference.CapabilityGroupModel),
		},
	}
}

func (m *Model) complete(ctx context.Context, messages []inference.Message, opts ...inference.GenerateOption) (string, inference.GenerateMetrics, error) {
	if m == nil || m.backend == nil {
		return "", inference.GenerateMetrics{}, core.E("ai.openai.complete", "model is nil", nil)
	}
	cfg := inference.ApplyGenerateOpts(opts)
	messages, err := m.contextMessages(ctx, messages)
	if err != nil {
		return "", inference.GenerateMetrics{}, err
	}
	if limiter := m.backend.cfg.Limiter; limiter != nil {
		if err := limiter.WaitForCapacity(ctx, m.modelID, m.estimateTokens(messages, cfg)); err != nil {
			return "", inference.GenerateMetrics{}, err
		}
	}

	req := openaicompat.ChatCompletionRequest{
		Model:    m.modelID,
		Messages: openaiMessages(messages),
		Stream:   false,
	}
	if cfg.MaxTokens > 0 {
		req.MaxTokens = &cfg.MaxTokens
	}
	req.Temperature = &cfg.Temperature
	if cfg.TopP > 0 {
		req.TopP = &cfg.TopP
	}
	if cfg.TopK > 0 {
		req.TopK = &cfg.TopK
	}

	started := time.Now()
	response, err := m.doRequest(ctx, req)
	if err != nil {
		return "", inference.GenerateMetrics{}, err
	}
	metrics := inference.GenerateMetrics{
		PromptTokens:    response.Usage.PromptTokens,
		GeneratedTokens: response.Usage.CompletionTokens,
		TotalDuration:   time.Since(started),
	}
	if limiter := m.backend.cfg.Limiter; limiter != nil {
		limiter.RecordUsage(m.modelID, response.Usage.PromptTokens, response.Usage.CompletionTokens)
	}
	if len(response.Choices) == 0 {
		return "", metrics, core.E("ai.openai.complete", "provider response contained no choices", nil)
	}
	return response.Choices[0].Message.Content, metrics, nil
}

func (m *Model) contextMessages(ctx context.Context, messages []inference.Message) ([]inference.Message, error) {
	out := append([]inference.Message(nil), messages...)
	assembler := m.backend.cfg.ContextAssembler
	if assembler == nil {
		return out, nil
	}
	contextText, err := assembler.AssembleContext(ctx, out)
	if err != nil {
		return nil, err
	}
	contextText = core.Trim(contextText)
	if contextText == "" {
		return out, nil
	}
	contextMessage := inference.Message{
		Role:    "system",
		Content: core.Concat("Context:\n", contextText),
	}
	out = append([]inference.Message{contextMessage}, out...)
	return out, nil
}

func (m *Model) doRequest(ctx context.Context, req openaicompat.ChatCompletionRequest) (openaicompat.ChatCompletionResponse, error) {
	payload := core.JSONMarshalString(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(m.backend.cfg.BaseURL), core.NewReader(payload))
	if err != nil {
		return openaicompat.ChatCompletionResponse{}, core.E("ai.openai.doRequest", "create request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if key := core.Trim(m.backend.cfg.APIKey); key != "" {
		httpReq.Header.Set("Authorization", core.Concat("Bearer ", key))
	}
	if organisation := core.Trim(m.backend.cfg.Organisation); organisation != "" {
		httpReq.Header.Set("OpenAI-Organization", organisation)
	}
	if project := core.Trim(m.backend.cfg.Project); project != "" {
		httpReq.Header.Set("OpenAI-Project", project)
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return openaicompat.ChatCompletionResponse{}, core.E("ai.openai.doRequest", "provider request", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return openaicompat.ChatCompletionResponse{}, core.E("ai.openai.doRequest", "read provider response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return openaicompat.ChatCompletionResponse{}, providerError(resp.StatusCode, string(body))
	}
	var out openaicompat.ChatCompletionResponse
	result := core.JSONUnmarshalString(string(body), &out)
	if !result.OK {
		return openaicompat.ChatCompletionResponse{}, core.NewError(result.Error())
	}
	return out, nil
}

func (m *Model) estimateTokens(messages []inference.Message, cfg inference.GenerateConfig) int {
	if estimate := m.backend.cfg.EstimateTokens; estimate != nil {
		return estimate(messages, cfg)
	}
	totalRunes := 0
	for _, msg := range messages {
		totalRunes += core.RuneCount(msg.Content)
	}
	estimate := totalRunes / 4
	if estimate < 1 {
		estimate = 1
	}
	if cfg.MaxTokens > 0 {
		estimate += cfg.MaxTokens
	}
	return estimate
}

func (m *Model) setResult(metrics inference.GenerateMetrics, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = metrics
	m.lastErr = err
}

func openaiMessages(messages []inference.Message) []openaicompat.ChatMessage {
	out := make([]openaicompat.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, openaicompat.ChatMessage{Role: msg.Role, Content: msg.Content})
	}
	return out
}

func chatCompletionsURL(baseURL string) string {
	return core.Concat(trimTrailingSlash(baseURL), openaicompat.DefaultChatCompletionsPath)
}

func providerError(status int, body string) error {
	var payload openaicompat.ErrorResponse
	if result := core.JSONUnmarshalString(body, &payload); result.OK && payload.Error.Message != "" {
		return core.E("ai.openai.provider", core.Sprintf("provider returned HTTP %d: %s", status, payload.Error.Message), nil)
	}
	if body != "" {
		return core.E("ai.openai.provider", core.Sprintf("provider returned HTTP %d: %s", status, body), nil)
	}
	return core.E("ai.openai.provider", core.Sprintf("provider returned HTTP %d", status), nil)
}

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func defaultString(value, fallback string) string {
	if core.Trim(value) == "" {
		return fallback
	}
	return value
}

func trimTrailingSlash(value string) string {
	value = core.Trim(value)
	for core.HasSuffix(value, "/") {
		value = core.TrimSuffix(value, "/")
	}
	return value
}
