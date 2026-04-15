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

func TestMetrics_clampMetricsSince_Good(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	if got := clampMetricsSince(time.Time{}, now); !got.Equal(now.AddDate(0, 0, -maxMetricsReadWindowDays)) {
		t.Fatalf("clampMetricsSince(zero) = %v, want %v", got, now.AddDate(0, 0, -maxMetricsReadWindowDays))
	}

	tooOld := now.AddDate(0, 0, -2*maxMetricsReadWindowDays)
	if got := clampMetricsSince(tooOld, now); !got.Equal(now.AddDate(0, 0, -maxMetricsReadWindowDays)) {
		t.Fatalf("clampMetricsSince(old) = %v, want cutoff %v", got, now.AddDate(0, 0, -maxMetricsReadWindowDays))
	}

	future := now.Add(time.Hour)
	if got := clampMetricsSince(future, now); !got.Equal(now) {
		t.Fatalf("clampMetricsSince(future) = %v, want %v", got, now)
	}
}

func TestMetrics_clampMetricsSince_Bad_RejectsVeryOldTimestamp(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	tooOld := now.Add(-2 * 24 * time.Hour * maxMetricsReadWindowDays)

	got := clampMetricsSince(tooOld, now)
	want := now.AddDate(0, 0, -maxMetricsReadWindowDays)
	if !got.Equal(want) {
		t.Fatalf("clampMetricsSince(%v, %v) = %v, want %v", tooOld, now, got, want)
	}
}

func TestMetrics_clampMetricsSince_Ugly_AllowsFutureClampToNow(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	future := now.Add(3 * time.Hour)

	if got := clampMetricsSince(future, now); !got.Equal(now) {
		t.Fatalf("clampMetricsSince(%v, %v) = %v, want %v", future, now, got, now)
	}
}

func TestMetrics_daysScannedFromDate_Good(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	current := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)

	if got := daysScannedFromDate(start, current); got != 3 {
		t.Fatalf("daysScannedFromDate(%v, %v) = %d, want 3", start, current, got)
	}

	if got := daysScannedFromDate(current, start); got != 0 {
		t.Fatalf("daysScannedFromDate(%v, %v) = %d, want 0", current, start, got)
	}
}

func TestMetrics_daysScannedFromDate_Bad_CurrentBeforeStart(t *testing.T) {
	start := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	current := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	if got := daysScannedFromDate(start, current); got != 0 {
		t.Fatalf("daysScannedFromDate should floor negative windows to 0, got %d", got)
	}
}

func TestMetrics_daysScannedFromDate_Ugly_SameDate(t *testing.T) {
	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	if got := daysScannedFromDate(now, now); got != 0 {
		t.Fatalf("daysScannedFromDate(%v, %v) = %d, want 0", now, now, got)
	}
}

func TestMetrics_sanitizeMetricsData_Good_RemovesSensitiveKeys(t *testing.T) {
	input := map[string]any{
		"api_key":      "keepme",
		"token":        "sensitive",
		"count":        12,
		"nested":       map[string]any{"secret": "x", "safe": "ok", "bearer_token": "shh"},
		"credentials": []any{"a", map[string]any{"Password": "zzz", "role": "svc"}, map[string]any{"not_sensitive": true}},
	}

	got := sanitizeMetricsData(input)

	if _, ok := got["api_key"]; ok {
		t.Fatal("api_key was not sanitized")
	}
	if _, ok := got["token"]; ok {
		t.Fatal("token was not sanitized")
	}

	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested = %T, want map", got["nested"])
	}
	if _, ok := nested["secret"]; ok {
		t.Fatal("nested secret was not sanitized")
	}
	if _, ok := nested["bearer_token"]; ok {
		t.Fatal("nested bearer token was not sanitized")
	}

	creds, ok := got["credentials"].([]any)
	if !ok {
		t.Fatalf("credentials = %T, want []any", got["credentials"])
	}
	if creds[1].(map[string]any)["Password"] != nil {
		t.Fatal("map value with password key was not sanitized")
	}
	if creds[1].(map[string]any)["role"] != "svc" {
		t.Fatalf("unexpected nested map value %v", creds[1])
	}
}

func TestMetrics_sanitizeMetricsData_Bad_NonSensitiveKeysPassThrough(t *testing.T) {
	input := map[string]any{"safe": "value", "count": 9, "nested": map[string]any{"inner": "ok"}}

	got := sanitizeMetricsData(input)
	if got["safe"] != "value" || got["count"] != 9 {
		t.Fatalf("non-sensitive fields were altered: %v", got)
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok || nested["inner"] != "ok" {
		t.Fatalf("nested non-sensitive map was altered: %v", got["nested"])
	}
}

func TestMetrics_sanitizeMetricsData_Ugly_NilInputReturnsNilMap(t *testing.T) {
	if got := sanitizeMetricsData(nil); got != nil {
		t.Fatalf("sanitizeMetricsData(nil) = %v, want nil", got)
	}
}
