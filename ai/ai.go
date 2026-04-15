// Package ai provides the canonical AI facade for the core CLI.
//
// Example:
//
//	contextText, err := ai.QueryRAGForTask(ai.TaskInfo{
//		Title:       "Investigate build failure",
//		Description: "CI compile step fails",
//	})
//
// Example:
//
//	err := ai.Record(ai.Event{Type: "security.scan", Repo: "wailsapp/wails"})
package ai
