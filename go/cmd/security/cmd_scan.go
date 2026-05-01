package security

import (
	"time"

	"dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func addScanCommand(parent *cli.Command) {
	commandOptions := &ScanCommandOptions{}

	cmd := &cli.Command{
		Use:   "scan",
		Short: cli.T("cmd.security.scan.short"),
		Long:  cli.T("cmd.security.scan.long"),
		RunE: func(c *cli.Command, args []string) error {
			r := runScan(*commandOptions)
			if !r.OK {
				err, _ := coreResultError(r).(error)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&commandOptions.Selection.RegistryPath, "registry", "", cli.T("common.flag.registry"))
	cmd.Flags().StringVar(&commandOptions.Selection.RepositoryName, "repo", "", cli.T("cmd.security.flag.repo"))
	cmd.Flags().StringVar(&commandOptions.Selection.SeverityFilter, "severity", "", cli.T("cmd.security.flag.severity"))
	cmd.Flags().StringVar(&commandOptions.ToolName, "tool", "", cli.T("cmd.security.scan.flag.tool"))
	cmd.Flags().BoolVar(&commandOptions.Selection.JSONOutput, "json", false, cli.T("common.flag.json"))
	cmd.Flags().StringVar(&commandOptions.Selection.ExternalTarget, "target", "", cli.T("cmd.security.flag.target"))

	parent.AddCommand(cmd)
}

// ScanAlert is the normalised row emitted by `core security scan --json`.
type ScanAlert struct {
	Repo        string `json:"repo"`
	Severity    string `json:"severity"`
	RuleID      string `json:"rule_id"`
	Tool        string `json:"tool"`
	Path        string `json:"\x70ath"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Message     string `json:"message"`
}

func runScan(commandOptions ScanCommandOptions) core.Result {
	startedAt := time.Now()

	targetsResult := resolveSecurityTargets(commandOptions.Selection.RegistryPath, commandOptions.Selection.RepositoryName, commandOptions.Selection.ExternalTarget)
	if !targetsResult.OK {
		return targetsResult
	}
	targets := targetsResult.Value.([]SecurityTarget)

	if r := checkGitHubCLI(); !r.OK {
		return r
	}

	var allAlerts []ScanAlert
	summary := &AlertSummary{}
	targetErrors := map[string]error{}

	for _, target := range targets {
		targetAlertsResult := collectScanAlerts(target, commandOptions)
		if !targetAlertsResult.OK {
			targetErrors[target.FullName], _ = coreResultError(targetAlertsResult).(error)
			continue
		}
		targetAlerts := targetAlertsResult.Value.([]ScanAlert)

		for _, alert := range targetAlerts {
			summary.Add(alert.Severity)
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	if r := combineSecurityTargetErrors("security scan", targetErrors); !r.OK {
		return r
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
		return core.Ok(nil)
	}

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Code Scanning", commandOptions.Selection.ExternalTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return core.Ok(nil)
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

	return core.Ok(nil)
}

func collectScanAlerts(target SecurityTarget, commandOptions ScanCommandOptions) core.Result {
	alertsResult := fetchCodeScanningAlerts(target.FullName)
	if !alertsResult.OK {
		return alertsResult
	}
	alerts := alertsResult.Value.([]CodeScanningAlert)

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
	return core.Ok(allAlerts)
}
