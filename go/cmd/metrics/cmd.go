// Package metrics exposes `core ai metrics`, for example:
//
//	core ai metrics --since 7d
//	core ai metrics --json
package metrics

import (
	"cmp"
	"slices"
	"time"

	"dappco.re/go"
	"dappco.re/go/ai/ai"
	"dappco.re/go/cli/pkg/cli"
)

// MetricsCommandOptions{SinceWindow: 168 * time.Hour, JSONOutput: true} captures one `core ai metrics` invocation.
type MetricsCommandOptions struct {
	SinceWindow time.Duration
	JSONOutput  bool
}

// core ai metrics --since 7d
// core ai metrics --json
func AddMetricsCommand(c *core.Core) core.Result {
	return RegisterMetricsCommand(c, "metrics")
}

func RegisterMetricsCommand(c *core.Core, path string) core.Result {
	if c.Command(path).OK {
		return core.Ok(nil)
	}
	return c.Command(path, metricsCommand())
}

func metricsCommand() core.Command {
	return core.Command{
		Description: cli.T("cmd.ai.metrics.long"),
		Flags: core.NewOptions(
			core.Option{Key: "since", Value: "7d"},
			core.Option{Key: "json", Value: false},
		),
		Action: func(opts core.Options) core.Result {
			sinceInput := opts.String("since")
			if sinceInput == "" {
				sinceInput = "7d"
			}
			durationResult := parseSinceDuration(sinceInput)
			if !durationResult.OK {
				return durationResult
			}
			return runMetrics(MetricsCommandOptions{
				SinceWindow: durationResult.Value.(time.Duration),
				JSONOutput:  opts.Bool("json"),
			})
		},
	}
}

func runMetrics(options MetricsCommandOptions) core.Result {
	eventsResult := ai.ReadEvents(time.Now().Add(-options.SinceWindow))
	if !eventsResult.OK {
		return core.Fail(cli.WrapVerb(core.NewError(eventsResult.Error()), "read", "metrics"))
	}
	events := eventsResult.Value.([]ai.Event)

	summary := ai.Summary(events)
	if options.JSONOutput {
		outputResult := marshalMetricsSummaryJSON(summary)
		if !outputResult.OK {
			return core.Fail(cli.Wrap(core.NewError(outputResult.Error()), "marshal metrics JSON"))
		}
		output := outputResult.Value.([]byte)
		cli.Text(string(output))
		return core.Ok(nil)
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
		cli.Text(cli.T("cmd.ai.metrics.none_found"))
	}

	return core.Ok(nil)
}

// parseSinceDuration("7d") returns 168 hours for the default metrics window shorthand.
func parseSinceDuration(input string) core.Result {
	trimmed := core.Trim(input)
	if trimmed == "" {
		return core.Fail(core.E("metrics", "invalid duration: "+input, nil))
	}

	if duration, err := time.ParseDuration(trimmed); err == nil {
		if duration <= 0 {
			return core.Fail(core.E("metrics", "duration must be positive: "+input, nil))
		}
		return core.Ok(duration)
	}

	if len(trimmed) < 2 {
		return core.Fail(core.E("metrics", "invalid duration: "+input, nil))
	}

	unit := trimmed[len(trimmed)-1]
	value := trimmed[:len(trimmed)-1]

	n, ok := parseShorthandDurationValue(value)
	if !ok {
		return core.Fail(core.E("metrics", "invalid duration: "+input, nil))
	}
	if n <= 0 {
		return core.Fail(core.E("metrics", "duration must be positive: "+input, nil))
	}

	switch unit {
	case 'd':
		return core.Ok(time.Duration(n) * 24 * time.Hour)
	default:
		return core.Fail(core.E("metrics", "invalid duration: "+input, nil))
	}
}

func parseShorthandDurationValue(value string) (int, bool) {
	if value == "" {
		return 0, false
	}

	maxInt := int(^uint(0) >> 1)
	n := 0
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		digit := int(c - '0')
		if n > (maxInt-digit)/10 {
			return 0, false
		}
		n = n*10 + digit
	}
	return n, true
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

func marshalMetricsSummaryJSON(summary map[string]any) core.Result {
	return core.Ok([]byte(core.JSONMarshalString(summary)))
}
