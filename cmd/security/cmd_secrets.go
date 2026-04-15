package security

import (
	"time"

	"dappco.re/go/ai/ai"
	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"forge.lthn.ai/core/cli/pkg/cli"
)

func addSecretsCommand(parent *cli.Command) {
	selectionOptions := &SecuritySelectionOptions{}

	cmd := &cli.Command{
		Use:   "secrets",
		Short: i18n.T("cmd.security.secrets.short"),
		Long:  i18n.T("cmd.security.secrets.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runSecrets(*selectionOptions)
		},
	}

	cmd.Flags().StringVar(&selectionOptions.RegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&selectionOptions.RepositoryName, "repo", "", i18n.T("cmd.security.flag.repo"))
	cmd.Flags().BoolVar(&selectionOptions.JSONOutput, "json", false, i18n.T("common.flag.json"))
	cmd.Flags().StringVar(&selectionOptions.ExternalTarget, "target", "", i18n.T("cmd.security.flag.target"))

	parent.AddCommand(cmd)
}

// SecretAlert represents a secret scanning alert for output.
type SecretAlert struct {
	Repo           string `json:"repo"`
	Number         int    `json:"number"`
	SecretType     string `json:"secret_type"`
	State          string `json:"state"`
	Resolution     string `json:"resolution,omitempty"`
	PushProtection bool   `json:"push_protection_bypassed"`
}

func runSecrets(selectionOptions SecuritySelectionOptions) error {
	if err := checkGitHubCLI(); err != nil {
		return err
	}

	targets, err := resolveSecurityTargets(selectionOptions.RegistryPath, selectionOptions.RepositoryName, selectionOptions.ExternalTarget)
	if err != nil {
		return err
	}

	var allAlerts []SecretAlert
	summary := &AlertSummary{}

	for _, target := range targets {
		targetAlerts, err := collectSecretAlerts(target)
		if err != nil {
			if selectionOptions.ExternalTarget != "" {
				return err
			}
			cli.Print("%s %s: %v\n", cli.WarningStyle.Render(">>"), target.FullName, err)
			continue
		}

		for range targetAlerts {
			summary.Add("high")
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	recordedRepo := metricRepositoryForTargets(targets)
	recordedTarget := recordedRepo
	recordSecurityMetricsEvent(ai.Event{
		Type:      "security.secrets",
		Timestamp: time.Now(),
		Repo:      recordedRepo,
		Data: map[string]any{
			"target": recordedTarget,
			"total":  summary.Total,
		},
	})

	if selectionOptions.JSONOutput {
		cli.Text(core.JSONMarshalString(allAlerts))
		return nil
	}

	// Print summary
	cli.Blank()
	if summary.Total > 0 {
		cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Secrets", selectionOptions.ExternalTarget)+":"), cli.ErrorStyle.Render(core.Sprintf("%d open", summary.Total)))
	} else {
		cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Secrets", selectionOptions.ExternalTarget)+":"), cli.SuccessStyle.Render("No exposed secrets"))
	}
	cli.Blank()

	if len(allAlerts) == 0 {
		return nil
	}

	// Print table
	for _, alert := range allAlerts {
		bypassed := ""
		if alert.PushProtection {
			bypassed = cli.WarningStyle.Render(" (push protection bypassed)")
		}

		cli.Print("%-16s %-6d %-30s%s\n",
			cli.ValueStyle.Render(alert.Repo),
			alert.Number,
			cli.ErrorStyle.Render(alert.SecretType),
			bypassed,
		)
	}
	cli.Blank()

	return nil
}

func collectSecretAlerts(target SecurityTarget) ([]SecretAlert, error) {
	alerts, err := fetchSecretScanningAlerts(target.FullName)
	if err != nil {
		return nil, err
	}

	var allAlerts []SecretAlert
	for _, alert := range alerts {
		if alert.State != "open" {
			continue
		}

		allAlerts = append(allAlerts, SecretAlert{
			Repo:           target.DisplayName,
			Number:         alert.Number,
			SecretType:     alert.SecretType,
			State:          alert.State,
			Resolution:     alert.Resolution,
			PushProtection: alert.PushProtection,
		})
	}
	return allAlerts, nil
}
