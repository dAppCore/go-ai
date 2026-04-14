package security

import (
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/ai/ai"
	"dappco.re/go/core/i18n"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var (
	scanTool string
)

func addScanCommand(parent *cli.Command) {
	cmd := &cli.Command{
		Use:   "scan",
		Short: i18n.T("cmd.security.scan.short"),
		Long:  i18n.T("cmd.security.scan.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runScan()
		},
	}

	cmd.Flags().StringVar(&securityRegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&securityRepo, "repo", "", i18n.T("cmd.security.flag.repo"))
	cmd.Flags().StringVar(&securitySeverity, "severity", "", i18n.T("cmd.security.flag.severity"))
	cmd.Flags().StringVar(&scanTool, "tool", "", i18n.T("cmd.security.scan.flag.tool"))
	cmd.Flags().BoolVar(&securityJSON, "json", false, i18n.T("common.flag.json"))
	cmd.Flags().StringVar(&securityTarget, "target", "", i18n.T("cmd.security.flag.target"))

	parent.AddCommand(cmd)
}

// ScanAlert represents a code scanning alert for output.
type ScanAlert struct {
	Repo        string `json:"repo"`
	Severity    string `json:"severity"`
	RuleID      string `json:"rule_id"`
	Tool        string `json:"tool"`
	Path        string `json:"path"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Message     string `json:"message"`
}

func runScan() error {
	if err := checkGitHubCLI(); err != nil {
		return err
	}

	targets, err := resolveSecurityTargets(securityRegistryPath, securityRepo, securityTarget)
	if err != nil {
		return err
	}

	var allAlerts []ScanAlert
	summary := &AlertSummary{}

	for _, target := range targets {
		targetAlerts, err := collectScanAlerts(target)
		if err != nil {
			if securityTarget != "" {
				return err
			}
			cli.Print("%s %s: %v\n", cli.WarningStyle.Render(">>"), target.FullName, err)
			continue
		}

		for _, alert := range targetAlerts {
			summary.Add(alert.Severity)
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	// Record metrics
	recordedRepo := ""
	recordedTarget := ""
	if securityTarget != "" {
		recordedRepo = securityTarget
		recordedTarget = securityTarget
	}
	_ = ai.Record(ai.Event{
		Type:      "security.scan",
		Timestamp: time.Now(),
		Repo:      recordedRepo,
		Data: map[string]any{
			"target":   recordedTarget,
			"total":    summary.Total,
			"critical": summary.Critical,
			"high":     summary.High,
			"medium":   summary.Medium,
			"low":      summary.Low,
		},
	})

	if securityJSON {
		cli.Text(core.JSONMarshalString(allAlerts))
		return nil
	}

	// Print summary
	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Code Scanning", securityTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return nil
	}

	// Print table
	for _, alert := range allAlerts {
		sevStyle := severityStyle(alert.Severity)

		location := core.Sprintf("%s:%d", alert.Path, alert.Line)

		cli.Print("%-16s %s  %-20s %-40s %s\n",
			cli.ValueStyle.Render(alert.Repo),
			sevStyle.Render(core.Sprintf("%-8s", alert.Severity)),
			alert.RuleID,
			location,
			cli.DimStyle.Render(alert.Tool),
		)
	}
	cli.Blank()

	return nil
}

func collectScanAlerts(target SecurityTarget) ([]ScanAlert, error) {
	alerts, err := fetchCodeScanningAlerts(target.FullName)
	if err != nil {
		return nil, err
	}

	var allAlerts []ScanAlert
	for _, alert := range alerts {
		if alert.State != "open" {
			continue
		}
		if scanTool != "" && alert.Tool.Name != scanTool {
			continue
		}
		severity := alert.Rule.Severity
		if severity == "" {
			severity = "medium"
		}
		if !filterBySeverity(severity, securitySeverity) {
			continue
		}

		allAlerts = append(allAlerts, ScanAlert{
			Repo:        target.DisplayName,
			Severity:    severity,
			RuleID:      alert.Rule.ID,
			Tool:        alert.Tool.Name,
			Path:        alert.MostRecentInstance.Location.Path,
			Line:        alert.MostRecentInstance.Location.StartLine,
			Description: alert.Rule.Description,
			Message:     alert.MostRecentInstance.Message.Text,
		})
	}
	return allAlerts, nil
}
