package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Helpers ---

// setupBenchMetricsDir overrides the metrics directory to a temp dir for benchmarks.
// Returns a cleanup function to restore the original.
func setupBenchMetricsDir(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	// Override HOME so metricsDir() resolves to our temp dir
	origHome := os.Getenv("HOME")
	tmpHome := b.TempDir()
	// Create the metrics path under the fake HOME
	metricsPath := filepath.Join(tmpHome, ".core", "ai", "metrics")
	if err := os.MkdirAll(metricsPath, 0o755); err != nil {
		b.Fatalf("Failed to create metrics dir: %v", err)
	}
	os.Setenv("HOME", tmpHome)
	b.Cleanup(func() {
		os.Setenv("HOME", origHome)
	})
	_ = dir
	return metricsPath
}

// seedEvents writes n events to the metrics directory for the current day.
func seedEvents(b *testing.B, n int) {
	b.Helper()
	now := time.Now()
	for i := range n {
		ev := Event{
			Type:      fmt.Sprintf("type-%d", i%10),
			Timestamp: now.Add(-time.Duration(i) * time.Millisecond),
			AgentID:   fmt.Sprintf("agent-%d", i%5),
			Repo:      fmt.Sprintf("repo-%d", i%3),
			Data:      map[string]any{"i": i, "tool": "bench_tool"},
		}
		if err := Record(ev); err != nil {
			b.Fatalf("Failed to record event %d: %v", i, err)
		}
	}
}

// --- Benchmarks ---

// BenchmarkMetricsRecord benchmarks writing individual metric events.
func BenchmarkMetricsRecord(b *testing.B) {
	setupBenchMetricsDir(b)

	now := time.Now()
	b.ResetTimer()

	for i := range b.N {
		ev := Event{
			Type:      "bench_record",
			Timestamp: now,
			AgentID:   "bench-agent",
			Repo:      "bench-repo",
			Data:      map[string]any{"i": i},
		}
		if err := Record(ev); err != nil {
			b.Fatalf("Record failed at iteration %d: %v", i, err)
		}
	}
}

// BenchmarkMetricsRecord_Parallel benchmarks concurrent metric recording.
func BenchmarkMetricsRecord_Parallel(b *testing.B) {
	setupBenchMetricsDir(b)

	now := time.Now()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ev := Event{
				Type:      "bench_parallel",
				Timestamp: now,
				AgentID:   "bench-agent",
				Repo:      "bench-repo",
				Data:      map[string]any{"i": i},
			}
			if err := Record(ev); err != nil {
				b.Fatalf("Parallel Record failed: %v", err)
			}
			i++
		}
	})
}

// BenchmarkMetricsQuery_10K benchmarks querying 10K events.
func BenchmarkMetricsQuery_10K(b *testing.B) {
	setupBenchMetricsDir(b)
	seedEvents(b, 10_000)

	since := time.Now().Add(-24 * time.Hour)
	b.ResetTimer()

	for range b.N {
		events, err := ReadEvents(since)
		if err != nil {
			b.Fatalf("ReadEvents failed: %v", err)
		}
		if len(events) < 10_000 {
			b.Fatalf("Expected at least 10K events, got %d", len(events))
		}
	}
}

// BenchmarkMetricsQuery_50K benchmarks querying 50K events.
func BenchmarkMetricsQuery_50K(b *testing.B) {
	setupBenchMetricsDir(b)
	seedEvents(b, 50_000)

	since := time.Now().Add(-24 * time.Hour)
	b.ResetTimer()

	for range b.N {
		events, err := ReadEvents(since)
		if err != nil {
			b.Fatalf("ReadEvents failed: %v", err)
		}
		if len(events) < 50_000 {
			b.Fatalf("Expected at least 50K events, got %d", len(events))
		}
	}
}

// BenchmarkMetricsSummary_10K benchmarks summarising 10K events.
func BenchmarkMetricsSummary_10K(b *testing.B) {
	setupBenchMetricsDir(b)
	seedEvents(b, 10_000)

	since := time.Now().Add(-24 * time.Hour)
	events, err := ReadEvents(since)
	if err != nil {
		b.Fatalf("ReadEvents failed: %v", err)
	}

	b.ResetTimer()
	for range b.N {
		summary := Summary(events)
		if summary["total"].(int) < 10_000 {
			b.Fatalf("Expected total >= 10K, got %d", summary["total"].(int))
		}
	}
}

// BenchmarkMetricsRecordAndQuery benchmarks the full write-then-read cycle at 10K scale.
func BenchmarkMetricsRecordAndQuery(b *testing.B) {
	setupBenchMetricsDir(b)

	now := time.Now()

	// Write 10K events
	for i := range 10_000 {
		ev := Event{
			Type:      fmt.Sprintf("type-%d", i%10),
			Timestamp: now,
			AgentID:   "bench",
			Repo:      "bench-repo",
		}
		if err := Record(ev); err != nil {
			b.Fatalf("Record failed: %v", err)
		}
	}

	since := now.Add(-24 * time.Hour)
	b.ResetTimer()

	for range b.N {
		events, err := ReadEvents(since)
		if err != nil {
			b.Fatalf("ReadEvents failed: %v", err)
		}
		_ = Summary(events)
	}
}

// --- Unit tests for metrics at scale ---

// TestMetricsRecordAndRead_10K_Good writes 10K events and reads them back.
func TestMetricsRecordAndRead_10K_Good(t *testing.T) {
	// Override HOME to temp dir
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	metricsPath := filepath.Join(tmpHome, ".core", "ai", "metrics")
	if err := os.MkdirAll(metricsPath, 0o755); err != nil {
		t.Fatalf("Failed to create metrics dir: %v", err)
	}
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
	})

	now := time.Now()
	const n = 10_000

	// Write events
	for i := range n {
		ev := Event{
			Type:      fmt.Sprintf("type-%d", i%10),
			Timestamp: now.Add(-time.Duration(i) * time.Millisecond),
			AgentID:   fmt.Sprintf("agent-%d", i%5),
			Repo:      fmt.Sprintf("repo-%d", i%3),
			Data:      map[string]any{"index": i},
		}
		if err := Record(ev); err != nil {
			t.Fatalf("Record failed at %d: %v", i, err)
		}
	}

	// Read back
	since := now.Add(-24 * time.Hour)
	events, err := ReadEvents(since)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if len(events) != n {
		t.Errorf("Expected %d events, got %d", n, len(events))
	}

	// Summarise
	summary := Summary(events)
	total, ok := summary["total"].(int)
	if !ok || total != n {
		t.Errorf("Expected total %d, got %v", n, summary["total"])
	}

	// Verify aggregation counts
	byType, ok := summary["by_type"].([]map[string]any)
	if !ok || len(byType) == 0 {
		t.Fatal("Expected non-empty by_type")
	}
	// Each of 10 types should have n/10 = 1000 events
	for _, entry := range byType {
		count, _ := entry["count"].(int)
		if count != 1000 {
			t.Errorf("Expected count 1000 for type %v, got %d", entry["key"], count)
		}
	}
}
