package ai

import (
	"context"

	core "dappco.re/go"
	rag "dappco.re/go/rag"
)

func ExampleQueryRAGForTask() {
	origNewQdrantClient := newQdrantClient
	origNewOllamaClient := newOllamaClient
	origRunRAGQuery := runRAGQuery
	origCloseQdrant := closeQdrant
	defer func() {
		newQdrantClient = origNewQdrantClient
		newOllamaClient = origNewOllamaClient
		runRAGQuery = origRunRAGQuery
		closeQdrant = origCloseQdrant
	}()

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
		return []rag.QueryResult{{Text: "Use the build runbook", Source: "docs/build.md", Section: "Checks", Score: 0.9}}, nil
	}

	result := QueryRAGForTask(TaskInfo{Title: "Investigate build failure", Description: "CI failed"})
	contextText := result.Value.(string)

	core.Println(result.OK)
	core.Println(core.Contains(contextText, "Use the build runbook"))
	// Output:
	// true
	// true
}
