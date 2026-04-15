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

func TestBuildTaskQuery_Good_TruncatesCombinedQuery(t *testing.T) {
	got := buildTaskQuery(TaskInfo{
		Title:       strings.Repeat("t", ragTaskQueryRuneLimit),
		Description: "extra",
	})

	if gotRuneLen := len([]rune(got)); gotRuneLen != ragTaskQueryRuneLimit {
		t.Fatalf("buildTaskQuery() rune length = %d, want %d", gotRuneLen, ragTaskQueryRuneLimit)
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

func TestBuildTaskQuery_Good_TruncatesDescriptionBeforeComposition(t *testing.T) {
	got := buildTaskQuery(TaskInfo{
		Title:       "Investigate",
		Description: strings.Repeat("y", ragTaskQueryRuneLimit+25),
	})

	if gotRuneLen := len([]rune(got)); gotRuneLen != ragTaskQueryRuneLimit {
		t.Fatalf("buildTaskQuery() rune length = %d, want %d", gotRuneLen, ragTaskQueryRuneLimit)
	}
	if !strings.HasPrefix(got, "Investigate: ") {
		t.Fatalf("buildTaskQuery() = %q, want title prefix preserved", got)
	}
}

func TestBuildTaskQuery_Good_TruncatesCombinedQueryExactly(t *testing.T) {
	title := strings.Repeat("t", 320)
	description := strings.Repeat("d", 320)

	got := buildTaskQuery(TaskInfo{
		Title:       title,
		Description: description,
	})

	want := truncateRunes(title+": "+description, ragTaskQueryRuneLimit)
	if got != want {
		t.Fatalf("buildTaskQuery() = %q, want %q", got, want)
	}
}

func TestBuildTaskQuery_Good_BlankTaskReturnsEmpty(t *testing.T) {
	got := buildTaskQuery(TaskInfo{})
	if got != "" {
		t.Fatalf("buildTaskQuery() = %q, want empty string", got)
	}
}

func TestBuildTaskQuery_Good_UsesDescriptionWithRFCSeparator(t *testing.T) {
	got := buildTaskQuery(TaskInfo{
		Description: "CI compile step fails",
	})

	want := ": CI compile step fails"
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

func TestRag_QueryRAGForTask_Good_ReturnsFormattedContext(t *testing.T) {
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

	var seenQuery string
	var seenConfig rag.QueryConfig
	newQdrantClient = func(rag.QdrantConfig) (*rag.QdrantClient, error) {
		return nil, nil
	}
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		return nil, nil
	}
	closeQdrant = func(*rag.QdrantClient) error { return nil }
	runRAGQuery = func(
		_ context.Context,
		_ rag.VectorStore,
		_ rag.Embedder,
		query string,
		cfg rag.QueryConfig,
	) ([]rag.QueryResult, error) {
		seenQuery = query
		seenConfig = cfg
		return []rag.QueryResult{
			{
				Text:    "Build failure runbook",
				Source:  "docs/build.md",
				Section: "Troubleshooting",
				Score:   0.91,
			},
		}, nil
	}

	got, err := QueryRAGForTask(TaskInfo{
		Title:       "Investigate build failure",
		Description: "CI compile step fails",
	})
	if err != nil {
		t.Fatalf("QueryRAGForTask() error = %v, want nil", err)
	}
	if got == "" {
		t.Fatal("QueryRAGForTask() returned empty context for a populated result set")
	}
	if seenQuery != "Investigate build failure: CI compile step fails" {
		t.Fatalf("QueryRAGForTask() query = %q, want task title + description", seenQuery)
	}
	if seenConfig.Collection != ragTaskCollection || seenConfig.Limit != ragTaskResultLimit || seenConfig.Threshold != ragTaskSimilarityThreshold {
		t.Fatalf("QueryRAGForTask() config = %+v, want collection/limit/threshold defaults", seenConfig)
	}

	want := rag.FormatResultsContext([]rag.QueryResult{{
		Text:    "Build failure runbook",
		Source:  "docs/build.md",
		Section: "Troubleshooting",
		Score:   0.91,
	}})
	if got != want {
		t.Fatalf("QueryRAGForTask() = %q, want %q", got, want)
	}
}

func TestRag_QueryRAGForTask_Bad_ReturnsEmptyStringWhenNoResults(t *testing.T) {
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
		return nil, nil
	}
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		return nil, nil
	}
	closeQdrant = func(*rag.QdrantClient) error { return nil }
	runRAGQuery = func(
		_ context.Context,
		_ rag.VectorStore,
		_ rag.Embedder,
		_ string,
		_ rag.QueryConfig,
	) ([]rag.QueryResult, error) {
		return nil, nil
	}

	got, err := QueryRAGForTask(TaskInfo{
		Title:       "Investigate build failure",
		Description: "CI compile step fails",
	})
	if err != nil {
		t.Fatalf("QueryRAGForTask() error = %v, want nil", err)
	}
	if got != "" {
		t.Fatalf("QueryRAGForTask() = %q, want empty string for no matches", got)
	}
}

func TestRag_QueryRAGForTask_Ugly_EmptyTaskShortCircuitsSeams(t *testing.T) {
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
		t.Fatal("newQdrantClient should not be called for an empty task")
		return nil, nil
	}
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) {
		t.Fatal("newOllamaClient should not be called for an empty task")
		return nil, nil
	}
	runRAGQuery = func(
		_ context.Context,
		_ rag.VectorStore,
		_ rag.Embedder,
		_ string,
		_ rag.QueryConfig,
	) ([]rag.QueryResult, error) {
		t.Fatal("runRAGQuery should not be called for an empty task")
		return nil, nil
	}
	closeQdrant = func(*rag.QdrantClient) error {
		t.Fatal("closeQdrant should not be called for an empty task")
		return nil
	}

	got, err := QueryRAGForTask(TaskInfo{})
	if err != nil {
		t.Fatalf("QueryRAGForTask() error = %v, want nil", err)
	}
	if got != "" {
		t.Fatalf("QueryRAGForTask() = %q, want empty string for empty task", got)
	}
}

func TestRag_truncateRunes_Ugly_NonPositiveLimitReturnsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
	}{
		{name: "zero", limit: 0},
		{name: "negative", limit: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateRunes("hello", tc.limit); got != "" {
				t.Fatalf("truncateRunes(%q, %d) = %q, want empty string", "hello", tc.limit, got)
			}
		})
	}
}
