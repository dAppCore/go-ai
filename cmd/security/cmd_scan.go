package security

import (
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"forge.lthn.ai/core/cli/pkg/cli"
)

func addScanCommand(parent *cli.Command) {
	commandOptions := &ScanCommandOptions{}

	cmd := &cli.Command{
		Use:   "scan",
		Short: i18n.T("cmd.security.scan.short"),
		Long:  i18n.T("cmd.security.scan.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runScan(*commandOptions)
		},
	}

	cmd.Flags().StringVar(&commandOptions.Selection.RegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&commandOptions.Selection.RepositoryName, "repo", "", i18n.T("cmd.security.flag.repo"))
	cmd.Flags().StringVar(&commandOptions.Selection.SeverityFilter, "severity", "", i18n.T("cmd.security.flag.severity"))
	cmd.Flags().StringVar(&commandOptions.ToolName, "tool", "", i18n.T("cmd.security.scan.flag.tool"))
	cmd.Flags().BoolVar(&commandOptions.Selection.JSONOutput, "json", false, i18n.T("common.flag.json"))
	cmd.Flags().StringVar(&commandOptions.Selection.ExternalTarget, "target", "", i18n.T("cmd.security.flag.target"))

	parent.AddCommand(cmd)
}

// ScanAlert is the normalised row emitted by `core security scan --json`.
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

func runScan(commandOptions ScanCommandOptions) error {
	startedAt := time.Now()

	targets, err := resolveSecurityTargets(commandOptions.Selection.RegistryPath, commandOptions.Selection.RepositoryName, commandOptions.Selection.ExternalTarget)
	if err != nil {
		return err
	}

	if err := checkGitHubCLI(); err != nil {
		return err
	}

	var allAlerts []ScanAlert
	summary := &AlertSummary{}
	targetErrors := map[string]error{}

	for _, target := range targets {
		targetAlerts, err := collectScanAlerts(target, commandOptions)
		if err != nil {
			targetErrors[target.FullName] = err
			continue
		}

		for _, alert := range targetAlerts {
			summary.Add(alert.Severity)
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	if err := combineSecurityTargetErrors("security scan", targetErrors); err != nil {
		return err
	}

	recordedRepo := metricRepositoryForTargets(targets)
	recordedTarget := recordedRepo
	recordSecurityMetricsEvent(buildSecurityMetricsEvent("security.scan", startedAt, recordedRepo, map[string]any{
		"target":   recordedTarget,
		"total":    summary.Total,
		"critical": summary.Critical,
		"high":     summary.High,
		"medium":   summary.Medium,
		"low":      summary.Low,
	}))

	if commandOptions.Selection.JSONOutput {
		cli.Text(core.JSONMarshalString(allAlerts))
		return nil
	}

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Code Scanning", commandOptions.Selection.ExternalTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return nil
	}

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

func collectScanAlerts(target SecurityTarget, commandOptions ScanCommandOptions) ([]ScanAlert, error) {
	alerts, err := fetchCodeScanningAlerts(target.FullName)
	if err != nil {
		return nil, err
	}

	var allAlerts []ScanAlert
	for _, alert := range alerts {
		if alert.State != "open" {
			continue
		}
		if commandOptions.ToolName != "" && alert.Tool.Name != commandOptions.ToolName {
			continue
		}
		severity := alert.Rule.Severity
		if severity == "" {
			severity = "medium"
		}
		if !filterBySeverity(severity, commandOptions.Selection.SeverityFilter) {
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
