package security

import (
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"forge.lthn.ai/core/cli/pkg/cli"
)

func addAlertsCommand(parent *cli.Command) {
	selectionOptions := &SecuritySelectionOptions{}

	cmd := &cli.Command{
		Use:   "alerts",
		Short: i18n.T("cmd.security.alerts.short"),
		Long:  i18n.T("cmd.security.alerts.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runAlerts(*selectionOptions)
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

func runAlerts(selectionOptions SecuritySelectionOptions) error {
	startedAt := time.Now()

	targets, err := resolveSecurityTargets(selectionOptions.RegistryPath, selectionOptions.RepositoryName, selectionOptions.ExternalTarget)
	if err != nil {
		return err
	}

	if err := checkGitHubCLI(); err != nil {
		return err
	}

	var allAlerts []AlertOutput
	summary := &AlertSummary{}
	targetErrors := map[string]error{}

	for _, target := range targets {
		targetAlerts, err := collectAlertOutputs(target, selectionOptions.SeverityFilter)
		if err != nil {
			targetErrors[target.FullName] = err
			continue
		}

		for _, alert := range targetAlerts {
			summary.Add(alert.Severity)
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	if err := combineSecurityTargetErrors("security alerts", targetErrors); err != nil {
		return err
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
		return nil
	}

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Alerts", selectionOptions.ExternalTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return nil
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

	return nil
}

func collectAlertOutputs(target SecurityTarget, severityFilter string) ([]AlertOutput, error) {
	dependabotAlerts, dependabotError := collectDepAlerts(target, severityFilter)
	codeScanningAlerts, codeScanningError := collectScanAlerts(target, ScanCommandOptions{
		Selection: SecuritySelectionOptions{
			SeverityFilter: severityFilter,
		},
	})
	secretScanningAlerts, secretScanningError := collectSecretAlerts(target)

	if dependabotError != nil || codeScanningError != nil || secretScanningError != nil {
		return nil, combineSecurityCollectorErrors(target.FullName, map[string]error{
			"dependabot":      dependabotError,
			"code-scanning":   codeScanningError,
			"secret-scanning": secretScanningError,
		})
	}

	return buildAlertOutputs(dependabotAlerts, codeScanningAlerts, secretScanningAlerts, severityFilter), nil
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

func fetchDependabotAlerts(repoFullName string) ([]DependabotAlert, error) {
	endpoint := core.Sprintf("repos/%s/dependabot/alerts?state=open", repoFullName)
	output, err := callGitHubAPIRequest(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("fetch dependabot alerts for %s", repoFullName))
	}

	alerts, err := decodeDependabotAlerts(output)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("parse dependabot alerts for %s", repoFullName))
	}
	return alerts, nil
}

func fetchCodeScanningAlerts(repoFullName string) ([]CodeScanningAlert, error) {
	endpoint := core.Sprintf("repos/%s/code-scanning/alerts?state=open", repoFullName)
	output, err := callGitHubAPIRequest(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("fetch code-scanning alerts for %s", repoFullName))
	}

	alerts, err := decodeCodeScanningAlerts(output)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("parse code-scanning alerts for %s", repoFullName))
	}
	return alerts, nil
}

func fetchSecretScanningAlerts(repoFullName string) ([]SecretScanningAlert, error) {
	endpoint := core.Sprintf("repos/%s/secret-scanning/alerts?state=open", repoFullName)
	output, err := callGitHubAPIRequest(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("fetch secret-scanning alerts for %s", repoFullName))
	}

	alerts, err := decodeSecretScanningAlerts(output)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("parse secret-scanning alerts for %s", repoFullName))
	}
	return alerts, nil
}
