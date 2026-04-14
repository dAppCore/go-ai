package security

import (
	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"forge.lthn.ai/core/cli/pkg/cli"
)

func addAlertsCommand(parent *cli.Command) {
	cmd := &cli.Command{
		Use:   "alerts",
		Short: i18n.T("cmd.security.alerts.short"),
		Long:  i18n.T("cmd.security.alerts.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runAlerts()
		},
	}

	cmd.Flags().StringVar(&securityRegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&securityRepo, "repo", "", i18n.T("cmd.security.flag.repo"))
	cmd.Flags().StringVar(&securitySeverity, "severity", "", i18n.T("cmd.security.flag.severity"))
	cmd.Flags().BoolVar(&securityJSON, "json", false, i18n.T("common.flag.json"))
	cmd.Flags().StringVar(&securityTarget, "target", "", i18n.T("cmd.security.flag.target"))

	parent.AddCommand(cmd)
}

// AlertOutput represents a unified alert for output.
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

func runAlerts() error {
	if err := checkGitHubCLI(); err != nil {
		return err
	}

	targets, err := resolveSecurityTargets(securityRegistryPath, securityRepo, securityTarget)
	if err != nil {
		return err
	}

	var allAlerts []AlertOutput
	summary := &AlertSummary{}

	for _, target := range targets {
		targetAlerts, err := collectAlertOutputs(target)
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

	if securityJSON {
		cli.Text(core.JSONMarshalString(allAlerts))
		return nil
	}

	// Print summary
	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Alerts", securityTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return nil
	}

	// Print table
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

func collectAlertOutputs(target SecurityTarget) ([]AlertOutput, error) {
	var allAlerts []AlertOutput
	var fetchErrors int

	dependabotAlerts, err := fetchDependabotAlerts(target.FullName)
	if err != nil {
		fetchErrors++
	} else {
		for _, alert := range dependabotAlerts {
			if alert.State != "open" {
				continue
			}
			severity := alert.Advisory.Severity
			if !filterBySeverity(severity, securitySeverity) {
				continue
			}
			allAlerts = append(allAlerts, AlertOutput{
				Repo:     target.DisplayName,
				Severity: severity,
				ID:       alert.Advisory.CVEID,
				Package:  alert.Dependency.Package.Name,
				Version:  alert.SecurityVulnerability.VulnerableVersionRange,
				Type:     "dependabot",
				Message:  alert.Advisory.Summary,
			})
		}
	}

	codeScanningAlerts, err := fetchCodeScanningAlerts(target.FullName)
	if err != nil {
		fetchErrors++
	} else {
		for _, alert := range codeScanningAlerts {
			if alert.State != "open" {
				continue
			}
			severity := alert.Rule.Severity
			if !filterBySeverity(severity, securitySeverity) {
				continue
			}
			location := core.Sprintf("%s:%d", alert.MostRecentInstance.Location.Path, alert.MostRecentInstance.Location.StartLine)
			allAlerts = append(allAlerts, AlertOutput{
				Repo:     target.DisplayName,
				Severity: severity,
				ID:       alert.Rule.ID,
				Location: location,
				Type:     "code-scanning",
				Message:  alert.MostRecentInstance.Message.Text,
			})
		}
	}

	secretScanningAlerts, err := fetchSecretScanningAlerts(target.FullName)
	if err != nil {
		fetchErrors++
	} else {
		for _, alert := range secretScanningAlerts {
			if alert.State != "open" {
				continue
			}
			if !filterBySeverity("high", securitySeverity) {
				continue
			}
			allAlerts = append(allAlerts, AlertOutput{
				Repo:     target.DisplayName,
				Severity: "high",
				ID:       core.Sprintf("secret-%d", alert.Number),
				Type:     "secret-scanning",
				Message:  alert.SecretType,
			})
		}
	}

	if fetchErrors == 3 {
		return nil, cli.Err("failed to fetch any alerts for %s", target.FullName)
	}

	return allAlerts, nil
}

func fetchDependabotAlerts(repoFullName string) ([]DependabotAlert, error) {
	endpoint := core.Sprintf("repos/%s/dependabot/alerts?state=open", repoFullName)
	output, err := runGitHubAPI(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("fetch dependabot alerts for %s", repoFullName))
	}

	var alerts []DependabotAlert
	r := core.JSONUnmarshal(output, &alerts)
	if !r.OK {
		return nil, cli.Wrap(r.Value.(error), core.Sprintf("parse dependabot alerts for %s", repoFullName))
	}
	return alerts, nil
}

func fetchCodeScanningAlerts(repoFullName string) ([]CodeScanningAlert, error) {
	endpoint := core.Sprintf("repos/%s/code-scanning/alerts?state=open", repoFullName)
	output, err := runGitHubAPI(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("fetch code-scanning alerts for %s", repoFullName))
	}

	var alerts []CodeScanningAlert
	r := core.JSONUnmarshal(output, &alerts)
	if !r.OK {
		return nil, cli.Wrap(r.Value.(error), core.Sprintf("parse code-scanning alerts for %s", repoFullName))
	}
	return alerts, nil
}

func fetchSecretScanningAlerts(repoFullName string) ([]SecretScanningAlert, error) {
	endpoint := core.Sprintf("repos/%s/secret-scanning/alerts?state=open", repoFullName)
	output, err := runGitHubAPI(endpoint)
	if err != nil {
		return nil, cli.Wrap(err, core.Sprintf("fetch secret-scanning alerts for %s", repoFullName))
	}

	var alerts []SecretScanningAlert
	r := core.JSONUnmarshal(output, &alerts)
	if !r.OK {
		return nil, cli.Wrap(r.Value.(error), core.Sprintf("parse secret-scanning alerts for %s", repoFullName))
	}
	return alerts, nil
}
