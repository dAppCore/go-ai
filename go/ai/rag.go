// RAG helpers for task-scoped documentation lookup.
package ai

import (
	"context"
	"time"

	"dappco.re/go"
	rag "dappco.re/go/rag"
)

const (
	ragTaskCollection          = "hostuk-docs"
	ragTaskResultLimit         = 3
	ragTaskSimilarityThreshold = 0.5
	ragTaskQueryRuneLimit      = 500
)

var (
	newQdrantClient = func(cfg rag.QdrantConfig) (*rag.QdrantClient, error) {
		result := rag.NewQdrantClient(cfg)
		if !result.OK {
			return nil, core.NewError(result.Error())
		}
		client, _ := result.Value.(*rag.QdrantClient)
		return client, nil
	}
	newOllamaClient = func(cfg rag.OllamaConfig) (*rag.OllamaClient, error) {
		result := rag.NewOllamaClient(cfg)
		if !result.OK {
			return nil, core.NewError(result.Error())
		}
		client, _ := result.Value.(*rag.OllamaClient)
		return client, nil
	}
	runRAGQuery = func(ctx context.Context, store rag.VectorStore, embedder rag.Embedder, query string, cfg rag.QueryConfig) ([]rag.QueryResult, error) {
		result := rag.Query(ctx, store, embedder, query, cfg)
		if !result.OK {
			return nil, core.NewError(result.Error())
		}
		results, _ := result.Value.([]rag.QueryResult)
		return results, nil
	}
	closeQdrant = func(client *rag.QdrantClient) error { return client.Close() }
)

// ai.TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"} carries the minimal task data needed for RAG queries.
type TaskInfo struct {
	Title       string
	Description string
}

//	contextResult := ai.QueryRAGForTask(ai.TaskInfo{
//		Title:       "Investigate build failure",
//		Description: "CI compile step fails",
//	})
func QueryRAGForTask(task TaskInfo) core.Result {
	queryText := buildTaskQuery(task)
	if queryText == "" {
		return core.Ok("")
	}

	qdrantConfiguration := rag.DefaultQdrantConfig()
	qdrantClient, err := newQdrantClient(qdrantConfiguration)
	if err != nil {
		return core.Ok("")
	}
	if qdrantClient != nil {
		defer func() { _ = closeQdrant(qdrantClient) }()
	}

	ollamaConfiguration := rag.DefaultOllamaConfig()
	ollamaClient, err := newOllamaClient(ollamaConfiguration)
	if err != nil {
		return core.Ok("")
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
		return core.Ok("")
	}
	if len(results) == 0 {
		return core.Ok("")
	}

	return core.Ok(rag.FormatResultsContext(results))
}

func buildTaskQuery(task TaskInfo) string {
	if core.Trim(task.Title) == "" && core.Trim(task.Description) == "" {
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
