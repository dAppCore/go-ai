package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	rag "dappco.re/go/rag"
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

func TestBuildTaskQuery_Good_BlankTaskReturnsEmpty(t *testing.T) {
	got := buildTaskQuery(TaskInfo{})
	if got != "" {
		t.Fatalf("buildTaskQuery() = %q, want empty string", got)
	}
}

func TestBuildTaskQuery_Good_UsesDescriptionWithoutLeadingSeparator(t *testing.T) {
	got := buildTaskQuery(TaskInfo{
		Description: "CI compile step fails",
	})

	want := "CI compile step fails"
	if got != want {
		t.Fatalf("buildTaskQuery() = %q, want %q", got, want)
	}
}

func TestQueryRAGForTask_Good_DegradesOnClientErrors(t *testing.T) {
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

	if got, err := QueryRAGForTask(TaskInfo{Title: "Investigate", Description: "failure"}); err != nil {
		t.Fatalf("QueryRAGForTask() error = %v, want nil", err)
	} else if got != "" {
		t.Fatalf("QueryRAGForTask() = %q, want empty string", got)
	}

	newQdrantClient = origNewQdrantClient
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		return nil, errors.New("ollama unavailable")
	}

	if got, err := QueryRAGForTask(TaskInfo{Title: "Investigate", Description: "failure"}); err != nil {
		t.Fatalf("QueryRAGForTask() error = %v, want nil", err)
	} else if got != "" {
		t.Fatalf("QueryRAGForTask() = %q, want empty string", got)
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

	if got, err := QueryRAGForTask(TaskInfo{Title: "Investigate", Description: "failure"}); err != nil {
		t.Fatalf("QueryRAGForTask() error = %v, want nil", err)
	} else if got != "" {
		t.Fatalf("QueryRAGForTask() = %q, want empty string", got)
	}
}
