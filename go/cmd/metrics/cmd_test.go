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
		result := parseSinceDuration(tc.input)
		if !result.OK {
			t.Errorf("parseSinceDuration(%q): unexpected error: %s", tc.input, result.Error())
			continue
		}
		got := result.Value.(time.Duration)
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
		result := parseSinceDuration(input)
		if result.OK {
			t.Errorf("parseSinceDuration(%q): expected error, got nil", input)
		}
	}
}

func TestCmdMetrics_parseSinceDuration_Ugly_RejectsZeroSecondDuration(t *testing.T) {
	if result := parseSinceDuration("0s"); result.OK {
		t.Fatal("expected parseSinceDuration to reject zero-second durations")
	}
}

func TestAddMetricsCommand_Good_CommandInstancesKeepFlagStateLocal(t *testing.T) {
	firstRoot := core.New()
	secondRoot := core.New()

	if r := AddMetricsCommand(firstRoot); !r.OK {
		t.Fatalf("register first metrics command: %s", r.Error())
	}
	if r := AddMetricsCommand(secondRoot); !r.OK {
		t.Fatalf("register second metrics command: %s", r.Error())
	}

	firstResult := firstRoot.Command("metrics")
	if !firstResult.OK {
		t.Fatalf("find first metrics command: %s", firstResult.Error())
	}
	secondResult := secondRoot.Command("metrics")
	if !secondResult.OK {
		t.Fatalf("find second metrics command: %s", secondResult.Error())
	}
	firstCommand := firstResult.Value.(*core.Command)
	secondCommand := secondResult.Value.(*core.Command)

	firstCommand.Flags.Set("since", "24h")
	firstSince := firstCommand.Flags.String("since")
	secondSince := secondCommand.Flags.String("since")

	if firstSince != "24h" {
		t.Fatalf("first command since = %v, want %v", firstSince, "24h")
	}
	if secondSince != "7d" {
		t.Fatalf("second command since leaked shared state: got %v, want %v", secondSince, "7d")
	}
}

func TestAddMetricsCommand_Good_DoesNotDuplicateCommand(t *testing.T) {
	root := core.New()

	if r := AddMetricsCommand(root); !r.OK {
		t.Fatalf("register metrics command: %s", r.Error())
	}
	if r := AddMetricsCommand(root); !r.OK {
		t.Fatalf("register duplicate metrics command: %s", r.Error())
	}

	commands := root.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected a single metrics command, got %d", len(commands))
	}
	if commands[0] != "metrics" {
		t.Fatalf("expected metrics command, got %s", commands[0])
	}
}

func TestCmd_RegisterMetricsCommand_Good(t *testing.T) {
	root := core.New()
	result := RegisterMetricsCommand(root, "metrics")

	if !result.OK {
		t.Fatalf("RegisterMetricsCommand() error = %s", result.Error())
	}
	if command := root.Command("metrics"); !command.OK {
		t.Fatalf("RegisterMetricsCommand() did not register metrics: %s", command.Error())
	}
}

func TestCmd_RegisterMetricsCommand_Bad(t *testing.T) {
	root := core.New()
	result := RegisterMetricsCommand(root, "/metrics")

	if result.OK {
		t.Fatal("RegisterMetricsCommand() OK = true, want invalid command path failure")
	}
}

func TestCmd_RegisterMetricsCommand_Ugly(t *testing.T) {
	root := core.New()
	first := RegisterMetricsCommand(root, "metrics")
	second := RegisterMetricsCommand(root, "metrics")

	if !first.OK || !second.OK {
		t.Fatalf("RegisterMetricsCommand() idempotent registration failed: first=%s second=%s", first.Error(), second.Error())
	}
	if commands := root.Commands(); len(commands) != 1 || commands[0] != "metrics" {
		t.Fatalf("RegisterMetricsCommand() commands = %#v, want one metrics command", commands)
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

func TestRunMetrics_Good_PrintsHumanSummary(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", "")
	t.Setenv("DIR_HOME", "")
	t.Setenv("HOME", tempHome)

	if result := ai.Record(ai.Event{Type: "scan", Repo: "core/go-ai", AgentID: "agent-1"}); !result.OK {
		t.Fatalf("Record: %s", result.Error())
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

	if result := ai.Record(ai.Event{Type: "deps", Repo: "core/go-rag", AgentID: "agent-2"}); !result.OK {
		t.Fatalf("Record: %s", result.Error())
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

	result := marshalMetricsSummaryJSON(summary)
	if !result.OK {
		t.Fatalf("marshalMetricsSummaryJSON: %s", result.Error())
	}
	got := result.Value.([]byte)

	var decoded any
	if r := core.JSONUnmarshal(got, &decoded); !r.OK {
		t.Fatalf("marshalMetricsSummaryJSON returned invalid JSON: %s", string(got))
	}
	if string(got) != `{"by_type":{"scan":2},"recent":[]}` {
		t.Fatalf("marshalMetricsSummaryJSON = %s, want compact JSON", string(got))
	}
}

// --- AX-7 canonical triplets ---

func TestCmd_AddMetricsCommand_Good(t *core.T) {
	root := core.New()
	r := AddMetricsCommand(root)
	cmd := root.Command("metrics")

	core.AssertTrue(t, r.OK)
	core.AssertTrue(t, cmd.OK)
	core.AssertEqual(t, "metrics", cmd.Value.(*core.Command).Name)
}

func TestCmd_AddMetricsCommand_Bad(t *core.T) {
	root := core.New()
	AddMetricsCommand(root)
	AddMetricsCommand(root)

	core.AssertLen(t, root.Commands(), 1)
	core.AssertEqual(t, "metrics", root.Commands()[0])
}

func TestCmd_AddMetricsCommand_Ugly(t *core.T) {
	first := core.New()
	second := core.New()
	AddMetricsCommand(first)
	AddMetricsCommand(second)

	core.AssertLen(t, first.Commands(), 1)
	core.AssertLen(t, second.Commands(), 1)
}
