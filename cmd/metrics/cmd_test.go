package metrics

import (
	"testing"
	"time"

	core "dappco.re/go"
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
		if r := runMetrics(MetricsCommandOptions{SinceWindow: 24 * time.Hour}); !r.OK {
			t.Fatalf("runMetrics: %s", r.Error())
		}
	})

	for _, want := range []string{"Period:", "Total events:", "By type:", "Recent events:"} {
		if !core.Contains(output, want) {
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
		if r := runMetrics(MetricsCommandOptions{SinceWindow: 24 * time.Hour, JSONOutput: true}); !r.OK {
			t.Fatalf("runMetrics JSON: %s", r.Error())
		}
	})

	var decoded any
	if r := core.JSONUnmarshal([]byte(core.Trim(output)), &decoded); !r.OK {
		t.Fatalf("expected JSON output, got %q", output)
	}
	if !core.Contains(output, `"by_type"`) || !core.Contains(output, `"recent"`) {
		t.Fatalf("JSON output missing expected fields: %q", output)
	}
}

func TestCmdMetrics_runMetrics_Good_PrintsNoEventsMessage(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", "")
	t.Setenv("DIR_HOME", "")
	t.Setenv("HOME", tempHome)

	output := captureStdout(t, func() {
		if r := runMetrics(MetricsCommandOptions{SinceWindow: 24 * time.Hour}); !r.OK {
			t.Fatalf("runMetrics empty: %s", r.Error())
		}
	})

	if !core.Contains(output, i18n.T("cmd.ai.metrics.none_found")) {
		t.Fatalf("empty metrics output %q missing none-found message", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	buf := core.NewBuilder()
	cli.SetStdout(buf)
	defer cli.SetStdout(nil)

	fn()

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

	var decoded any
	if r := core.JSONUnmarshal(got, &decoded); !r.OK {
		t.Fatalf("marshalMetricsSummaryJSON returned invalid JSON: %s", string(got))
	}
	if string(got) != `{"by_type":{"scan":2},"recent":[]}` {
		t.Fatalf("marshalMetricsSummaryJSON = %s, want compact JSON", string(got))
	}
}

// --- AX-7 canonical triplets ---

func TestCmd_DurationFlagValue_String_Good(t *core.T) {
	value := 2 * time.Hour
	flag := &sinceDurationFlagValue{target: &value}
	got := flag.String()

	core.AssertEqual(t, "2h0m0s", got)
}

func TestCmd_DurationFlagValue_String_Bad(t *core.T) {
	flag := &sinceDurationFlagValue{}
	got := flag.String()
	want := ""

	core.AssertEqual(t, want, got)
}

func TestCmd_DurationFlagValue_String_Ugly(t *core.T) {
	var flag *sinceDurationFlagValue
	got := flag.String()
	want := ""

	core.AssertEqual(t, want, got)
}

func TestCmd_DurationFlagValue_Set_Good(t *core.T) {
	value := time.Duration(0)
	flag := &sinceDurationFlagValue{target: &value}
	err := flag.Set("2d")

	core.AssertNoError(t, err)
	core.AssertEqual(t, 48*time.Hour, value)
}

func TestCmd_DurationFlagValue_Set_Bad(t *core.T) {
	value := time.Hour
	flag := &sinceDurationFlagValue{target: &value}
	err := flag.Set("bad")

	core.AssertError(t, err)
	core.AssertEqual(t, time.Hour, value)
}

func TestCmd_DurationFlagValue_Set_Ugly(t *core.T) {
	value := time.Hour
	flag := &sinceDurationFlagValue{target: &value}
	err := flag.Set("0s")

	core.AssertError(t, err)
	core.AssertEqual(t, time.Hour, value)
}

func TestCmd_DurationFlagValue_Type_Good(t *core.T) {
	value := time.Minute
	flag := &sinceDurationFlagValue{target: &value}
	got := flag.Type()

	core.AssertEqual(t, "duration", got)
}

func TestCmd_DurationFlagValue_Type_Bad(t *core.T) {
	flag := &sinceDurationFlagValue{}
	got := flag.Type()
	want := "duration"

	core.AssertEqual(t, want, got)
}

func TestCmd_DurationFlagValue_Type_Ugly(t *core.T) {
	var flag *sinceDurationFlagValue
	got := flag.Type()
	want := "duration"

	core.AssertEqual(t, want, got)
}

func TestCmd_AddMetricsCommand_Good(t *core.T) {
	root := &cli.Command{Use: "core"}
	AddMetricsCommand(root)
	cmd, _, err := root.Find([]string{"metrics"})

	core.AssertNoError(t, err)
	core.AssertEqual(t, "metrics", cmd.Name())
}

func TestCmd_AddMetricsCommand_Bad(t *core.T) {
	root := &cli.Command{Use: "core"}
	AddMetricsCommand(root)
	AddMetricsCommand(root)

	core.AssertLen(t, root.Commands(), 1)
	core.AssertEqual(t, "metrics", root.Commands()[0].Name())
}

func TestCmd_AddMetricsCommand_Ugly(t *core.T) {
	first := &cli.Command{Use: "core"}
	second := &cli.Command{Use: "core"}
	AddMetricsCommand(first)
	AddMetricsCommand(second)

	core.AssertLen(t, first.Commands(), 1)
	core.AssertLen(t, second.Commands(), 1)
}
