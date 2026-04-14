// Package metrics implements the metrics viewing command.
package metrics

import (
	"strconv"
	"sync"
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/ai/ai"
	"dappco.re/go/core/cli/pkg/cli"
	"dappco.re/go/core/i18n"
	coreerr "dappco.re/go/core/log"
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
		metricsCmd.Flags().StringVar(&metricsSince, "since", "7d", i18n.T("cmd.ai.metrics.flag.since"))
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
	since, err := parseDuration(metricsSince)
	if err != nil {
		return cli.Err("invalid --since value %q: %v", metricsSince, err)
	}

	sinceTime := time.Now().Add(-since)
	events, err := ai.ReadEvents(sinceTime)
	if err != nil {
		return cli.WrapVerb(err, "read", "metrics")
	}

	if metricsJSON {
		summary := ai.Summary(events)
		cli.Text(core.JSONMarshalString(summary))
		return nil
	}

	summary := ai.Summary(events)

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render("Period:"), metricsSince)
	total, _ := summary["total"].(int)
	cli.Print("%s %d\n", cli.DimStyle.Render("Total events:"), total)
	cli.Blank()

	// By type
	if byType, ok := summary["by_type"].([]map[string]any); ok && len(byType) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("By type:"))
		for _, entry := range byType {
			cli.Print("  %-30s %v\n", entry["key"], entry["count"])
		}
		cli.Blank()
	}

	// By repo
	if byRepo, ok := summary["by_repo"].([]map[string]any); ok && len(byRepo) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("By repo:"))
		for _, entry := range byRepo {
			cli.Print("  %-30s %v\n", entry["key"], entry["count"])
		}
		cli.Blank()
	}

	// By agent
	if byAgent, ok := summary["by_agent"].([]map[string]any); ok && len(byAgent) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("By contributor:"))
		for _, entry := range byAgent {
			cli.Print("  %-30s %v\n", entry["key"], entry["count"])
		}
		cli.Blank()
	}

	// Recent events
	if recent, ok := summary["events"].([]map[string]any); ok && len(recent) > 0 {
		cli.Print("%s\n", cli.DimStyle.Render("Recent events:"))
		for _, entry := range recent {
			ts, _ := entry["timestamp"].(time.Time)
			agent, _ := entry["agent_id"].(string)
			repo, _ := entry["repo"].(string)
			cli.Print("  %-20s %-24s %-20s %-20s\n",
				ts.Format(time.RFC3339),
				entry["type"],
				agent,
				repo,
			)
		}
		cli.Blank()
	}

	if len(events) == 0 {
		cli.Text(i18n.T("cmd.ai.metrics.none_found"))
	}

	return nil
}

// parseDuration parses a human-friendly duration like "7d", "24h", "30d".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, coreerr.E("metrics.parseDuration", core.Sprintf("invalid duration: %s", s), nil)
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, coreerr.E("metrics.parseDuration", core.Sprintf("invalid duration: %s", s), nil)
	}

	if n <= 0 {
		return 0, coreerr.E("metrics.parseDuration", core.Sprintf("duration must be positive: %s", s), nil)
	}

	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, coreerr.E("metrics.parseDuration", core.Sprintf("unknown unit %c in duration: %s", unit, s), nil)
	}
}
