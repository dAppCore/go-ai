package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coreio "dappco.re/go/core/io"
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

func TestReadMetricsFile_Good_LargeJSONLine(t *testing.T) {
	withTempHome(t)

	dir, err := metricsDir()
	if err != nil {
		t.Fatalf("metricsDir: %v", err)
	}

	path := metricsFilePath(dir, time.Now())
	largeData := strings.Repeat("x", 256*1024)
	content := `{"type":"large","timestamp":"2026-03-15T10:00:00Z","data":{"payload":"` + largeData + `"}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	events, err := readMetricsFile(path, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("readMetricsFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 valid event, got %d", len(events))
	}
	if got := events[0].Data["payload"]; got != largeData {
		t.Fatalf("expected large payload to round-trip, got %T", got)
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
	byType, ok := s["by_type"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_type map, got %T", s["by_type"])
	}
	if len(byType) != 0 {
		t.Errorf("expected empty by_type map, got %v", byType)
	}
}

func TestSummary_Good(t *testing.T) {
	events := []Event{
		{Type: "build", Repo: "core-php", AgentID: "agent-1", Timestamp: time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)},
		{Type: "build", Repo: "core-php", AgentID: "agent-2", Timestamp: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)},
		{Type: "test", Repo: "core-api", AgentID: "agent-1", Timestamp: time.Date(2026, 3, 15, 11, 0, 0, 0, time.UTC)},
	}

	s := Summary(events)

	total, _ := s["total"].(int)
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	byType, ok := s["by_type"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_type map, got %T", s["by_type"])
	}
	if byType["build"] != 2 || byType["test"] != 1 {
		t.Errorf("expected type counts build=2 and test=1, got %v", byType)
	}

	byRepo, ok := s["by_repo"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_repo map, got %T", s["by_repo"])
	}
	if byRepo["core-php"] != 2 || byRepo["core-api"] != 1 {
		t.Errorf("expected repo counts core-php=2 and core-api=1, got %v", byRepo)
	}

	byAgent, ok := s["by_agent"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_agent map, got %T", s["by_agent"])
	}
	if byAgent["agent-1"] != 2 || byAgent["agent-2"] != 1 {
		t.Errorf("expected agent counts agent-1=2 and agent-2=1, got %v", byAgent)
	}

	recent, _ := s["recent"].([]Event)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent events, got %d", len(recent))
	}
	if recent[0].Type != "test" {
		t.Errorf("expected newest event first, got %v", recent[0].Type)
	}
	if recent[0].Timestamp.IsZero() {
		t.Error("expected recent event timestamp to be preserved")
	}
}

func TestSummary_Good_RecentEventsLimit(t *testing.T) {
	events := make([]Event, 0, 12)
	for i := 0; i < 12; i++ {
		events = append(events, Event{
			Type:      "type",
			Timestamp: time.Date(2026, 3, 15, 12, i, 0, 0, time.UTC),
		})
	}

	s := Summary(events)
	recent, _ := s["recent"].([]Event)
	if len(recent) != 10 {
		t.Fatalf("expected 10 recent events, got %d", len(recent))
	}
	if recent[0].Timestamp.Minute() != 11 {
		t.Errorf("expected newest event first, got minute %d", recent[0].Timestamp.Minute())
	}
	if recent[9].Timestamp.Minute() != 2 {
		t.Errorf("expected tenth newest event last, got minute %d", recent[9].Timestamp.Minute())
	}
}
