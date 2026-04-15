package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forge.lthn.ai/core/go-rag"
)

func TestBuildTaskQuery_Good_CombinesAndTruncates(t *testing.T) {
	got := buildTaskQuery(TaskInfo{
		Title:       "Investigate build failure",
		Description: "CI compile step fails",
	})

	want := "Investigate build failure: CI compile step fails"
	if got != want {
		t.Fatalf("buildTaskQuery() = %q, want %q", got, want)
	}
}

func TestBuildTaskQuery_Good_TruncatesToLimit(t *testing.T) {
	got := buildTaskQuery(TaskInfo{
		Title:       "",
		Description: strings.Repeat("x", ragTaskQueryRuneLimit+25),
	})

	if got == "" {
		t.Fatal("buildTaskQuery() returned empty string for non-empty task")
	}
	if gotRuneLen := len([]rune(got)); gotRuneLen != ragTaskQueryRuneLimit {
		t.Fatalf("buildTaskQuery() rune length = %d, want %d", gotRuneLen, ragTaskQueryRuneLimit)
	}
}

func TestQueryRAGForTask_Bad_PropagatesClientErrors(t *testing.T) {
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

	if _, err := QueryRAGForTask(TaskInfo{Title: "Investigate", Description: "failure"}); err == nil {
		t.Fatal("QueryRAGForTask() expected qdrant error, got nil")
	}

	newQdrantClient = origNewQdrantClient
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		return nil, errors.New("ollama unavailable")
	}

	if _, err := QueryRAGForTask(TaskInfo{Title: "Investigate", Description: "failure"}); err == nil {
		t.Fatal("QueryRAGForTask() expected ollama error, got nil")
	}

	newOllamaClient = origNewOllamaClient
	runRAGQuery = func(
		_ context.Context,
		_ rag.VectorStore,
		_ rag.Embedder,
		_ string,
		_ rag.QueryConfig,
	) ([]rag.QueryResult, error) {
		return nil, errors.New("query failed")
	}

	if _, err := QueryRAGForTask(TaskInfo{Title: "Investigate", Description: "failure"}); err == nil {
		t.Fatal("QueryRAGForTask() expected query error, got nil")
	}
}
