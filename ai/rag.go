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
)

// TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"} carries the minimal task data needed for RAG queries.
type TaskInfo struct {
	Title       string
	Description string
}

// QueryRAGForTask(TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"}) returns formatted RAG context, or "" when dependencies are unavailable.
func QueryRAGForTask(task TaskInfo) string {
	queryText := task.Title + " " + task.Description

	// Truncate to 500 runes to keep the embedding focused.
	runes := []rune(queryText)
	if len(runes) > 500 {
		queryText = string(runes[:500])
	}

	qdrantConfig := rag.DefaultQdrantConfig()
	qdrantClient, err := newQdrantClient(qdrantConfig)
	if err != nil {
		return ""
	}
	defer func() { _ = qdrantClient.Close() }()

	ollamaConfig := rag.DefaultOllamaConfig()
	ollamaClient, err := newOllamaClient(ollamaConfig)
	if err != nil {
		return ""
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
		return ""
	}

	return rag.FormatResultsContext(results)
}
