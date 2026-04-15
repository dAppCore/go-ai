// Package metrics exposes `core ai metrics`.
package metrics

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"dappco.re/go/ai/ai"
	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	coreerr "dappco.re/go/core/log"
	"forge.lthn.ai/core/cli/pkg/cli"
)

// MetricsCommandOptions{SinceWindow: 168 * time.Hour, JSONOutput: true} captures one
// `core ai metrics` invocation.
type MetricsCommandOptions struct {
	SinceWindow time.Duration
	JSONOutput  bool
}

// core ai metrics --since 7d
func AddMetricsCommand(parent *cli.Command) {
	if commandExists(parent, "metrics") {
		return
	}

	options := &MetricsCommandOptions{
		SinceWindow: 168 * time.Hour,
	}

	metricsCommand := &cli.Command{
		Use:   "metrics",
		Short: i18n.T("cmd.ai.metrics.short"),
		Long:  i18n.T("cmd.ai.metrics.long"),
		RunE: func(cmd *cli.Command, args []string) error {
			return runMetrics(*options)
		},
	}

	metricsCommand.Flags().Var(&sinceDurationFlagValue{target: &options.SinceWindow}, "since", i18n.T("cmd.ai.metrics.flag.since"))
	metricsCommand.Flags().BoolVar(&options.JSONOutput, "json", false, i18n.T("common.flag.json"))

	parent.AddCommand(metricsCommand)
}

func commandExists(parent *cli.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}

func runMetrics(options MetricsCommandOptions) error {
	events, err := ai.ReadEvents(time.Now().Add(-options.SinceWindow))
	if err != nil {
		return cli.WrapVerb(err, "read", "metrics")
	}

	summary := ai.Summary(events)
	if options.JSONOutput {
		output, err := marshalMetricsSummaryJSON(summary)
		if err != nil {
			return cli.Wrap(err, "marshal metrics JSON")
		}
		cli.Text(string(output))
		return nil
	}

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render("Period:"), formatDurationShort(options.SinceWindow))
	cli.Print("%s %d\n", cli.DimStyle.Render("Total events:"), len(events))
	cli.Blank()

	if byType := summaryCountPairs(summary, "by_type"); len(byType) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("By type:"))
		for _, entry := range byType {
			cli.Print("  %-30s %v\n", entry["key"], entry["count"])
		}
		cli.Blank()
	}

	if byRepo := summaryCountPairs(summary, "by_repo"); len(byRepo) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("By repo:"))
		for _, entry := range byRepo {
			cli.Print("  %-30s %v\n", entry["key"], entry["count"])
		}
		cli.Blank()
	}

	if byAgent := summaryCountPairs(summary, "by_agent"); len(byAgent) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("By agent:"))
		for _, entry := range byAgent {
			cli.Print("  %-30s %v\n", entry["key"], entry["count"])
		}
		cli.Blank()
	}

	if recent, ok := summary["recent"].([]ai.Event); ok && len(recent) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("Recent events:"))
		for _, event := range recent {
			cli.Print("  %-20s %-24s %-20s %-20s\n",
				event.Timestamp.Format(time.RFC3339),
				event.Type,
				event.AgentID,
				event.Repo,
			)
		}
		cli.Blank()
	}

	if len(events) == 0 {
		cli.Text(i18n.T("cmd.ai.metrics.none_found"))
	}

	return nil
}

// parseSinceDuration("168h") parses the metrics window and keeps "7d" compatibility for older callers.
func parseSinceDuration(input string) (time.Duration, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, coreerr.E("metrics", "invalid duration: "+input, nil)
	}

	if duration, err := time.ParseDuration(trimmed); err == nil {
		if duration <= 0 {
			return 0, coreerr.E("metrics", "duration must be positive: "+input, nil)
		}
		return duration, nil
	}

	if len(trimmed) < 2 {
		return 0, coreerr.E("metrics", "invalid duration: "+input, nil)
	}

	unit := trimmed[len(trimmed)-1]
	value := trimmed[:len(trimmed)-1]

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, coreerr.E("metrics", "invalid duration: "+input, nil)
	}
	if n <= 0 {
		return 0, coreerr.E("metrics", "duration must be positive: "+input, nil)
	}

	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	default:
		return 0, coreerr.E("metrics", "invalid duration: "+input, nil)
	}
}

func formatDurationShort(duration time.Duration) string {
	switch {
	case duration == 0:
		return "0s"
	case duration%time.Hour == 0:
		return core.Sprintf("%dh", duration/time.Hour)
	case duration%time.Minute == 0:
		return core.Sprintf("%dm", duration/time.Minute)
	default:
		return duration.String()
	}
}

type sinceDurationFlagValue struct {
	target *time.Duration
}

func (v *sinceDurationFlagValue) String() string {
	if v == nil || v.target == nil {
		return ""
	}
	return v.target.String()
}

func (v *sinceDurationFlagValue) Set(input string) error {
	duration, err := parseSinceDuration(input)
	if err != nil {
		return err
	}
	*v.target = duration
	return nil
}

func (v *sinceDurationFlagValue) Type() string {
	return "duration"
}

func summaryCountPairs(summary map[string]any, key string) []map[string]any {
	counts, ok := summary[key].(map[string]int)
	if !ok || len(counts) == 0 {
		return nil
	}

	type entry struct {
		key   string
		count int
	}

	entries := make([]entry, 0, len(counts))
	for k, v := range counts {
		entries = append(entries, entry{key: k, count: v})
	}

	slices.SortFunc(entries, func(a, b entry) int {
		if result := cmp.Compare(b.count, a.count); result != 0 {
			return result
		}
		return cmp.Compare(a.key, b.key)
	})

	result := make([]map[string]any, len(entries))
	for i, entry := range entries {
		result[i] = map[string]any{"key": entry.key, "count": entry.count}
	}
	return result
}

func marshalMetricsSummaryJSON(summary map[string]any) ([]byte, error) {
	return json.Marshal(summary)
}
