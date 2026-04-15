package ai

import (
	"time"

	pkgai "dappco.re/go/ai/pkg/ai"
)

// Event{Type: "security.scan", Repo: "wailsapp/wails"} records AI or security activity in ~/.core/ai/metrics/YYYY-MM-DD.jsonl.
type Event = pkgai.Event

// Record(Event{Type: "security.scan", Repo: "wailsapp/wails"}) appends the event to the daily JSONL log.
func Record(event Event) error {
	return pkgai.Record(event)
}

// ReadEvents(time.Now().Add(-24 * time.Hour)) reads recent daily JSONL files and silently skips any missing days.
func ReadEvents(since time.Time) ([]Event, error) {
	return pkgai.ReadEvents(since)
}

// Summary([]Event{{Type: "build", Repo: "core-php", AgentID: "agent-1"}}) aggregates counts by type, repo, and agent.
func Summary(events []Event) map[string]any {
	return pkgai.Summary(events)
}
