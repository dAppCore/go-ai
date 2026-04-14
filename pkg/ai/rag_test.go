package ai

import (
	"context"
	"errors"
	"testing"

	"forge.lthn.ai/core/go-rag"
)

func TestQueryRAGForTask_Bad_ReturnsError(t *testing.T) {
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
	if err == nil {
		t.Fatal("expected error, got nil")
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
		if cfg.Collection != "hostuk-docs" || cfg.Limit != 3 || cfg.Threshold != 0.5 {
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
	if len([]rune(capturedQuery)) != 500 {
		t.Fatalf("expected 500-rune query, got %d", len([]rune(capturedQuery)))
	}
	if capturedQuery[:26] != "Investigate build failure:" {
		t.Fatalf("expected RFC title separator, got %q", capturedQuery[:26])
	}
}
