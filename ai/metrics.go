// Metrics helpers for recording and summarising AI and security events.
package ai

import (
	"cmp"
	// Note: AX-6 — goio is structurally required for the stream interface returned by coreio append handles.
	goio "io"
	"slices"
	// Note: AX-6 — syscall is structurally required for intrinsic OS resource metric calls.
	"syscall"
	"time"

	"dappco.re/go"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
)

var metricsWriteLock = core.New().Lock("ai.metrics.write")

const recentEventLimit = 10
const (
	maxMetricsReadWindowDays = 365
	maxMetricsLineBytes      = 1 << 20
	metricsFileMode          = 0o600
	metricsDirMode           = 0o700
)

// ai.Record(ai.Event{Type: "security.scan", Repo: "wailsapp/wails"})
type Event struct {
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	AgentID   string         `json:"agent_id,omitempty"`
	Repo      string         `json:"repo,omitempty"`
	Duration  time.Duration  `json:"duration,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

func metricsDir() (string, error) {
	home := core.Env("CORE_HOME")
	if home == "" {
		home = core.Env("HOME")
	}
	if home == "" {
		home = core.Env("USERPROFILE")
	}
	if home == "" {
		home = metricsDirHomeEnv()
	}
	if home == "" {
		return "", core.E("ai.metricsDir", "resolve metrics home directory", nil)
	}
	return core.JoinPath(home, ".core", "ai", "metrics"), nil
}

func metricsDirHomeEnv() string {
	if home, ok := syscall.Getenv("DIR_HOME"); ok && home != "" {
		return home
	}
	return core.Env("DIR_HOME")
}

func metricsFilePath(dir string, t time.Time) string {
	return core.JoinPath(dir, t.Format("2006-01-02")+".jsonl")
}

// ai.Record(ai.Event{Type: "security.scan", Repo: "wailsapp/wails"})
func Record(event Event) (err error) {
	recordedAt := time.Now()
	if event.Timestamp.IsZero() {
		event.Timestamp = recordedAt
	}

	event.Data = sanitizeMetricsData(event.Data)

	metricsWriteLock.Mutex.Lock()
	defer metricsWriteLock.Mutex.Unlock()

	dir, err := metricsDir()
	if err != nil {
		return coreerr.E("ai", "record event", err)
	}

	if err := coreio.Local.EnsureDir(dir); err != nil {
		return coreerr.E("ai", "record event", err)
	}
	if err := chmodMetricsPath(dir, metricsDirMode); err != nil {
		return coreerr.E("ai", "record event", err)
	}

	path := metricsFilePath(dir, recordedAt)
	file, err := openMetricsEventFile(path)
	if err != nil {
		return coreerr.E("ai", "record event", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = coreerr.E("ai", "record event", closeErr)
		}
	}()

	data := core.JSONMarshal(event)
	if !data.OK {
		if marshalErr, ok := data.Value.(error); ok {
			return coreerr.E("ai", "record event", marshalErr)
		}
		return coreerr.E("ai", "record event", nil)
	}

	if _, err := file.Write(append(data.Value.([]byte), '\n')); err != nil {
		return coreerr.E("ai", "record event", err)
	}

	return nil
}

// events, err := ai.ReadEvents(time.Now().Add(-24 * time.Hour))
func ReadEvents(since time.Time) ([]Event, error) {
	dir, err := metricsDir()
	if err != nil {
		return nil, coreerr.E("ai", "read events", err)
	}

	var events []Event
	now := time.Now()
	since = clampMetricsSince(since, now)

	// Iterate each day from the caller's `since` timestamp to now in the caller's location.
	loc := since.Location()
	scanStart := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, loc)
	today := now.In(loc)
	for day := scanStart; !day.After(today); day = day.AddDate(0, 0, 1) {
		path := metricsFilePath(dir, day)

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

func clampMetricsSince(since, now time.Time) time.Time {
	if since.IsZero() {
		return now.AddDate(0, 0, -maxMetricsReadWindowDays)
	}

	cutoff := now.AddDate(0, 0, -maxMetricsReadWindowDays)
	if since.Before(cutoff) {
		return cutoff
	}
	if since.After(now) {
		return now
	}
	return since
}

func daysScannedFromDate(start, current time.Time) int {
	if current.Before(start) {
		return 0
	}
	return int(current.Sub(start).Hours() / 24)
}

func readMetricsFile(path string, since time.Time) ([]Event, error) {
	if !coreio.Local.Exists(path) {
		return nil, nil
	}

	content, err := coreio.Local.Read(path)
	if err != nil {
		return nil, coreerr.E("ai", "read events", err)
	}

	var events []Event
	for _, line := range core.Split(content, "\n") {
		if len(line) > maxMetricsLineBytes {
			return nil, coreerr.E("ai", "read events", core.E("ai.readMetricsFile", "metrics line exceeds maximum size", nil))
		}

		var event Event
		if unmarshalResult := core.JSONUnmarshalString(line, &event); !unmarshalResult.OK {
			continue // skip malformed lines
		}
		if !event.Timestamp.Before(since) {
			events = append(events, event)
		}
	}
	return events, nil
}

func openMetricsEventFile(path string) (goio.WriteCloser, error) {
	if !coreio.Local.Exists(path) {
		if err := coreio.Local.WriteMode(path, "", metricsFileMode); err != nil {
			return nil, err
		}
	}

	file, err := coreio.Local.Append(path)
	if err != nil {
		return nil, err
	}

	if err := chmodMetricsPath(path, metricsFileMode); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func chmodMetricsPath(path string, mode uint32) error {
	return syscall.Chmod(path, mode)
}

var sensitiveMetricKeys = []string{
	"password",
	"secret",
	"token",
	"api_key",
	"apikey",
	"bearer",
}

func sanitizeMetricsData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return data
	}

	sanitized := make(map[string]any, len(data))
	for key, value := range data {
		if isSensitiveMetricKey(key) {
			continue
		}
		sanitized[key] = sanitizeMetricsValue(value)
	}
	return sanitized
}

func sanitizeMetricsValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeMetricsData(typed)
	case []any:
		sanitized := make([]any, 0, len(typed))
		for _, item := range typed {
			sanitized = append(sanitized, sanitizeMetricsValue(item))
		}
		return sanitized
	default:
		return value
	}
}

func isSensitiveMetricKey(key string) bool {
	lowerKey := core.Lower(key)
	for _, sensitive := range sensitiveMetricKeys {
		if core.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// summary := ai.Summary([]ai.Event{{Type: "build", Repo: "core-php", AgentID: "agent-1"}})
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

	recentEvents := events
	if len(recentEvents) > recentEventLimit {
		recentEvents = recentEvents[len(recentEvents)-recentEventLimit:]
	}
	recentCopy := make([]Event, len(recentEvents))
	for i, event := range recentEvents {
		recentCopy[i] = cloneEvent(event)
	}

	return map[string]any{
		"by_type":  cloneCounts(byTypeCounts),
		"by_repo":  cloneCounts(byRepoCounts),
		"by_agent": cloneCounts(byAgentCounts),
		"recent":   recentCopy,
	}
}

func cloneCounts(counts map[string]int) map[string]int {
	cloned := make(map[string]int, len(counts))
	for key, count := range counts {
		cloned[key] = count
	}
	return cloned
}

func cloneEvent(event Event) Event {
	cloned := event
	if len(event.Data) > 0 {
		cloned.Data = make(map[string]any, len(event.Data))
		for key, value := range event.Data {
			cloned.Data[key] = cloneMetricValue(value)
		}
	}
	return cloned
}

func cloneMetricValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, item := range typed {
			cloned[key] = cloneMetricValue(item)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneMetricValue(item)
		}
		return cloned
	default:
		return value
	}
}
