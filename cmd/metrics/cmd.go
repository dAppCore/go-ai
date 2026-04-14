// Package metrics implements the metrics viewing command.
package metrics

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"dappco.re/go/core/ai/ai"
	"dappco.re/go/core/i18n"
	coreerr "dappco.re/go/core/log"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var (
	metricsSince string
	metricsJSON  bool

	metricsFlagsOnce sync.Once
)

var metricsCmd = &cli.Command{
	Use:   "metrics",
	Short: i18n.T("cmd.ai.metrics.short"),
	Long:  i18n.T("cmd.ai.metrics.long"),
	RunE: func(cmd *cli.Command, args []string) error {
		return runMetrics()
	},
}

func initMetricsFlags() {
	metricsFlagsOnce.Do(func() {
		metricsCmd.Flags().StringVar(&metricsSince, "since", "168h", i18n.T("cmd.ai.metrics.flag.since"))
		metricsCmd.Flags().BoolVar(&metricsJSON, "json", false, i18n.T("common.flag.json"))
	})
}

// AddMetricsCommand adds the 'metrics' command to the parent.
func AddMetricsCommand(parent *cli.Command) {
	initMetricsFlags()
	if hasCommand(parent, metricsCmd.Name()) {
		return
	}
	parent.AddCommand(metricsCmd)
}

func hasCommand(parent *cli.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}

func runMetrics() error {
	since, err := parseSinceDuration(metricsSince)
	if err != nil {
		return cli.Err("invalid --since value %q: %v", metricsSince, err)
	}

	events, err := ai.ReadEvents(time.Now().Add(-since))
	if err != nil {
		return cli.WrapVerb(err, "read", "metrics")
	}

	summary := ai.Summary(events)
	if metricsJSON {
		output, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return cli.Wrap(err, "marshal metrics JSON")
		}
		cli.Text(string(output))
		return nil
	}

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render("Period:"), metricsSince)
	total, _ := summary["total"].(int)
	cli.Print("%s %d\n", cli.DimStyle.Render("Total events:"), total)
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
		cli.Print("%s\n", cli.DimStyle.Render("By contributor:"))
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
		return 0, coreerr.E("metrics.parseSinceDuration", "invalid duration: "+input, nil)
	}

	if duration, err := time.ParseDuration(trimmed); err == nil {
		if duration <= 0 {
			return 0, coreerr.E("metrics.parseSinceDuration", "duration must be positive: "+input, nil)
		}
		return duration, nil
	}

	if len(trimmed) < 2 {
		return 0, coreerr.E("metrics.parseSinceDuration", "invalid duration: "+input, nil)
	}

	unit := trimmed[len(trimmed)-1]
	value := trimmed[:len(trimmed)-1]

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, coreerr.E("metrics.parseSinceDuration", "invalid duration: "+input, nil)
	}
	if n <= 0 {
		return 0, coreerr.E("metrics.parseSinceDuration", "duration must be positive: "+input, nil)
	}

	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	default:
		return 0, coreerr.E("metrics.parseSinceDuration", "invalid duration: "+input, nil)
	}
}

func summaryCountPairs(summary map[string]any, key string) []map[string]any {
	if sorted, ok := summary[key+"_sorted"].([]map[string]any); ok {
		return sorted
	}
	if pairs, ok := summary[key].([]map[string]any); ok {
		return pairs
	}
	return nil
}
