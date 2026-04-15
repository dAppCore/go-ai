package security

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"slices"

	"dappco.re/go/ai/ai"
	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/scm/repos"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var callGitHubAPIRequest = runGitHubAPI

func recordSecurityMetricsEvent(event ai.Event) {
	_ = ai.Record(event)
}

// core security alerts --target wailsapp/wails
// core security jobs --targets all --copies 4
type SecuritySelectionOptions struct {
	RegistryPath   string
	RepositoryName string
	SeverityFilter string
	JSONOutput     bool
	ExternalTarget string
}

// ScanCommandOptions{Selection: SecuritySelectionOptions{ExternalTarget: "wailsapp/wails"}, ToolName: "CodeQL"} runs one scoped scan.
type ScanCommandOptions struct {
	Selection SecuritySelectionOptions
	ToolName  string
}

// JobsCommandOptions{Targets: "all", IssueRepository: "host-uk/core", WorkerCount: 4} runs one multi-repo batch scan.
type JobsCommandOptions struct {
	RegistryPath    string
	Targets         string
	IssueRepository string
	DryRun          bool
	WorkerCount     int
}

// core security alerts --repo core-php
// core security jobs --targets all --copies 4
func AddSecurityCommands(root *cli.Command) {
	if commandExists(root, "security") {
		return
	}

	securityCommand := &cli.Command{
		Use:   "security",
		Short: i18n.T("cmd.security.short"),
		Long:  i18n.T("cmd.security.long"),
	}

	addAlertsCommand(securityCommand)
	addDepsCommand(securityCommand)
	addScanCommand(securityCommand)
	addSecretsCommand(securityCommand)
	addJobsCommand(securityCommand)

	root.AddCommand(securityCommand)
}

func commandExists(parent *cli.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}

// DependabotAlert represents a Dependabot vulnerability alert.
type DependabotAlert struct {
	Number   int    `json:"number"`
	State    string `json:"state"`
	Advisory struct {
		Severity    string `json:"severity"`
		CVEID       string `json:"cve_id"`
		Summary     string `json:"summary"`
		Description string `json:"description"`
	} `json:"security_advisory"`
	Dependency struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		ManifestPath string `json:"manifest_path"`
	} `json:"dependency"`
	SecurityVulnerability struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		FirstPatchedVersion struct {
			Identifier string `json:"identifier"`
		} `json:"first_patched_version"`
		VulnerableVersionRange string `json:"vulnerable_version_range"`
	} `json:"security_vulnerability"`
}

// CodeScanningAlert represents a code scanning alert.
type CodeScanningAlert struct {
	Number          int    `json:"number"`
	State           string `json:"state"`
	DismissedReason string `json:"dismissed_reason"`
	Rule            struct {
		ID          string   `json:"id"`
		Severity    string   `json:"severity"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	} `json:"rule"`
	Tool struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"tool"`
	MostRecentInstance struct {
		Location struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		} `json:"location"`
		Message struct {
			Text string `json:"text"`
		} `json:"message"`
	} `json:"most_recent_instance"`
}

// SecretScanningAlert represents a secret scanning alert.
type SecretScanningAlert struct {
	Number         int    `json:"number"`
	State          string `json:"state"`
	SecretType     string `json:"secret_type"`
	Secret         string `json:"secret"`
	PushProtection bool   `json:"push_protection_bypassed"`
	Resolution     string `json:"resolution"`
}

func loadRegistry(registryPath string) (*repos.Registry, error) {
	if registryPath != "" {
		registry, err := repos.LoadRegistry(io.Local, registryPath)
		if err != nil {
			return nil, cli.Wrap(err, "load registry")
		}
		return registry, nil
	}

	path, err := repos.FindRegistry(io.Local)
	if err != nil {
		return nil, cli.Wrap(err, "find registry")
	}
	registry, err := repos.LoadRegistry(io.Local, path)
	if err != nil {
		return nil, cli.Wrap(err, "load registry")
	}
	return registry, nil
}

func checkGitHubCLI() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return coreerr.E("security", i18n.T("error.gh_not_found"), nil)
	}
	return nil
}

func runGitHubAPI(endpoint string) ([]byte, error) {
	cmd := exec.Command("gh", "api", endpoint, "--paginate", "--slurp")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if core.Contains(stderr, "404") || core.Contains(stderr, "Not Found") {
				return []byte("[]"), nil
			}
			if core.Contains(stderr, "403") {
				return nil, coreerr.E("security", "access denied (check token permissions)", nil)
			}
		}
		return nil, cli.Wrap(err, "run gh api")
	}
	return output, nil
}

type githubRepoResponse struct {
	FullName string `json:"full_name"`
}

func decodeGitHubArrayItems(output []byte) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("[]")) {
		return nil, nil
	}

	var pages []json.RawMessage
	if err := json.Unmarshal(trimmed, &pages); err != nil {
		return nil, coreerr.E("security", "parse GitHub API response", err)
	}

	items := make([]json.RawMessage, 0, len(pages))
	for _, page := range pages {
		pageData := bytes.TrimSpace(page)
		if len(pageData) == 0 || bytes.Equal(pageData, []byte("[]")) {
			continue
		}

		if pageData[0] != '[' {
			items = append(items, page)
			continue
		}

		var pageItems []json.RawMessage
		if err := json.Unmarshal(pageData, &pageItems); err != nil {
			return nil, coreerr.E("security", "parse GitHub API page", err)
		}
		items = append(items, pageItems...)
	}

	return items, nil
}

func decodeDependabotAlerts(output []byte) ([]DependabotAlert, error) {
	items, err := decodeGitHubArrayItems(output)
	if err != nil {
		return nil, err
	}

	alerts := make([]DependabotAlert, 0, len(items))
	for _, item := range items {
		var alert DependabotAlert
		if err := json.Unmarshal(item, &alert); err != nil {
			return nil, coreerr.E("security", "parse dependabot alert", err)
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func decodeCodeScanningAlerts(output []byte) ([]CodeScanningAlert, error) {
	items, err := decodeGitHubArrayItems(output)
	if err != nil {
		return nil, err
	}

	alerts := make([]CodeScanningAlert, 0, len(items))
	for _, item := range items {
		var alert CodeScanningAlert
		if err := json.Unmarshal(item, &alert); err != nil {
			return nil, coreerr.E("security", "parse code scanning alert", err)
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func decodeSecretScanningAlerts(output []byte) ([]SecretScanningAlert, error) {
	items, err := decodeGitHubArrayItems(output)
	if err != nil {
		return nil, err
	}

	alerts := make([]SecretScanningAlert, 0, len(items))
	for _, item := range items {
		var alert SecretScanningAlert
		if err := json.Unmarshal(item, &alert); err != nil {
			return nil, coreerr.E("security", "parse secret scanning alert", err)
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func decodeGitHubRepositoryNames(output []byte) ([]string, error) {
	items, err := decodeGitHubArrayItems(output)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		var repository githubRepoResponse
		if err := json.Unmarshal(item, &repository); err != nil {
			return nil, coreerr.E("security", "parse GitHub repository", err)
		}
		if repository.FullName == "" {
			continue
		}
		if _, ok := seen[repository.FullName]; ok {
			continue
		}
		seen[repository.FullName] = struct{}{}
		names = append(names, repository.FullName)
	}

	slices.Sort(names)
	return names, nil
}

// severityStyle returns the appropriate style for a severity level.
func severityStyle(severity string) *cli.AnsiStyle {
	switch core.Lower(severity) {
	case "critical":
		return cli.ErrorStyle
	case "high":
		return cli.WarningStyle
	case "medium":
		return cli.ValueStyle
	default:
		return cli.DimStyle
	}
}

// filterBySeverity checks if the severity matches the filter.
func filterBySeverity(severity, filter string) bool {
	if filter == "" {
		return true
	}

	sev := core.Lower(severity)
	parts := core.Split(core.Lower(filter), ",")
	return slices.ContainsFunc(parts, func(s string) bool {
		return core.Trim(s) == sev
	})
}

// AlertSummary holds aggregated alert counts.
type AlertSummary struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Unknown  int
	Total    int
}

// Add increments summary counters for the provided severity.
func (s *AlertSummary) Add(severity string) {
	s.Total++
	switch core.Lower(severity) {
	case "critical":
		s.Critical++
	case "high":
		s.High++
	case "medium":
		s.Medium++
	case "low":
		s.Low++
	default:
		s.Unknown++
	}
}

// String renders a styled summary of alert counts.
func (s *AlertSummary) String() string {
	plain := s.parts()
	if len(plain) == 0 {
		return cli.SuccessStyle.Render("No alerts")
	}

	styled := make([]string, 0, len(plain))
	for _, part := range plain {
		fields := core.Split(part, " ")
		switch fields[len(fields)-1] {
		case "critical":
			styled = append(styled, cli.ErrorStyle.Render(part))
		case "high":
			styled = append(styled, cli.WarningStyle.Render(part))
		case "medium":
			styled = append(styled, cli.ValueStyle.Render(part))
		default:
			styled = append(styled, cli.DimStyle.Render(part))
		}
	}
	return core.Join(" | ", styled...)
}

// PlainString() returns strings like "1 critical | 2 high" for logs and issue bodies.
func (s *AlertSummary) PlainString() string {
	parts := s.parts()
	if len(parts) == 0 {
		return "No alerts"
	}
	return core.Join(" | ", parts...)
}

func (s *AlertSummary) parts() []string {
	var parts []string
	if s.Critical > 0 {
		parts = append(parts, core.Sprintf("%d critical", s.Critical))
	}
	if s.High > 0 {
		parts = append(parts, core.Sprintf("%d high", s.High))
	}
	if s.Medium > 0 {
		parts = append(parts, core.Sprintf("%d medium", s.Medium))
	}
	if s.Low > 0 {
		parts = append(parts, core.Sprintf("%d low", s.Low))
	}
	if s.Unknown > 0 {
		parts = append(parts, core.Sprintf("%d unknown", s.Unknown))
	}
	return parts
}
