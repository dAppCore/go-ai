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
	home := core.Env("DIR_HOME")
	if home == "" {
		return "", coreerr.E("ai.metricsDir", "get home directory", nil)
	}
	return core.JoinPath(home, ".core", "ai", "metrics"), nil
}

// metricsFilePath returns the JSONL file path for the given date.
func metricsFilePath(dir string, t time.Time) string {
	return core.JoinPath(dir, t.Format("2006-01-02")+".jsonl")
}

// Record(Event{Type: "security.scan", Repo: "wailsapp/wails"}) appends the event to the daily JSONL log.
func Record(event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	metricsWriteMu.Lock()
	defer metricsWriteMu.Unlock()

	dir, err := metricsDir()
	if err != nil {
		return err
	}

	if err := coreio.Local.EnsureDir(dir); err != nil {
		return coreerr.E("ai.Record", "create metrics directory", err)
	}

	path := metricsFilePath(dir, event.Timestamp)

	r := core.JSONMarshal(event)
	if !r.OK {
		return coreerr.E("ai.Record", "marshal event", r.Value.(error))
	}
	data := r.Value.([]byte)

	// Read existing content (if any) and append the new line.
	existing, _ := coreio.Local.Read(path)
	line := core.Concat(existing, string(data), "\n")
	if err := coreio.Local.Write(path, line); err != nil {
		return coreerr.E("ai.Record", "write event", err)
	}

	return nil
}

// ReadEvents(time.Now().Add(-24 * time.Hour)) reads recent daily JSONL files and silently skips any missing days.
func ReadEvents(since time.Time) ([]Event, error) {
	dir, err := metricsDir()
	if err != nil {
		return nil, err
	}

	var events []Event
	now := time.Now()

	// Iterate each day from since to now.
	for d := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, time.Local); !d.After(now); d = d.AddDate(0, 0, 1) {
		path := metricsFilePath(dir, d)

		dayEvents, err := readMetricsFile(path, since)
		if err != nil {
			return nil, err
		}
		events = append(events, dayEvents...)
	}

	return events, nil
}

// readMetricsFile reads events from a single JSONL file, returning only those at or after since.
func readMetricsFile(path string, since time.Time) ([]Event, error) {
	if !coreio.Local.Exists(path) {
		return nil, nil
	}

	content, err := coreio.Local.Read(path)
	if err != nil {
		return nil, coreerr.E("ai.readMetricsFile", "read metrics file", err)
	}

	var events []Event
	scanner := bufio.NewScanner(core.NewReader(content))
	for scanner.Scan() {
		var ev Event
		r := core.JSONUnmarshal(scanner.Bytes(), &ev)
		if !r.OK {
			continue // skip malformed lines
		}
		if !ev.Timestamp.Before(since) {
			events = append(events, ev)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, coreerr.E("ai.readMetricsFile", "scan metrics file", err)
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

	recentEvents := make([]Event, len(events))
	copy(recentEvents, events)
	slices.SortStableFunc(recentEvents, func(a, b Event) int {
		return cmp.Compare(b.Timestamp.UnixNano(), a.Timestamp.UnixNano())
	})
	if len(recentEvents) > 10 {
		recentEvents = recentEvents[:10]
	}

	return map[string]any{
		"total":    len(events),
		"by_type":  sortedCountPairs(byTypeCounts),
		"by_repo":  sortedCountPairs(byRepoCounts),
		"by_agent": sortedCountPairs(byAgentCounts),
		"events":   compactEvents(recentEvents),
	}
}

// sortedCountPairs returns a slice of key-count pairs sorted by count descending,
// with key ascending as a deterministic tie-breaker.
func sortedCountPairs(counts map[string]int) []map[string]any {
	type entry struct {
		key   string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for k, v := range counts {
		entries = append(entries, entry{k, v})
	}

	slices.SortFunc(entries, func(a, b entry) int {
		if result := cmp.Compare(b.count, a.count); result != 0 {
			return result
		}
		return cmp.Compare(a.key, b.key)
	})

	result := make([]map[string]any, len(entries))
	for i, e := range entries {
		result[i] = map[string]any{"key": e.key, "count": e.count}
	}
	return result
}

// compactEvents converts events into the compact shape used by metrics_query.
func compactEvents(events []Event) []map[string]any {
	result := make([]map[string]any, len(events))
	for i, ev := range events {
		item := map[string]any{
			"type":      ev.Type,
			"timestamp": ev.Timestamp,
		}
		if ev.AgentID != "" {
			item["agent_id"] = ev.AgentID
		}
		if ev.Repo != "" {
			item["repo"] = ev.Repo
		}
		result[i] = item
	}
	return result
}
