package ai

import (
	"context"
	"strings"
	"time"

	"forge.lthn.ai/core/go-rag"
)

const (
	ragTaskCollection          = "hostuk-docs"
	ragTaskResultLimit         = 3
	ragTaskSimilarityThreshold = 0.5
	ragTaskQueryRuneLimit      = 500
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

// QueryRAGForTask(TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"}) returns formatted RAG context,
// or an empty string when retrieval is unavailable or yields no matches.
func QueryRAGForTask(task TaskInfo) (string, error) {
	queryText := buildTaskQuery(task)
	if queryText == "" {
		return "", nil
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
		Collection: ragTaskCollection,
		Limit:      ragTaskResultLimit,
		Threshold:  ragTaskSimilarityThreshold,
	}

	results, err := runRAGQuery(ctx, qdrantClient, ollamaClient, queryText, queryConfig)
	if err != nil {
		return "", nil
	}
	if len(results) == 0 {
		return "", nil
	}

	return rag.FormatResultsContext(results), nil
}

func buildTaskQuery(task TaskInfo) string {
	if strings.TrimSpace(task.Title) == "" && strings.TrimSpace(task.Description) == "" {
		return ""
	}
	query := task.Title + ": " + task.Description
	return truncateRunes(query, ragTaskQueryRuneLimit)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
