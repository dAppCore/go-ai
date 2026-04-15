package ai

import (
	"bufio"
	"cmp"
	"slices"
	"sync"
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// metricsWriteMu protects concurrent file writes in Record.
var metricsWriteMu sync.Mutex

// Event{Type: "security.scan", Repo: "wailsapp/wails"} records AI or security activity in ~/.core/ai/metrics/YYYY-MM-DD.jsonl.
type Event struct {
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	AgentID   string         `json:"agent_id,omitempty"`
	Repo      string         `json:"repo,omitempty"`
	Duration  time.Duration  `json:"duration,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// metricsDir returns the base directory for metrics storage.
func metricsDir() (string, error) {
	home := core.Env("CORE_HOME")
	if home == "" {
		home = core.Env("DIR_HOME")
	}
	return core.JoinPath(home, ".core", "ai", "metrics"), nil
}

// metricsFilePath returns the JSONL file path for the given date.
func metricsFilePath(dir string, t time.Time) string {
	return core.JoinPath(dir, t.Format("2006-01-02")+".jsonl")
}

// Record(Event{Type: "security.scan", Repo: "wailsapp/wails"}) appends the event to the daily JSONL log.
func Record(event Event) (err error) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	metricsWriteMu.Lock()
	defer metricsWriteMu.Unlock()

	dir, err := metricsDir()
	if err != nil {
		return coreerr.E("ai", "record event", err)
	}

	if err := coreio.Local.EnsureDir(dir); err != nil {
		return coreerr.E("ai", "record event", err)
	}

	path := metricsFilePath(dir, event.Timestamp)

	file, err := coreio.Local.Append(path)
	if err != nil {
		return coreerr.E("ai", "record event", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = coreerr.E("ai", "record event", closeErr)
		}
	}()

	data := core.JSONMarshal(event)
	if !data.OK {
		if marshalErr, ok := data.Value.(error); ok {
			return coreerr.E("ai", "record event", marshalErr)
		}
		return coreerr.E("ai", "record event", nil)
	}

	if _, err := file.Write(append(data.Value.([]byte), '\n')); err != nil {
		return coreerr.E("ai", "record event", err)
	}

	return nil
}

// ReadEvents(time.Now().Add(-24 * time.Hour)) reads recent daily JSONL files and silently skips any missing days.
func ReadEvents(since time.Time) ([]Event, error) {
	dir, err := metricsDir()
	if err != nil {
		return nil, coreerr.E("ai", "read events", err)
	}

	var events []Event
	now := time.Now()

	// Iterate each day from since to now in the caller's location.
	loc := since.Location()
	for d := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, loc); !d.After(now.In(loc)); d = d.AddDate(0, 0, 1) {
		path := metricsFilePath(dir, d)

		dayEvents, err := readMetricsFile(path, since)
		if err != nil {
			return nil, err
		}
		events = append(events, dayEvents...)
	}

	slices.SortStableFunc(events, func(a, b Event) int {
		return cmp.Compare(a.Timestamp.UnixNano(), b.Timestamp.UnixNano())
	})

	return events, nil
}

// readMetricsFile reads events from a single JSONL file, returning only those at or after since.
func readMetricsFile(path string, since time.Time) ([]Event, error) {
	if !coreio.Local.Exists(path) {
		return nil, nil
	}

	content, err := coreio.Local.Read(path)
	if err != nil {
		return nil, coreerr.E("ai", "read events", err)
	}

	var events []Event
	scanner := bufio.NewScanner(core.NewReader(content))
	// Metrics payloads are small in practice, but the default scanner token limit
	// is too low for larger JSON events with rich Data payloads.
	scanner.Buffer(make([]byte, 1024), 1<<20)
	for scanner.Scan() {
		var ev Event
		if r := core.JSONUnmarshal(scanner.Bytes(), &ev); !r.OK {
			continue // skip malformed lines
		}
		if !ev.Timestamp.Before(since) {
			events = append(events, ev)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, coreerr.E("ai", "read events", err)
	}
	return events, nil
}

// Summary(events) aggregates counts by type, repo, and agent.
// Example: Summary([]Event{{Type: "build", Repo: "core-php", AgentID: "agent-1"}})
func Summary(events []Event) map[string]any {
	byTypeCounts := make(map[string]int)
	byRepoCounts := make(map[string]int)
	byAgentCounts := make(map[string]int)

	for _, ev := range events {
		byTypeCounts[ev.Type]++
		if ev.Repo != "" {
			byRepoCounts[ev.Repo]++
		}
		if ev.AgentID != "" {
			byAgentCounts[ev.AgentID]++
		}
	}

	recentEvents := events
	if len(recentEvents) > 10 {
		recentEvents = recentEvents[len(recentEvents)-10:]
	}
	recentCopy := make([]Event, len(recentEvents))
	copy(recentCopy, recentEvents)

	return map[string]any{
		"by_type":  cloneCounts(byTypeCounts),
		"by_repo":  cloneCounts(byRepoCounts),
		"by_agent": cloneCounts(byAgentCounts),
		"recent":   recentCopy,
	}
}

func cloneCounts(counts map[string]int) map[string]int {
	cloned := make(map[string]int, len(counts))
	for key, count := range counts {
		cloned[key] = count
	}
	return cloned
}
