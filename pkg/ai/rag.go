package ai

import (
	"context"
	"time"

	"forge.lthn.ai/core/go-rag"
)

var (
	newQdrantClient = rag.NewQdrantClient
	newOllamaClient = rag.NewOllamaClient
	runRAGQuery     = rag.Query
	closeQdrant     = func(client *rag.QdrantClient) error { return client.Close() }
)

// TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"} carries the minimal task data needed for RAG queries.
type TaskInfo struct {
	Title       string
	Description string
}

// QueryRAGForTask(TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"}) returns formatted RAG context.
func QueryRAGForTask(task TaskInfo) (string, error) {
	queryText := task.Title
	if task.Description != "" {
		queryText += ": " + task.Description
	}

	// Truncate to 500 runes to keep the embedding focused.
	runes := []rune(queryText)
	if len(runes) > 500 {
		queryText = string(runes[:500])
	}

	qdrantConfig := rag.DefaultQdrantConfig()
	qdrantClient, err := newQdrantClient(qdrantConfig)
	if err != nil {
		return "", nil
	}
	defer func() { _ = closeQdrant(qdrantClient) }()

	ollamaConfig := rag.DefaultOllamaConfig()
	ollamaClient, err := newOllamaClient(ollamaConfig)
	if err != nil {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryConfig := rag.QueryConfig{
		Collection: "hostuk-docs",
		Limit:      3,
		Threshold:  0.5,
	}

	results, err := runRAGQuery(ctx, qdrantClient, ollamaClient, queryText, queryConfig)
	if err != nil {
		return "", nil
	}

	return rag.FormatResultsContext(results), nil
}
