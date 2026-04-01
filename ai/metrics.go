package ai

import (
	"bufio"
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// metricsMu protects concurrent file writes in Record.
var metricsMu sync.Mutex

// Event represents a recorded AI/security metric event.
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
		return "", coreerr.E("ai.metricsDir", "get home directory", err)
	}
	return filepath.Join(home, ".core", "ai", "metrics"), nil
}

// metricsFilePath returns the JSONL file path for the given date.
func metricsFilePath(dir string, t time.Time) string {
	return filepath.Join(dir, t.Format("2006-01-02")+".jsonl")
}

// Record appends an event to the daily JSONL file at
// ~/.core/ai/metrics/YYYY-MM-DD.jsonl.
func Record(event Event) (err error) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	metricsMu.Lock()
	defer metricsMu.Unlock()

	dir, err := metricsDir()
	if err != nil {
		return err
	}

	if err := coreio.Local.EnsureDir(dir); err != nil {
		return coreerr.E("ai.Record", "create metrics directory", err)
	}

	path := metricsFilePath(dir, event.Timestamp)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return coreerr.E("ai.Record", "open metrics file", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = coreerr.E("ai.Record", "close metrics file", cerr)
		}
	}()

	data, err := json.Marshal(event)
	if err != nil {
		return coreerr.E("ai.Record", "marshal event", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return coreerr.E("ai.Record", "write event", err)
	}

	return nil
}

// ReadEvents reads events from JSONL files within the given time range.
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
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, coreerr.E("ai.readMetricsFile", "open metrics file", err)
	}
	defer func() { _ = f.Close() }()

	var events []Event
	scanner := bufio.NewScanner(f)
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
		return nil, coreerr.E("ai.readMetricsFile", "read metrics file", err)
	}
	return events, nil
}

// Summary aggregates events into counts by type, repo, and agent.
func Summary(events []Event) map[string]any {
	byType := make(map[string]int)
	byRepo := make(map[string]int)
	byAgent := make(map[string]int)

	for _, ev := range events {
		byType[ev.Type]++
		if ev.Repo != "" {
			byRepo[ev.Repo]++
		}
		if ev.AgentID != "" {
			byAgent[ev.AgentID]++
		}
	}

	recent := make([]Event, len(events))
	copy(recent, events)
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].Timestamp.After(recent[j].Timestamp)
	})
	if len(recent) > 10 {
		recent = recent[:10]
	}

	return map[string]any{
		"total":    len(events),
		"by_type":  sortedMap(byType),
		"by_repo":  sortedMap(byRepo),
		"by_agent": sortedMap(byAgent),
		"events":   briefEvents(recent),
	}
}

// sortedMap returns a slice of key-count pairs sorted by count descending.
func sortedMap(m map[string]int) []map[string]any {
	type entry struct {
		key   string
		count int
	}
	entries := make([]entry, 0, len(m))
	for k, v := range m {
		entries = append(entries, entry{k, v})
	}

	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Compare(b.count, a.count)
	})

	result := make([]map[string]any, len(entries))
	for i, e := range entries {
		result[i] = map[string]any{"key": e.key, "count": e.count}
	}
	return result
}

// briefEvents converts events into the compact shape used by metrics_query.
func briefEvents(events []Event) []map[string]any {
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
