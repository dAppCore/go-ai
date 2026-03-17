package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	coreio "forge.lthn.ai/core/go-io"
)

// withTempHome overrides HOME to a temp dir for the duration of the test.
func withTempHome(t *testing.T) {
	t.Helper()
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	metricsPath := filepath.Join(tmpHome, ".core", "ai", "metrics")
	if err := coreio.Local.EnsureDir(metricsPath); err != nil {
		t.Fatalf("create metrics dir: %v", err)
	}
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
}

// --- Record ---

func TestRecord_Good(t *testing.T) {
	withTempHome(t)

	ev := Event{
		Type:    "test_event",
		AgentID: "agent-1",
		Repo:    "repo-1",
		Data:    map[string]any{"key": "value"},
	}
	if err := Record(ev); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, err := ReadEvents(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "test_event" {
		t.Errorf("expected type test_event, got %s", events[0].Type)
	}
	if events[0].Timestamp.IsZero() {
		t.Error("expected auto-set timestamp, got zero")
	}
}

func TestRecord_Good_AutoTimestamp(t *testing.T) {
	withTempHome(t)

	before := time.Now()
	ev := Event{Type: "auto_ts"}
	if err := Record(ev); err != nil {
		t.Fatalf("Record: %v", err)
	}
	after := time.Now()

	events, err := ReadEvents(before)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ts := events[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v not in range [%v, %v]", ts, before, after)
	}
}

func TestRecord_Good_PresetTimestamp(t *testing.T) {
	withTempHome(t)

	fixed := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	ev := Event{Type: "preset_ts", Timestamp: fixed}
	if err := Record(ev); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, err := ReadEvents(fixed.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].Timestamp.Equal(fixed) {
		t.Errorf("expected timestamp %v, got %v", fixed, events[0].Timestamp)
	}
}

// --- ReadEvents ---

func TestReadEvents_Good_Empty(t *testing.T) {
	withTempHome(t)

	events, err := ReadEvents(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestReadEvents_Good_MultiDay(t *testing.T) {
	withTempHome(t)

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	// Write event for yesterday
	ev1 := Event{Type: "yesterday", Timestamp: yesterday}
	if err := Record(ev1); err != nil {
		t.Fatalf("Record yesterday: %v", err)
	}

	// Write event for today
	ev2 := Event{Type: "today", Timestamp: now}
	if err := Record(ev2); err != nil {
		t.Fatalf("Record today: %v", err)
	}

	// Read from 2 days ago — should get both
	events, err := ReadEvents(now.AddDate(0, 0, -2))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestReadEvents_Good_FiltersBySince(t *testing.T) {
	withTempHome(t)

	now := time.Now()
	// Write an old event and a recent one
	old := Event{Type: "old", Timestamp: now.Add(-2 * time.Hour)}
	if err := Record(old); err != nil {
		t.Fatalf("Record old: %v", err)
	}
	recent := Event{Type: "recent", Timestamp: now}
	if err := Record(recent); err != nil {
		t.Fatalf("Record recent: %v", err)
	}

	// Read only events from the last hour
	events, err := ReadEvents(now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "recent" {
		t.Errorf("expected type recent, got %s", events[0].Type)
	}
}

// --- readMetricsFile ---

func TestReadMetricsFile_Good_MalformedLines(t *testing.T) {
	withTempHome(t)

	dir, err := metricsDir()
	if err != nil {
		t.Fatalf("metricsDir: %v", err)
	}

	// Write a file with a mix of valid and invalid lines
	path := metricsFilePath(dir, time.Now())
	content := `{"type":"valid","timestamp":"2026-03-15T10:00:00Z"}
not-json
{"type":"also_valid","timestamp":"2026-03-15T11:00:00Z"}
{broken
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	events, err := readMetricsFile(path, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("readMetricsFile: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 valid events (skipping malformed), got %d", len(events))
	}
}

func TestReadMetricsFile_Good_NonExistent(t *testing.T) {
	events, err := readMetricsFile("/tmp/nonexistent-metrics-file.jsonl", time.Time{})
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// --- metricsFilePath ---

func TestMetricsFilePath_Good(t *testing.T) {
	ts := time.Date(2026, 3, 17, 14, 30, 0, 0, time.UTC)
	got := metricsFilePath("/base", ts)
	want := "/base/2026-03-17.jsonl"
	if got != want {
		t.Errorf("metricsFilePath = %q, want %q", got, want)
	}
}

// --- Summary ---

func TestSummary_Good_Empty(t *testing.T) {
	s := Summary(nil)
	total, ok := s["total"].(int)
	if !ok || total != 0 {
		t.Errorf("expected total 0, got %v", s["total"])
	}
}

func TestSummary_Good(t *testing.T) {
	events := []Event{
		{Type: "build", Repo: "core-php", AgentID: "agent-1"},
		{Type: "build", Repo: "core-php", AgentID: "agent-2"},
		{Type: "test", Repo: "core-api", AgentID: "agent-1"},
	}

	s := Summary(events)

	total, _ := s["total"].(int)
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	byType, _ := s["by_type"].([]map[string]any)
	if len(byType) != 2 {
		t.Fatalf("expected 2 types, got %d", len(byType))
	}
	// Sorted by count descending — "build" (2) first
	if byType[0]["key"] != "build" || byType[0]["count"] != 2 {
		t.Errorf("expected build:2 first, got %v:%v", byType[0]["key"], byType[0]["count"])
	}
}

// --- sortedMap ---

func TestSortedMap_Good_Empty(t *testing.T) {
	result := sortedMap(map[string]int{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(result))
	}
}

func TestSortedMap_Good_Ordering(t *testing.T) {
	m := map[string]int{"a": 1, "b": 3, "c": 2}
	result := sortedMap(m)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	// Descending by count
	if result[0]["key"] != "b" {
		t.Errorf("expected first key 'b', got %v", result[0]["key"])
	}
	if result[1]["key"] != "c" {
		t.Errorf("expected second key 'c', got %v", result[1]["key"])
	}
	if result[2]["key"] != "a" {
		t.Errorf("expected third key 'a', got %v", result[2]["key"])
	}
}
