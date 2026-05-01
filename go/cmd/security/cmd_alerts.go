package security

import (
	"time"

	"dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/i18n"
)

func addAlertsCommand(parent *cli.Command) {
	selectionOptions := &SecuritySelectionOptions{}

	cmd := &cli.Command{
		Use:   "alerts",
		Short: i18n.T("cmd.security.alerts.short"),
		Long:  i18n.T("cmd.security.alerts.long"),
		RunE: func(c *cli.Command, args []string) error {
			r := runAlerts(*selectionOptions)
			if !r.OK {
				err, _ := coreResultError(r).(error)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&selectionOptions.RegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&selectionOptions.RepositoryName, "repo", "", i18n.T("cmd.security.flag.repo"))
	cmd.Flags().StringVar(&selectionOptions.SeverityFilter, "severity", "", i18n.T("cmd.security.flag.severity"))
	cmd.Flags().BoolVar(&selectionOptions.JSONOutput, "json", false, i18n.T("common.flag.json"))
	cmd.Flags().StringVar(&selectionOptions.ExternalTarget, "target", "", i18n.T("cmd.security.flag.target"))

	parent.AddCommand(cmd)
}

// AlertOutput is the normalised row emitted by `core security alerts --json`.
type AlertOutput struct {
	Repo     string `json:"repo"`
	Severity string `json:"severity"`
	ID       string `json:"id"`
	Package  string `json:"package,omitempty"`
	Version  string `json:"version,omitempty"`
	Location string `json:"location,omitempty"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

func runAlerts(selectionOptions SecuritySelectionOptions) core.Result {
	startedAt := time.Now()

	targetsResult := resolveSecurityTargets(selectionOptions.RegistryPath, selectionOptions.RepositoryName, selectionOptions.ExternalTarget)
	if !targetsResult.OK {
		return targetsResult
	}
	targets := targetsResult.Value.([]SecurityTarget)

	if r := checkGitHubCLI(); !r.OK {
		return r
	}

	var allAlerts []AlertOutput
	summary := &AlertSummary{}
	targetErrors := map[string]error{}

	for _, target := range targets {
		targetAlertsResult := collectAlertOutputs(target, selectionOptions.SeverityFilter)
		if !targetAlertsResult.OK {
			targetErrors[target.FullName], _ = coreResultError(targetAlertsResult).(error)
			continue
		}
		targetAlerts := targetAlertsResult.Value.([]AlertOutput)

		for _, alert := range targetAlerts {
			summary.Add(alert.Severity)
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	if r := combineSecurityTargetErrors("security alerts", targetErrors); !r.OK {
		return r
	}

	recordedRepo := metricRepositoryForTargets(targets)
	recordedTarget := recordedRepo
	recordSecurityMetricsEvent(buildSecurityMetricsEvent("security.alerts", startedAt, recordedRepo, map[string]any{
		"target":   recordedTarget,
		"total":    summary.Total,
		"critical": summary.Critical,
		"high":     summary.High,
		"medium":   summary.Medium,
		"low":      summary.Low,
		"unknown":  summary.Unknown,
	}))

	if selectionOptions.JSONOutput {
		cli.Text(core.JSONMarshalString(allAlerts))
		return core.Ok(nil)
	}

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Alerts", selectionOptions.ExternalTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return core.Ok(nil)
	}

	for _, alert := range allAlerts {
		sevStyle := severityStyle(alert.Severity)

		// Format: repo  SEVERITY  ID  package/location  type
		location := alert.Package
		if location == "" {
			location = alert.Location
		}
		if alert.Version != "" {
			location = core.Sprintf("%s %s", location, cli.DimStyle.Render(alert.Version))
		}

		cli.Print("%-20s %s  %-16s %-40s %s\n",
			cli.ValueStyle.Render(alert.Repo),
			sevStyle.Render(core.Sprintf("%-8s", alert.Severity)),
			alert.ID,
			location,
			cli.DimStyle.Render(alert.Type),
		)
	}
	cli.Blank()

	return core.Ok(nil)
}

func collectAlertOutputs(target SecurityTarget, severityFilter string) core.Result {
	dependabotResult := collectDepAlerts(target, severityFilter)
	codeScanningResult := collectScanAlerts(target, ScanCommandOptions{
		Selection: SecuritySelectionOptions{
			SeverityFilter: severityFilter,
		},
	})
	secretScanningResult := collectSecretAlerts(target)

	if !dependabotResult.OK || !codeScanningResult.OK || !secretScanningResult.OK {
		dependabotError, _ := coreResultError(dependabotResult).(error)
		codeScanningError, _ := coreResultError(codeScanningResult).(error)
		secretScanningError, _ := coreResultError(secretScanningResult).(error)
		r := combineSecurityCollectorErrors(target.FullName, map[string]error{
			"dependabot":      dependabotError,
			"code-scanning":   codeScanningError,
			"secret-scanning": secretScanningError,
		})
		return r
	}

	dependabotAlerts := dependabotResult.Value.([]DepAlert)
	codeScanningAlerts := codeScanningResult.Value.([]ScanAlert)
	secretScanningAlerts := secretScanningResult.Value.([]SecretAlert)
	return core.Ok(buildAlertOutputs(dependabotAlerts, codeScanningAlerts, secretScanningAlerts, severityFilter))
}

func buildAlertOutputs(dependabotAlerts []DepAlert, codeScanningAlerts []ScanAlert, secretScanningAlerts []SecretAlert, severityFilter string) []AlertOutput {
	allAlerts := make([]AlertOutput, 0, len(dependabotAlerts)+len(codeScanningAlerts)+len(secretScanningAlerts))

	for _, alert := range dependabotAlerts {
		allAlerts = append(allAlerts, AlertOutput{
			Repo:     alert.Repo,
			Severity: alert.Severity,
			ID:       alert.CVE,
			Package:  alert.Package,
			Version:  alert.Vulnerable,
			Type:     "dependabot",
			Message:  alert.Summary,
		})
	}

	for _, alert := range codeScanningAlerts {
		allAlerts = append(allAlerts, AlertOutput{
			Repo:     alert.Repo,
			Severity: alert.Severity,
			ID:       alert.RuleID,
			Location: core.Sprintf("%s:%d", alert.Path, alert.Line),
			Type:     "code-scanning",
			Message:  alert.Message,
		})
	}

	if filterBySeverity("high", severityFilter) {
		for _, alert := range secretScanningAlerts {
			allAlerts = append(allAlerts, AlertOutput{
				Repo:     alert.Repo,
				Severity: "high",
				ID:       core.Sprintf("secret-%d", alert.Number),
				Type:     "secret-scanning",
				Message:  alert.SecretType,
			})
		}
	}

	return allAlerts
}

func fetchDependabotAlerts(repoFullName string) core.Result {
	endpoint := core.Sprintf("repos/%s/dependabot/alerts?state=open", repoFullName)
	outputResult := callGitHubAPIRequest(endpoint)
	if !outputResult.OK {
		err, _ := coreResultError(outputResult).(error)
		return core.Fail(cli.Wrap(err, core.Sprintf("fetch dependabot alerts for %s", repoFullName)))
	}
	output := outputResult.Value.([]byte)

	alertsResult := decodeDependabotAlerts(output)
	if !alertsResult.OK {
		err, _ := coreResultError(alertsResult).(error)
		return core.Fail(cli.Wrap(err, core.Sprintf("parse dependabot alerts for %s", repoFullName)))
	}
	return alertsResult
}

func fetchCodeScanningAlerts(repoFullName string) core.Result {
	endpoint := core.Sprintf("repos/%s/code-scanning/alerts?state=open", repoFullName)
	outputResult := callGitHubAPIRequest(endpoint)
	if !outputResult.OK {
		err, _ := coreResultError(outputResult).(error)
		return core.Fail(cli.Wrap(err, core.Sprintf("fetch code-scanning alerts for %s", repoFullName)))
	}
	output := outputResult.Value.([]byte)

	alertsResult := decodeCodeScanningAlerts(output)
	if !alertsResult.OK {
		err, _ := coreResultError(alertsResult).(error)
		return core.Fail(cli.Wrap(err, core.Sprintf("parse code-scanning alerts for %s", repoFullName)))
	}
	return alertsResult
}

func fetchSecretScanningAlerts(repoFullName string) core.Result {
	endpoint := core.Sprintf("repos/%s/secret-scanning/alerts?state=open", repoFullName)
	outputResult := callGitHubAPIRequest(endpoint)
	if !outputResult.OK {
		err, _ := coreResultError(outputResult).(error)
		return core.Fail(cli.Wrap(err, core.Sprintf("fetch secret-scanning alerts for %s", repoFullName)))
	}
	output := outputResult.Value.([]byte)

	alertsResult := decodeSecretScanningAlerts(output)
	if !alertsResult.OK {
		err, _ := coreResultError(alertsResult).(error)
		return core.Fail(cli.Wrap(err, core.Sprintf("parse secret-scanning alerts for %s", repoFullName)))
	}
	return alertsResult
}
