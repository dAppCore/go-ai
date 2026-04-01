package ai

import (
	"context"
	"errors"
	"testing"

	"forge.lthn.ai/core/go-rag"
)

func TestQueryRAGForTask_Good_FallsBackToEmptyString(t *testing.T) {
	origNewQdrantClient := newQdrantClient
	origNewOllamaClient := newOllamaClient
	origRunRAGQuery := runRAGQuery
	t.Cleanup(func() {
		newQdrantClient = origNewQdrantClient
		newOllamaClient = origNewOllamaClient
		runRAGQuery = origRunRAGQuery
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

	got := QueryRAGForTask(TaskInfo{
		Title:       "Investigate build failure",
		Description: "The compile step is failing in CI",
	})

	if got != "" {
		t.Fatalf("expected empty fallback context, got %q", got)
	}
}
