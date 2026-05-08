// SPDX-License-Identifier: EUPL-1.2

package ai

import (
	"context"

	core "dappco.re/go"
	"dappco.re/go/inference"
)

// RAGContextAssembler adapts the package RAG helper to provider context
// injection.
type RAGContextAssembler struct {
	Task  TaskInfo
	Query func(TaskInfo) core.Result
}

// AssembleContext returns formatted retrieval context for the current chat.
func (a RAGContextAssembler) AssembleContext(_ context.Context, messages []inference.Message) (string, error) {
	task := a.Task
	if core.Trim(task.Title) == "" && core.Trim(task.Description) == "" {
		task.Title = lastUserMessage(messages)
	}
	if core.Trim(task.Title) == "" && core.Trim(task.Description) == "" {
		return "", nil
	}
	query := a.Query
	if query == nil {
		query = QueryRAGForTask
	}
	result := query(task)
	if !result.OK {
		return "", core.NewError(result.Error())
	}
	contextText, _ := result.Value.(string)
	return contextText, nil
}

func lastUserMessage(messages []inference.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if core.Lower(core.Trim(messages[i].Role)) == "user" {
			return core.Trim(messages[i].Content)
		}
	}
	return ""
}
