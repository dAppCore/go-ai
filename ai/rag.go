// RAG helpers for task-scoped documentation lookup.
package ai

import (
	"context"
	"strings"
	"time"

	rag "dappco.re/go/rag"
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

// ai.TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"} carries the minimal task data needed for RAG queries.
type TaskInfo struct {
	Title       string
	Description string
}

//	contextText, err := ai.QueryRAGForTask(ai.TaskInfo{
//		Title:       "Investigate build failure",
//		Description: "CI compile step fails",
//	})
func QueryRAGForTask(task TaskInfo) (string, error) {
	queryText := buildTaskQuery(task)
	if queryText == "" {
		return "", nil
	}

	qdrantConfiguration := rag.DefaultQdrantConfig()
	qdrantClient, err := newQdrantClient(qdrantConfiguration)
	if err != nil {
		return "", nil
	}
	if qdrantClient != nil {
		defer func() { _ = closeQdrant(qdrantClient) }()
	}

	ollamaConfiguration := rag.DefaultOllamaConfig()
	ollamaClient, err := newOllamaClient(ollamaConfiguration)
	if err != nil {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryConfiguration := rag.QueryConfig{
		Collection: ragTaskCollection,
		Limit:      ragTaskResultLimit,
		Threshold:  ragTaskSimilarityThreshold,
	}

	results, err := runRAGQuery(ctx, qdrantClient, ollamaClient, queryText, queryConfiguration)
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

	return truncateRunes(task.Title+": "+task.Description, ragTaskQueryRuneLimit)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	inputRunes := []rune(value)
	if len(inputRunes) <= limit {
		return value
	}
	return string(inputRunes[:limit])
}
