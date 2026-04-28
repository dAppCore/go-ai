package ai

import (
	"context"
	"time"

	. "dappco.re/go"
	rag "dappco.re/go/rag"
)

func TestAI_Record_Good(t *T) {
	withTempMetricsHome(t)
	err := Record(Event{Type: "security.scan", Repo: "core/go-ai"})
	events, readErr := ReadEvents(time.Now().Add(-time.Minute))

	AssertNoError(t, err)
	AssertNoError(t, readErr)
	AssertLen(t, events, 1)
}

func TestAI_Record_Bad(t *T) {
	withTempMetricsHome(t)
	err := Record(Event{Type: "security.scan", Data: map[string]any{"bad": make(chan int)}})
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "record event")
}

func TestAI_Record_Ugly(t *T) {
	withTempMetricsHome(t)
	err := Record(Event{})
	events, readErr := ReadEvents(time.Now().Add(-time.Minute))

	AssertNoError(t, err)
	AssertNoError(t, readErr)
	AssertLen(t, events, 1)
}

func TestAI_ReadEvents_Good(t *T) {
	withTempMetricsHome(t)
	recordErr := Record(Event{Type: "scan", Timestamp: time.Now().Add(-time.Second)})
	events, err := ReadEvents(time.Now().Add(-time.Minute))

	AssertNoError(t, recordErr)
	AssertNoError(t, err)
	AssertLen(t, events, 1)
}

func TestAI_ReadEvents_Bad(t *T) {
	withTempMetricsHome(t)
	events, err := ReadEvents(time.Now().Add(-time.Minute))
	got := len(events)

	AssertNoError(t, err)
	AssertEqual(t, 0, got)
}

func TestAI_ReadEvents_Ugly(t *T) {
	withTempMetricsHome(t)
	recordErr := Record(Event{Type: "scan", Timestamp: time.Now().Add(-time.Hour)})
	events, err := ReadEvents(time.Now().Add(time.Hour))

	AssertNoError(t, recordErr)
	AssertNoError(t, err)
	AssertLen(t, events, 0)
}

func TestAI_Summary_Good(t *T) {
	events := []Event{{Type: "scan", Repo: "core/go-ai", AgentID: "agent-1"}}
	summary := Summary(events)
	byType := summary["by_type"].(map[string]int)

	AssertEqual(t, 1, byType["scan"])
	AssertLen(t, summary["recent"].([]Event), 1)
}

func TestAI_Summary_Bad(t *T) {
	summary := Summary(nil)
	byType := summary["by_type"].(map[string]int)
	recent := summary["recent"].([]Event)

	AssertEmpty(t, byType)
	AssertEmpty(t, recent)
}

func TestAI_Summary_Ugly(t *T) {
	events := []Event{{Type: "scan", Data: map[string]any{"nested": []any{"x"}}}}
	summary := Summary(events)
	recent := summary["recent"].([]Event)

	recent[0].Data["nested"].([]any)[0] = "changed"
	AssertEqual(t, "x", events[0].Data["nested"].([]any)[0])
}

func TestAI_QueryRAGForTask_Good(t *T) {
	origNewQdrantClient := newQdrantClient
	origNewOllamaClient := newOllamaClient
	origRunRAGQuery := runRAGQuery
	t.Cleanup(func() {
		newQdrantClient = origNewQdrantClient
		newOllamaClient = origNewOllamaClient
		runRAGQuery = origRunRAGQuery
	})

	newQdrantClient = func(rag.QdrantConfig) (*rag.QdrantClient, error) { return nil, nil }
	newOllamaClient = func(rag.OllamaConfig) (*rag.OllamaClient, error) { return nil, nil }
	runRAGQuery = func(_ context.Context, _ rag.VectorStore, _ rag.Embedder, _ string, _ rag.QueryConfig) ([]rag.QueryResult, error) {
		return []rag.QueryResult{{Text: "Runbook", Source: "docs/build.md", Score: 0.9}}, nil
	}

	got, err := QueryRAGForTask(TaskInfo{Title: "Investigate", Description: "failure"})
	AssertNoError(t, err)
	AssertContains(t, got, "Runbook")
}

func TestAI_QueryRAGForTask_Bad(t *T) {
	got, err := QueryRAGForTask(TaskInfo{})
	want := ""

	AssertNoError(t, err)
	AssertEqual(t, want, got)
}

func TestAI_QueryRAGForTask_Ugly(t *T) {
	origNewQdrantClient := newQdrantClient
	t.Cleanup(func() {
		newQdrantClient = origNewQdrantClient
	})
	newQdrantClient = func(rag.QdrantConfig) (*rag.QdrantClient, error) {
		return nil, NewError("qdrant unavailable")
	}

	got, err := QueryRAGForTask(TaskInfo{Title: "Investigate"})
	AssertNoError(t, err)
	AssertEqual(t, "", got)
}
