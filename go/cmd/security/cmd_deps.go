package security

import (
	"time"

	"dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/i18n"
)

func addDepsCommand(parent *cli.Command) {
	selectionOptions := &SecuritySelectionOptions{}

	cmd := &cli.Command{
		Use:   "deps",
		Short: i18n.T("cmd.security.deps.short"),
		Long:  i18n.T("cmd.security.deps.long"),
		RunE: func(c *cli.Command, args []string) error {
			r := runDeps(*selectionOptions)
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

// DepAlert is the normalised row emitted by `core security deps --json`.
type DepAlert struct {
	Repo           string `json:"repo"`
	Severity       string `json:"severity"`
	CVE            string `json:"cve"`
	Package        string `json:"package"`
	Ecosystem      string `json:"ecosystem"`
	Vulnerable     string `json:"vulnerable_range"`
	PatchedVersion string `json:"patched_version,omitempty"`
	Manifest       string `json:"manifest"`
	Summary        string `json:"summary"`
}

func runDeps(selectionOptions SecuritySelectionOptions) core.Result {
	startedAt := time.Now()

	targetsResult := resolveSecurityTargets(selectionOptions.RegistryPath, selectionOptions.RepositoryName, selectionOptions.ExternalTarget)
	if !targetsResult.OK {
		return targetsResult
	}
	targets := targetsResult.Value.([]SecurityTarget)

	if r := checkGitHubCLI(); !r.OK {
		return r
	}

	var allAlerts []DepAlert
	summary := &AlertSummary{}
	targetErrors := map[string]error{}

	for _, target := range targets {
		targetAlertsResult := collectDepAlerts(target, selectionOptions.SeverityFilter)
		if !targetAlertsResult.OK {
			targetErrors[target.FullName], _ = coreResultError(targetAlertsResult).(error)
			continue
		}
		targetAlerts := targetAlertsResult.Value.([]DepAlert)

		for _, alert := range targetAlerts {
			summary.Add(alert.Severity)
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	if r := combineSecurityTargetErrors("security deps", targetErrors); !r.OK {
		return r
	}

	recordedRepo := metricRepositoryForTargets(targets)
	recordedTarget := recordedRepo
	recordSecurityMetricsEvent(buildSecurityMetricsEvent("security.deps", startedAt, recordedRepo, map[string]any{
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
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Dependabot", selectionOptions.ExternalTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return core.Ok(nil)
	}

	for _, alert := range allAlerts {
		sevStyle := severityStyle(alert.Severity)

		// Format upgrade suggestion
		upgrade := alert.Vulnerable
		if alert.PatchedVersion != "" {
			upgrade = core.Sprintf("%s -> %s", alert.Vulnerable, cli.SuccessStyle.Render(alert.PatchedVersion))
		}

		cli.Print("%-16s %s  %-16s %-30s %s\n",
			cli.ValueStyle.Render(alert.Repo),
			sevStyle.Render(core.Sprintf("%-8s", alert.Severity)),
			alert.CVE,
			alert.Package,
			upgrade,
		)
	}
	cli.Blank()

	return core.Ok(nil)
}

func collectDepAlerts(target SecurityTarget, severityFilter string) core.Result {
	alertsResult := fetchDependabotAlerts(target.FullName)
	if !alertsResult.OK {
		return alertsResult
	}
	alerts := alertsResult.Value.([]DependabotAlert)

	var allAlerts []DepAlert
	for _, alert := range alerts {
		if alert.State != "open" {
			continue
		}
		severity := alert.Advisory.Severity
		if !filterBySeverity(severity, severityFilter) {
			continue
		}

		allAlerts = append(allAlerts, DepAlert{
			Repo:           target.DisplayName,
			Severity:       severity,
			CVE:            alert.Advisory.CVEID,
			Package:        alert.Dependency.Package.Name,
			Ecosystem:      alert.Dependency.Package.Ecosystem,
			Vulnerable:     alert.SecurityVulnerability.VulnerableVersionRange,
			PatchedVersion: alert.SecurityVulnerability.FirstPatchedVersion.Identifier,
			Manifest:       alert.Dependency.ManifestPath,
			Summary:        alert.Advisory.Summary,
		})
	}
	return core.Ok(allAlerts)
}
