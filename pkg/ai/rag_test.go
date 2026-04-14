package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forge.lthn.ai/core/go-rag"
)

func TestQueryRAGForTask_Good_GracefulDegradationWhenQdrantUnavailable(t *testing.T) {
	origNewQdrantClient := newQdrantClient
	origNewOllamaClient := newOllamaClient
	origRunRAGQuery := runRAGQuery
	origCloseQdrant := closeQdrant
	t.Cleanup(func() {
		newQdrantClient = origNewQdrantClient
		newOllamaClient = origNewOllamaClient
		runRAGQuery = origRunRAGQuery
		closeQdrant = origCloseQdrant
	})

	newQdrantClient = func(rag.QdrantConfig) (*rag.QdrantClient, error) {
		return nil, errors.New("qdrant unavailable")
	}
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		t.Fatal("unexpected ollama client construction")
		return nil, nil
	}
	runRAGQuery = func(_ context.Context, _ rag.VectorStore, _ rag.Embedder, _ string, _ rag.QueryConfig) ([]rag.QueryResult, error) {
		t.Fatal("unexpected RAG query execution")
		return nil, nil
	}

	got, err := QueryRAGForTask(TaskInfo{
		Title:       "Investigate build failure",
		Description: "The compile step is failing in CI",
	})

	if got != "" {
		t.Fatalf("expected empty context on failure, got %q", got)
	}
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
}

func TestQueryRAGForTask_Good_UsesRFCQueryShape(t *testing.T) {
	origNewQdrantClient := newQdrantClient
	origNewOllamaClient := newOllamaClient
	origRunRAGQuery := runRAGQuery
	origCloseQdrant := closeQdrant
	t.Cleanup(func() {
		newQdrantClient = origNewQdrantClient
		newOllamaClient = origNewOllamaClient
		runRAGQuery = origRunRAGQuery
		closeQdrant = origCloseQdrant
	})

	newQdrantClient = func(rag.QdrantConfig) (*rag.QdrantClient, error) {
		return &rag.QdrantClient{}, nil
	}
	closeQdrant = func(*rag.QdrantClient) error { return nil }
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		return &rag.OllamaClient{}, nil
	}

	longDescription := ""
	for range 600 {
		longDescription += "x"
	}

	capturedQuery := ""
	runRAGQuery = func(_ context.Context, _ rag.VectorStore, _ rag.Embedder, query string, cfg rag.QueryConfig) ([]rag.QueryResult, error) {
		capturedQuery = query
		if cfg.Collection != ragTaskCollection || cfg.Limit != ragTaskResultLimit || cfg.Threshold != ragTaskSimilarityThreshold {
			t.Fatalf("unexpected query config: %+v", cfg)
		}
		return []rag.QueryResult{{
			Source: "docs",
			Text:   "matched context",
		}}, nil
	}

	got, err := QueryRAGForTask(TaskInfo{
		Title:       "Investigate build failure",
		Description: longDescription,
	})
	if err != nil {
		t.Fatalf("QueryRAGForTask: %v", err)
	}
	if got == "" {
		t.Fatal("expected formatted context, got empty string")
	}
	if want := ragTaskQueryRuneLimit; len([]rune(capturedQuery)) != want {
		t.Fatalf("expected RFC query limit of %d runes, got %d", want, len([]rune(capturedQuery)))
	}
	wantPrefix := "Investigate build failure:"
	if !strings.HasPrefix(capturedQuery, wantPrefix) {
		t.Fatalf("expected RFC title separator prefix %q, got %q", wantPrefix, capturedQuery)
	}
}

func TestQueryRAGForTask_Good_GracefulDegradationWhenQueryFails(t *testing.T) {
	origNewQdrantClient := newQdrantClient
	origNewOllamaClient := newOllamaClient
	origRunRAGQuery := runRAGQuery
	origCloseQdrant := closeQdrant
	t.Cleanup(func() {
		newQdrantClient = origNewQdrantClient
		newOllamaClient = origNewOllamaClient
		runRAGQuery = origRunRAGQuery
		closeQdrant = origCloseQdrant
	})

	newQdrantClient = func(rag.QdrantConfig) (*rag.QdrantClient, error) {
		return &rag.QdrantClient{}, nil
	}
	closeQdrant = func(*rag.QdrantClient) error { return nil }
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		return &rag.OllamaClient{}, nil
	}
	runRAGQuery = func(_ context.Context, _ rag.VectorStore, _ rag.Embedder, _ string, _ rag.QueryConfig) ([]rag.QueryResult, error) {
		return nil, errors.New("query failed")
	}

	got, err := QueryRAGForTask(TaskInfo{
		Title:       "Investigate build failure",
		Description: "The compile step is failing in CI",
	})

	if got != "" {
		t.Fatalf("expected empty context on query failure, got %q", got)
	}
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
}

func TestQueryRAGForTask_Good_EmptyWhenNoResults(t *testing.T) {
	origNewQdrantClient := newQdrantClient
	origNewOllamaClient := newOllamaClient
	origRunRAGQuery := runRAGQuery
	origCloseQdrant := closeQdrant
	t.Cleanup(func() {
		newQdrantClient = origNewQdrantClient
		newOllamaClient = origNewOllamaClient
		runRAGQuery = origRunRAGQuery
		closeQdrant = origCloseQdrant
	})

	newQdrantClient = func(rag.QdrantConfig) (*rag.QdrantClient, error) {
		return &rag.QdrantClient{}, nil
	}
	closeQdrant = func(*rag.QdrantClient) error { return nil }
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		return &rag.OllamaClient{}, nil
	}
	runRAGQuery = func(_ context.Context, _ rag.VectorStore, _ rag.Embedder, _ string, _ rag.QueryConfig) ([]rag.QueryResult, error) {
		return nil, nil
	}

	got, err := QueryRAGForTask(TaskInfo{
		Title:       "Investigate build failure",
		Description: "The compile step is failing in CI",
	})
	if err != nil {
		t.Fatalf("QueryRAGForTask: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty context on no results, got %q", got)
	}
}

func TestBuildTaskQuery_Good_TruncatesFullQueryToRuneLimit(t *testing.T) {
	longDescription := ""
	for range 600 {
		longDescription += "x"
	}

	got := buildTaskQuery(TaskInfo{
		Title:       "Investigate build failure",
		Description: longDescription,
	})

	wantPrefix := "Investigate build failure: "
	if got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("expected %q prefix, got %q", wantPrefix, got[:len(wantPrefix)])
	}
	if len([]rune(got)) != ragTaskQueryRuneLimit {
		t.Fatalf("expected %d-rune query, got %d", ragTaskQueryRuneLimit, len([]rune(got)))
	}
	if len([]rune(strings.TrimPrefix(got, wantPrefix))) != ragTaskQueryRuneLimit-len([]rune(wantPrefix)) {
		t.Fatalf("expected truncated description to fill the remaining query budget, got %d runes", len([]rune(strings.TrimPrefix(got, wantPrefix))))
	}
}
