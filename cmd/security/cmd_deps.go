package security

import (
	"time"

	"dappco.re/go/ai/ai"
	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"forge.lthn.ai/core/cli/pkg/cli"
)

func addDepsCommand(parent *cli.Command) {
	selectionOptions := &SecuritySelectionOptions{}

	cmd := &cli.Command{
		Use:   "deps",
		Short: i18n.T("cmd.security.deps.short"),
		Long:  i18n.T("cmd.security.deps.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runDeps(*selectionOptions)
		},
	}

	cmd.Flags().StringVar(&selectionOptions.RegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&selectionOptions.RepositoryName, "repo", "", i18n.T("cmd.security.flag.repo"))
	cmd.Flags().StringVar(&selectionOptions.SeverityFilter, "severity", "", i18n.T("cmd.security.flag.severity"))
	cmd.Flags().BoolVar(&selectionOptions.JSONOutput, "json", false, i18n.T("common.flag.json"))
	cmd.Flags().StringVar(&selectionOptions.ExternalTarget, "target", "", i18n.T("cmd.security.flag.target"))

	parent.AddCommand(cmd)
}

// DepAlert represents a dependency vulnerability for output.
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

func runDeps(selectionOptions SecuritySelectionOptions) error {
	if err := checkGitHubCLI(); err != nil {
		return err
	}

	targets, err := resolveSecurityTargets(selectionOptions.RegistryPath, selectionOptions.RepositoryName, selectionOptions.ExternalTarget)
	if err != nil {
		return err
	}

	var allAlerts []DepAlert
	summary := &AlertSummary{}

	for _, target := range targets {
		targetAlerts, err := collectDepAlerts(target, selectionOptions.SeverityFilter)
		if err != nil {
			if selectionOptions.ExternalTarget != "" {
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

	recordedRepo := metricRepositoryForTargets(targets)
	recordedTarget := recordedRepo
	recordSecurityMetricsEvent(ai.Event{
		Type:      "security.deps",
		Timestamp: time.Now(),
		Repo:      recordedRepo,
		Data: map[string]any{
			"target":   recordedTarget,
			"total":    summary.Total,
			"critical": summary.Critical,
			"high":     summary.High,
			"medium":   summary.Medium,
			"low":      summary.Low,
			"unknown":  summary.Unknown,
		},
	})

	if selectionOptions.JSONOutput {
		cli.Text(core.JSONMarshalString(allAlerts))
		return nil
	}

	// Print summary
	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Dependabot", selectionOptions.ExternalTarget)+":"), summary.String())
	cli.Blank()

	if len(allAlerts) == 0 {
		return nil
	}

	// Print table
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

	return nil
}

func collectDepAlerts(target SecurityTarget, severityFilter string) ([]DepAlert, error) {
	alerts, err := fetchDependabotAlerts(target.FullName)
	if err != nil {
		return nil, err
	}

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
	return allAlerts, nil
}
