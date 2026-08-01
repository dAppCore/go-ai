package security

import (
	"time"

	"dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func addSecretsCommand(c *core.Core, path string) core.Result {
	return registerSecurityCommand(c, path, core.Command{
		Description: cli.T("cmd.security.secrets.long"),
		Flags:       securitySelectionFlags(),
		Action: func(opts core.Options) core.Result {
			return runSecrets(securitySelectionFromOptions(opts))
		},
	})
}

// SecretAlert is the normalised row emitted by `core security secrets --json`.
type SecretAlert struct {
	Repo           string `json:"repo"`
	Number         int    `json:"number"`
	SecretType     string `json:"secret_type"`
	State          string `json:"state"`
	Resolution     string `json:"resolution,omitempty"`
	PushProtection bool   `json:"push_protection_bypassed"`
}

func runSecrets(selectionOptions SecuritySelectionOptions) core.Result {
	startedAt := time.Now()

	targetsResult := resolveSecurityTargets(selectionOptions.RegistryPath, selectionOptions.RepositoryName, selectionOptions.ExternalTarget)
	if !targetsResult.OK {
		return targetsResult
	}
	targets := targetsResult.Value.([]SecurityTarget)

	if r := checkGitHubCLI(); !r.OK {
		return r
	}

	var allAlerts []SecretAlert
	summary := &AlertSummary{}
	targetErrors := map[string]error{}

	for _, target := range targets {
		targetAlertsResult := collectSecretAlerts(target)
		if !targetAlertsResult.OK {
			targetErrors[target.FullName], _ = coreResultError(targetAlertsResult).(error)
			continue
		}
		targetAlerts := targetAlertsResult.Value.([]SecretAlert)

		for range targetAlerts {
			summary.Add("high")
		}
		allAlerts = append(allAlerts, targetAlerts...)
	}

	if r := combineSecurityTargetErrors("security secrets", targetErrors); !r.OK {
		return r
	}

	recordedRepo := metricRepositoryForTargets(targets)
	recordedTarget := recordedRepo
	recordSecurityMetricsEvent(buildSecurityMetricsEvent("security.secrets", startedAt, recordedRepo, map[string]any{
		"target": recordedTarget,
		"total":  summary.Total,
	}))

	if selectionOptions.JSONOutput {
		cli.Text(core.JSONMarshalString(allAlerts))
		return core.Ok(nil)
	}

	cli.Blank()
	if summary.Total > 0 {
		cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Secrets", selectionOptions.ExternalTarget)+":"), cli.ErrorStyle.Render(core.Sprintf("%d open", summary.Total)))
	} else {
		cli.Print("%s %s\n", cli.DimStyle.Render(securitySectionLabel("Secrets", selectionOptions.ExternalTarget)+":"), cli.SuccessStyle.Render("No exposed secrets"))
	}
	cli.Blank()

	if len(allAlerts) == 0 {
		return core.Ok(nil)
	}

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

	return core.Ok(nil)
}

func collectSecretAlerts(target SecurityTarget) core.Result {
	alertsResult := fetchSecretScanningAlerts(target.FullName)
	if !alertsResult.OK {
		return alertsResult
	}
	alerts := alertsResult.Value.([]SecretScanningAlert)

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
	return core.Ok(allAlerts)
}
