package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	coreio "dappco.re/go/core/io"
)

func withTempHome(t *testing.T) {
	t.Helper()

	originalHome := os.Getenv("HOME")
	originalCoreHome := os.Getenv("CORE_HOME")
	originalDirHome := os.Getenv("DIR_HOME")
	tempHome := t.TempDir()

	metricsPath := filepath.Join(tempHome, ".core", "ai", "metrics")
	if err := coreio.Local.EnsureDir(metricsPath); err != nil {
		t.Fatalf("create metrics dir: %v", err)
	}

	if err := os.Unsetenv("CORE_HOME"); err != nil {
		t.Fatalf("unset CORE_HOME: %v", err)
	}
	if err := os.Unsetenv("DIR_HOME"); err != nil {
		t.Fatalf("unset DIR_HOME: %v", err)
	}
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	t.Cleanup(func() {
		if originalCoreHome == "" {
			if err := os.Unsetenv("CORE_HOME"); err != nil {
				t.Fatalf("restore CORE_HOME: %v", err)
			}
		} else if err := os.Setenv("CORE_HOME", originalCoreHome); err != nil {
			t.Fatalf("restore CORE_HOME: %v", err)
		}
		if originalDirHome == "" {
			if err := os.Unsetenv("DIR_HOME"); err != nil {
				t.Fatalf("restore DIR_HOME: %v", err)
			}
		} else if err := os.Setenv("DIR_HOME", originalDirHome); err != nil {
			t.Fatalf("restore DIR_HOME: %v", err)
		}
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Fatalf("restore HOME: %v", err)
		}
	})
}

func TestRecordAndReadEvents_Good(t *testing.T) {
	withTempHome(t)

	before := time.Now()
	if err := Record(Event{
		Type:    "security.scan",
		AgentID: "agent-1",
		Repo:    "core/go-ai",
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, err := ReadEvents(before.Add(-time.Minute))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "security.scan" {
		t.Fatalf("expected security.scan event, got %s", events[0].Type)
	}
}

func TestReadEvents_Good_SkipsMissingDays(t *testing.T) {
	withTempHome(t)

	dayOne := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	dayThree := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)

	if err := Record(Event{Type: "scan", Timestamp: dayOne, Repo: "core/go-ai"}); err != nil {
		t.Fatalf("Record day one: %v", err)
	}
	if err := Record(Event{Type: "deps", Timestamp: dayThree, Repo: "core/go-rag"}); err != nil {
		t.Fatalf("Record day three: %v", err)
	}

	events, err := ReadEvents(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Timestamp != dayOne || events[1].Timestamp != dayThree {
		t.Fatalf("events not returned in chronological order: %+v", events)
	}
}

func TestSummary_Good(t *testing.T) {
	summary := Summary([]Event{
		{Type: "scan", Repo: "core/go-ai", AgentID: "agent-1", Timestamp: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)},
		{Type: "scan", Repo: "core/go-ai", AgentID: "agent-2", Timestamp: time.Date(2026, 3, 15, 11, 0, 0, 0, time.UTC)},
		{Type: "deps", Repo: "core/go-rag", AgentID: "agent-1", Timestamp: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)},
	})

	byType, ok := summary["by_type"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_type map, got %T", summary["by_type"])
	}
	if byType["scan"] != 2 || byType["deps"] != 1 {
		t.Fatalf("unexpected type counts: %v", byType)
	}

	if _, ok := summary["total"]; ok {
		t.Fatalf("summary should not include total: %+v", summary)
	}

	recent, ok := summary["recent"].([]Event)
	if !ok {
		t.Fatalf("expected recent slice, got %T", summary["recent"])
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent events, got %d", len(recent))
	}
	if recent[0].Type != "scan" || recent[1].AgentID != "agent-2" || recent[2].Repo != "core/go-rag" {
		t.Fatalf("recent events preserve input order: %+v", recent)
	}
}

func TestSummary_Good_TruncatesRecentEvents(t *testing.T) {
	events := make([]Event, 0, 11)
	for i := range 11 {
		events = append(events, Event{
			Type:      "scan",
			Repo:      "core/go-ai",
			AgentID:   "agent-1",
			Timestamp: time.Date(2026, 4, 15, 10, i, 0, 0, time.UTC),
		})
	}

	summary := Summary(events)
	recent, ok := summary["recent"].([]Event)
	if !ok {
		t.Fatalf("expected recent slice, got %T", summary["recent"])
	}
	if len(recent) != 10 {
		t.Fatalf("expected 10 recent events, got %d", len(recent))
	}
	if recent[0].Timestamp != events[1].Timestamp || recent[9].Timestamp != events[10].Timestamp {
		t.Fatalf("recent slice should contain the last 10 events: %+v", recent)
	}
}
