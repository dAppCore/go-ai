package metrics

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"dappco.re/go/ai/ai"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/i18n"
)

func TestParseSinceDuration_Good(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"168h", 168 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
	}

	for _, tc := range tests {
		got, err := parseSinceDuration(tc.input)
		if err != nil {
			t.Errorf("parseSinceDuration(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseSinceDuration(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseSinceDuration_Bad(t *testing.T) {
	bad := []string{
		"",    // too short
		"d",   // too short
		"0d",  // non-positive
		"-1d", // negative
		"abc", // non-numeric
		"7x",  // unknown unit
	}

	for _, input := range bad {
		_, err := parseSinceDuration(input)
		if err == nil {
			t.Errorf("parseSinceDuration(%q): expected error, got nil", input)
		}
	}
}

func TestCmdMetrics_parseSinceDuration_Ugly_RejectsZeroSecondDuration(t *testing.T) {
	if _, err := parseSinceDuration("0s"); err == nil {
		t.Fatal("expected parseSinceDuration to reject zero-second durations")
	}
}

func TestAddMetricsCommand_Good_CommandInstancesKeepFlagStateLocal(t *testing.T) {
	firstRoot := &cli.Command{Use: "core"}
	secondRoot := &cli.Command{Use: "core"}

	AddMetricsCommand(firstRoot)
	AddMetricsCommand(secondRoot)

	firstCommand, _, err := firstRoot.Find([]string{"metrics"})
	if err != nil {
		t.Fatalf("find first metrics command: %v", err)
	}
	secondCommand, _, err := secondRoot.Find([]string{"metrics"})
	if err != nil {
		t.Fatalf("find second metrics command: %v", err)
	}

	if err := firstCommand.Flags().Set("since", "24h"); err != nil {
		t.Fatalf("set first --since: %v", err)
	}

	firstSince, err := firstCommand.Flags().GetDuration("since")
	if err != nil {
		t.Fatalf("get first --since: %v", err)
	}
	secondSince, err := secondCommand.Flags().GetDuration("since")
	if err != nil {
		t.Fatalf("get second --since: %v", err)
	}

	if firstSince != 24*time.Hour {
		t.Fatalf("first command since = %v, want %v", firstSince, 24*time.Hour)
	}
	if secondSince != 168*time.Hour {
		t.Fatalf("second command since leaked shared state: got %v, want %v", secondSince, 168*time.Hour)
	}
}

func TestAddMetricsCommand_Good_DoesNotDuplicateCommand(t *testing.T) {
	root := &cli.Command{Use: "core"}

	AddMetricsCommand(root)
	AddMetricsCommand(root)

	commands := root.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected a single metrics command, got %d", len(commands))
	}
	if commands[0].Name() != "metrics" {
		t.Fatalf("expected metrics command, got %s", commands[0].Name())
	}
}

func TestFormatDurationShort_Good(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "zero", in: 0, want: "0s"},
		{name: "hours", in: 48 * time.Hour, want: "48h"},
		{name: "minutes", in: 90 * time.Minute, want: "90m"},
		{name: "mixed", in: 95 * time.Minute, want: "95m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDurationShort(tc.in); got != tc.want {
				t.Fatalf("formatDurationShort(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCmdMetrics_formatDurationShort_Ugly_UsesVerboseDurationForMixedValues(t *testing.T) {
	if got := formatDurationShort(95*time.Minute + 30*time.Second); got != (95*time.Minute + 30*time.Second).String() {
		t.Fatalf("formatDurationShort(mixed) = %q, want verbose duration", got)
	}
}

func TestSummaryCountPairs_Good_SortsByCountThenKey(t *testing.T) {
	summary := map[string]any{
		"by_type": map[string]int{
			"scan":   2,
			"deps":   2,
			"secret": 1,
		},
	}

	got := summaryCountPairs(summary, "by_type")
	if len(got) != 3 {
		t.Fatalf("expected 3 summary rows, got %d", len(got))
	}
	if got[0]["key"] != "deps" || got[1]["key"] != "scan" || got[2]["key"] != "secret" {
		t.Fatalf("unexpected sort order: %#v", got)
	}
}

func TestSummaryCountPairs_Bad_EmptyOrWrongTypeReturnsNil(t *testing.T) {
	if got := summaryCountPairs(map[string]any{}, "missing"); got != nil {
		t.Fatalf("expected nil for missing key, got %#v", got)
	}
	if got := summaryCountPairs(map[string]any{"by_type": []string{"scan"}}, "by_type"); got != nil {
		t.Fatalf("expected nil for wrong type, got %#v", got)
	}
}

func TestCmdMetrics_sinceDurationFlagValue_String_Ugly_NilReceiverReturnsEmpty(t *testing.T) {
	var flag *sinceDurationFlagValue
	if got := flag.String(); got != "" {
		t.Fatalf("nil flag String() = %q, want empty string", got)
	}
}

func TestCmdMetrics_sinceDurationFlagValue_Set_Bad_RejectsInvalidDuration(t *testing.T) {
	value := time.Hour
	flag := &sinceDurationFlagValue{target: &value}
	if err := flag.Set("bad"); err == nil {
		t.Fatal("expected Set to reject invalid durations")
	}
	if value != time.Hour {
		t.Fatalf("Set should not mutate target on failure, got %v", value)
	}
}

func TestRunMetrics_Good_PrintsHumanSummary(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", "")
	t.Setenv("DIR_HOME", "")
	t.Setenv("HOME", tempHome)

	if err := ai.Record(ai.Event{Type: "scan", Repo: "core/go-ai", AgentID: "agent-1"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	output := captureStdout(t, func() {
		if err := runMetrics(MetricsCommandOptions{SinceWindow: 24 * time.Hour}); err != nil {
			t.Fatalf("runMetrics: %v", err)
		}
	})

	for _, want := range []string{"Period:", "Total events:", "By type:", "Recent events:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("human output %q missing %q", output, want)
		}
	}
}

func TestRunMetrics_Bad_PrintsJSONSummary(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", "")
	t.Setenv("DIR_HOME", "")
	t.Setenv("HOME", tempHome)

	if err := ai.Record(ai.Event{Type: "deps", Repo: "core/go-rag", AgentID: "agent-2"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	output := captureStdout(t, func() {
		if err := runMetrics(MetricsCommandOptions{SinceWindow: 24 * time.Hour, JSONOutput: true}); err != nil {
			t.Fatalf("runMetrics JSON: %v", err)
		}
	})

	if !json.Valid(bytes.TrimSpace([]byte(output))) {
		t.Fatalf("expected JSON output, got %q", output)
	}
	if !strings.Contains(output, `"by_type"`) || !strings.Contains(output, `"recent"`) {
		t.Fatalf("JSON output missing expected fields: %q", output)
	}
}

func TestCmdMetrics_runMetrics_Good_PrintsNoEventsMessage(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", "")
	t.Setenv("DIR_HOME", "")
	t.Setenv("HOME", tempHome)

	output := captureStdout(t, func() {
		if err := runMetrics(MetricsCommandOptions{SinceWindow: 24 * time.Hour}); err != nil {
			t.Fatalf("runMetrics empty: %v", err)
		}
	})

	if !strings.Contains(output, i18n.T("cmd.ai.metrics.none_found")) {
		t.Fatalf("empty metrics output %q missing none-found message", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	os.Stdout = originalStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return buf.String()
}

func TestMarshalMetricsSummaryJSON_Good_CompactOutput(t *testing.T) {
	summary := map[string]any{
		"by_type": map[string]int{"scan": 2},
		"recent":  []any{},
	}

	got, err := marshalMetricsSummaryJSON(summary)
	if err != nil {
		t.Fatalf("marshalMetricsSummaryJSON: %v", err)
	}

	if json.Valid(got) == false {
		t.Fatalf("marshalMetricsSummaryJSON returned invalid JSON: %s", string(got))
	}
	if string(got) != `{"by_type":{"scan":2},"recent":[]}` {
		t.Fatalf("marshalMetricsSummaryJSON = %s, want compact JSON", string(got))
	}
}
