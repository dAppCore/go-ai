package ai

import (
	"context"
	"strings"
	"time"

	coreerr "dappco.re/go/core/log"
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

// QueryRAGForTask(TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"}) returns formatted RAG context.
func QueryRAGForTask(task TaskInfo) (string, error) {
	queryText := buildTaskQuery(task)
	if queryText == "" {
		return "", nil
	}

	qdrantConfig := rag.DefaultQdrantConfig()
	qdrantClient, err := newQdrantClient(qdrantConfig)
	if err != nil {
		return "", coreerr.E("ai", "query RAG for task", err)
	}
	defer func() { _ = closeQdrant(qdrantClient) }()

	ollamaConfig := rag.DefaultOllamaConfig()
	ollamaClient, err := newOllamaClient(ollamaConfig)
	if err != nil {
		return "", coreerr.E("ai", "query RAG for task", err)
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
		return "", coreerr.E("ai", "query RAG for task", err)
	}
	if len(results) == 0 {
		return "", nil
	}

	return rag.FormatResultsContext(results), nil
}

func buildTaskQuery(task TaskInfo) string {
	title := strings.TrimSpace(task.Title)
	description := strings.TrimSpace(task.Description)

	if title == "" && description == "" {
		return ""
	}

	return truncateRunes(title+": "+description, ragTaskQueryRuneLimit)
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
