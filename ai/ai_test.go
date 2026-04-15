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
	tempHome := t.TempDir()

	metricsPath := filepath.Join(tempHome, ".core", "ai", "metrics")
	if err := coreio.Local.EnsureDir(metricsPath); err != nil {
		t.Fatalf("create metrics dir: %v", err)
	}

	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	t.Cleanup(func() {
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

func TestSummary_Good(t *testing.T) {
	summary := Summary([]Event{
		{Type: "scan", Repo: "core/go-ai", AgentID: "agent-1", Timestamp: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)},
		{Type: "scan", Repo: "core/go-ai", AgentID: "agent-2", Timestamp: time.Date(2026, 3, 15, 11, 0, 0, 0, time.UTC)},
		{Type: "deps", Repo: "core/go-rag", AgentID: "agent-1", Timestamp: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)},
	})

	total, ok := summary["total"].(int)
	if !ok || total != 3 {
		t.Fatalf("expected total 3, got %v", summary["total"])
	}

	byType, ok := summary["by_type"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_type map, got %T", summary["by_type"])
	}
	if byType["scan"] != 2 || byType["deps"] != 1 {
		t.Fatalf("unexpected type counts: %v", byType)
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
