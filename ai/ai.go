// Package ai provides the canonical AI facade for the core CLI.
//
//	contextText, err := ai.QueryRAGForTask(ai.TaskInfo{
//		Title:       "Investigate build failure",
//		Description: "CI compile step fails",
//	})
//
// err := ai.Record(ai.Event{Type: "security.scan", Repo: "wailsapp/wails"})
package ai
