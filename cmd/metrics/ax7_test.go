package metrics

import (
	"time"

	. "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func TestMetrics_DurationFlagValue_String_Good(t *T) {
	value := 2 * time.Hour
	flag := &sinceDurationFlagValue{target: &value}
	got := flag.String()

	AssertEqual(t, "2h0m0s", got)
}

func TestMetrics_DurationFlagValue_String_Bad(t *T) {
	flag := &sinceDurationFlagValue{}
	got := flag.String()
	want := ""

	AssertEqual(t, want, got)
}

func TestMetrics_DurationFlagValue_String_Ugly(t *T) {
	var flag *sinceDurationFlagValue
	got := flag.String()
	want := ""

	AssertEqual(t, want, got)
}

func TestMetrics_DurationFlagValue_Set_Good(t *T) {
	value := time.Duration(0)
	flag := &sinceDurationFlagValue{target: &value}
	err := flag.Set("2d")

	AssertNoError(t, err)
	AssertEqual(t, 48*time.Hour, value)
}

func TestMetrics_DurationFlagValue_Set_Bad(t *T) {
	value := time.Hour
	flag := &sinceDurationFlagValue{target: &value}
	err := flag.Set("bad")

	AssertError(t, err)
	AssertEqual(t, time.Hour, value)
}

func TestMetrics_DurationFlagValue_Set_Ugly(t *T) {
	value := time.Hour
	flag := &sinceDurationFlagValue{target: &value}
	err := flag.Set("0s")

	AssertError(t, err)
	AssertEqual(t, time.Hour, value)
}

func TestMetrics_DurationFlagValue_Type_Good(t *T) {
	value := time.Minute
	flag := &sinceDurationFlagValue{target: &value}
	got := flag.Type()

	AssertEqual(t, "duration", got)
}

func TestMetrics_DurationFlagValue_Type_Bad(t *T) {
	flag := &sinceDurationFlagValue{}
	got := flag.Type()
	want := "duration"

	AssertEqual(t, want, got)
}

func TestMetrics_DurationFlagValue_Type_Ugly(t *T) {
	var flag *sinceDurationFlagValue
	got := flag.Type()
	want := "duration"

	AssertEqual(t, want, got)
}

func TestMetrics_AddMetricsCommand_Good(t *T) {
	root := &cli.Command{Use: "core"}
	AddMetricsCommand(root)
	cmd, _, err := root.Find([]string{"metrics"})

	AssertNoError(t, err)
	AssertEqual(t, "metrics", cmd.Name())
}

func TestMetrics_AddMetricsCommand_Bad(t *T) {
	root := &cli.Command{Use: "core"}
	AddMetricsCommand(root)
	AddMetricsCommand(root)

	AssertLen(t, root.Commands(), 1)
	AssertEqual(t, "metrics", root.Commands()[0].Name())
}

func TestMetrics_AddMetricsCommand_Ugly(t *T) {
	first := &cli.Command{Use: "core"}
	second := &cli.Command{Use: "core"}
	AddMetricsCommand(first)
	AddMetricsCommand(second)

	AssertLen(t, first.Commands(), 1)
	AssertLen(t, second.Commands(), 1)
}
