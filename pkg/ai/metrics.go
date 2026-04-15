package ai

import (
	"bufio"
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", coreerr.E("ai", "get metrics directory", err)
	}
	return filepath.Join(home, ".core", "ai", "metrics"), nil
}

// metricsFilePath returns the JSONL file path for the given date.
func metricsFilePath(dir string, t time.Time) string {
	return filepath.Join(dir, t.Format("2006-01-02")+".jsonl")
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
		return err
	}

	if err := coreio.Local.EnsureDir(dir); err != nil {
		return coreerr.E("ai", "record event", err)
	}

	path := metricsFilePath(dir, event.Timestamp)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return coreerr.E("ai", "record event", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = coreerr.E("ai", "record event", closeErr)
		}
	}()

	data, err := json.Marshal(event)
	if err != nil {
		return coreerr.E("ai", "record event", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return coreerr.E("ai", "record event", err)
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
		return nil, coreerr.E("ai", "read metrics", err)
	}

	var events []Event
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue // skip malformed lines
		}
		if !ev.Timestamp.Before(since) {
			events = append(events, ev)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, coreerr.E("ai", "read metrics", err)
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
		"total":           len(events),
		"by_type":         cloneCounts(byTypeCounts),
		"by_repo":         cloneCounts(byRepoCounts),
		"by_agent":        cloneCounts(byAgentCounts),
		"by_type_sorted":  sortedCountPairs(byTypeCounts),
		"by_repo_sorted":  sortedCountPairs(byRepoCounts),
		"by_agent_sorted": sortedCountPairs(byAgentCounts),
		"recent":          recentEvents,
	}
}

func cloneCounts(counts map[string]int) map[string]int {
	cloned := make(map[string]int, len(counts))
	for key, count := range counts {
		cloned[key] = count
	}
	return cloned
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
