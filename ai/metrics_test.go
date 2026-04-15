package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
)

func withTempMetricsHome(t *testing.T) string {
	t.Helper()

	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", "")
	t.Setenv("DIR_HOME", "")
	t.Setenv("HOME", tempHome)

	metricsPath := filepath.Join(tempHome, ".core", "ai", "metrics")
	if err := coreio.Local.EnsureDir(metricsPath); err != nil {
		t.Fatalf("create metrics dir: %v", err)
	}

	return tempHome
}

func TestMetrics_Record_Good_DefaultsTimestampAndCreatesFile(t *testing.T) {
	withTempMetricsHome(t)

	before := time.Now()
	if err := Record(Event{Type: "security.scan", Repo: "core/go-ai"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, err := ReadEvents(before.Add(-time.Minute))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Timestamp.IsZero() {
		t.Fatal("Record should populate a timestamp when one is not provided")
	}
	if events[0].Type != "security.scan" || events[0].Repo != "core/go-ai" {
		t.Fatalf("unexpected recorded event: %+v", events[0])
	}
}

func TestMetrics_ReadEvents_Bad_SkipsMalformedAndOldLines(t *testing.T) {
	tempHome := withTempMetricsHome(t)

	now := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	dir := core.JoinPath(tempHome, ".core", "ai", "metrics")
	path := metricsFilePath(dir, now)

	content := []byte(
		"{not-json}\n" +
			`{"type":"scan","timestamp":"2026-04-15T08:30:00Z","repo":"core/go-ai"}` + "\n" +
			`{"type":"scan","timestamp":"2026-04-15T10:30:00Z","repo":"core/go-rag"}` + "\n",
	)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write metrics file: %v", err)
	}

	events, err := ReadEvents(now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event after filtering, got %d", len(events))
	}
	if events[0].Repo != "core/go-rag" {
		t.Fatalf("expected the later event to survive filtering, got %+v", events[0])
	}
}

func TestMetrics_Summary_Good_ClonesReturnedMapsAndEvents(t *testing.T) {
	event := Event{
		Type:      "scan",
		Repo:      "core/go-ai",
		AgentID:   "agent-1",
		Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		Data:      map[string]any{"features": 3},
	}

	summary := Summary([]Event{event})

	byType, ok := summary["by_type"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_type map, got %T", summary["by_type"])
	}
	byType["scan"] = 99

	recent, ok := summary["recent"].([]Event)
	if !ok {
		t.Fatalf("expected recent slice, got %T", summary["recent"])
	}
	recent[0].Data["features"] = 99

	fresh := Summary([]Event{event})
	freshByType := fresh["by_type"].(map[string]int)
	if freshByType["scan"] != 1 {
		t.Fatalf("summary counts leaked mutation, got %+v", freshByType)
	}

	freshRecent := fresh["recent"].([]Event)
	if freshRecent[0].Data["features"] != 3 {
		t.Fatalf("summary event data leaked mutation, got %+v", freshRecent[0].Data)
	}
}

func TestMetrics_Summary_Good_CountsByRepoAndAgent(t *testing.T) {
	events := []Event{
		{Type: "scan", Repo: "core/go-ai", AgentID: "agent-1", Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)},
		{Type: "scan", Repo: "core/go-ai", AgentID: "agent-2", Timestamp: time.Date(2026, 4, 15, 10, 5, 0, 0, time.UTC)},
		{Type: "deps", Repo: "core/go-rag", AgentID: "agent-1", Timestamp: time.Date(2026, 4, 15, 10, 10, 0, 0, time.UTC)},
	}

	summary := Summary(events)

	byRepo, ok := summary["by_repo"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_repo map, got %T", summary["by_repo"])
	}
	if byRepo["core/go-ai"] != 2 || byRepo["core/go-rag"] != 1 {
		t.Fatalf("unexpected repo counts: %+v", byRepo)
	}

	byAgent, ok := summary["by_agent"].(map[string]int)
	if !ok {
		t.Fatalf("expected by_agent map, got %T", summary["by_agent"])
	}
	if byAgent["agent-1"] != 2 || byAgent["agent-2"] != 1 {
		t.Fatalf("unexpected agent counts: %+v", byAgent)
	}
}
