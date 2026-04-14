package ai

import pkgai "dappco.re/go/core/ai/pkg/ai"

// TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"} carries the minimal task data needed for RAG queries.
type TaskInfo = pkgai.TaskInfo

// QueryRAGForTask(TaskInfo{Title: "Investigate build failure", Description: "CI compile step fails"}) returns formatted RAG context.
func QueryRAGForTask(task TaskInfo) (string, error) {
	return pkgai.QueryRAGForTask(task)
}
