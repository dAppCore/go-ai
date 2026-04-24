package security

import (
	"context"
	exec "os/exec" // Note: retained until security commands receive a configured core.Process context.
	"slices"
	"strings"
	"time"

	"dappco.re/go/ai/ai"
	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/scm/repos"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var callGitHubAPIRequest = runGitHubAPIStrict

const (
	githubAPITimeout     = 25 * time.Second
	githubAPIMaxAttempts = 3
	githubAPIBaseBackoff = 500 * time.Millisecond
)

var (
	errGitHubAPITimeout          = core.E("security.github.api", "GitHub API request timed out", nil)
	errGitHubAPIEndpointNotFound = core.E("security.github.api", "GitHub API endpoint not found", nil)
	errGitHubAPIAccessDenied     = core.E("security.github.api", "GitHub API access denied", nil)
)

func recordSecurityMetricsEvent(event ai.Event) {
	_ = ai.Record(event)
}

// SecuritySelectionOptions{RepositoryName: "go-ai", SeverityFilter: "high", JSONOutput: true}
// captures one repo-scoped security command invocation.
type SecuritySelectionOptions struct {
	RegistryPath   string
	RepositoryName string
	SeverityFilter string
	JSONOutput     bool
	ExternalTarget string
}

// ScanCommandOptions{Selection: SecuritySelectionOptions{ExternalTarget: "wailsapp/wails"}, ToolName: "CodeQL"}
// captures one `core security scan` invocation.
type ScanCommandOptions struct {
	Selection SecuritySelectionOptions
	ToolName  string
}

// JobsCommandOptions{Targets: "all", IssueRepository: "host-uk/core", WorkerCount: 4}
// captures one `core security jobs` batch run.
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
	return runGitHubAPIWithMode(endpoint, true)
}

func runGitHubAPIStrict(endpoint string) ([]byte, error) {
	return runGitHubAPIWithMode(endpoint, false)
}

func runGitHubAPIWithMode(endpoint string, allowMissingEndpoint bool) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < githubAPIMaxAttempts; attempt++ {
		output, err := runGitHubAPIRequest(endpoint)
		if err == nil {
			return output, nil
		}

		lastErr = err
		if allowMissingEndpoint && core.Is(err, errGitHubAPIEndpointNotFound) {
			return []byte("[]"), nil
		}

		if core.Is(err, errGitHubAPIAccessDenied) {
			return nil, err
		}

		if attempt == githubAPIMaxAttempts-1 || !isRetryableGitHubAPIError(err) {
			return nil, cli.Wrap(lastErr, "run gh api")
		}

		time.Sleep(githubAPIBaseBackoff << attempt)
	}

	return nil, cli.Wrap(lastErr, "run gh api")
}

func runGitHubAPIRequest(endpoint string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubAPITimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "api", endpoint, "--paginate", "--slurp")
	output, err := cmd.Output()
	if err != nil {
		if core.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, core.E("security.github.api", "GitHub API request timed out", errGitHubAPITimeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if core.Contains(stderr, "404") || core.Contains(stderr, "Not Found") {
				return nil, errGitHubAPIEndpointNotFound
			}
			if core.Contains(stderr, "403") || core.Contains(stderr, "Forbidden") {
				return nil, core.E("security.github.api", "check token permissions", errGitHubAPIAccessDenied)
			}
		}
		return nil, err
	}
	return trimGitHubJSONBytes(output), nil
}

func isRetryableGitHubAPIError(err error) bool {
	return !core.Is(err, errGitHubAPIEndpointNotFound) &&
		!core.Is(err, errGitHubAPIAccessDenied)
}

type githubRepoResponse struct {
	FullName string `json:"full_name"`
}

type githubRawMessage []byte

func (m *githubRawMessage) UnmarshalJSON(data []byte) error {
	if m == nil {
		return core.E("security.github.rawMessage", "unmarshal JSON into nil raw message", nil)
	}
	*m = append((*m)[0:0], data...)
	return nil
}

func trimGitHubJSONBytes(data []byte) []byte {
	return []byte(core.Trim(string(data)))
}

func coreResultError(result core.Result) error {
	if err, ok := result.Value.(error); ok {
		return err
	}
	return core.E("security.core.result", "operation failed", nil)
}

func decodeGitHubArrayItems(output []byte) ([]githubRawMessage, error) {
	trimmed := trimGitHubJSONBytes(output)
	if len(trimmed) == 0 || string(trimmed) == "[]" {
		return nil, nil
	}

	var pages []githubRawMessage
	if result := core.JSONUnmarshal(trimmed, &pages); !result.OK {
		return nil, coreerr.E("security", "parse GitHub API response", coreResultError(result))
	}

	items := make([]githubRawMessage, 0, len(pages))
	for _, page := range pages {
		pageData := trimGitHubJSONBytes(page)
		if len(pageData) == 0 || string(pageData) == "[]" {
			continue
		}

		if pageData[0] != '[' {
			items = append(items, githubRawMessage(pageData))
			continue
		}

		var pageItems []githubRawMessage
		if result := core.JSONUnmarshal(pageData, &pageItems); !result.OK {
			return nil, coreerr.E("security", "parse GitHub API page", coreResultError(result))
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
		if result := core.JSONUnmarshal(item, &alert); !result.OK {
			return nil, coreerr.E("security", "parse dependabot alert", coreResultError(result))
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
		if result := core.JSONUnmarshal(item, &alert); !result.OK {
			return nil, coreerr.E("security", "parse code scanning alert", coreResultError(result))
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
		if result := core.JSONUnmarshal(item, &alert); !result.OK {
			return nil, coreerr.E("security", "parse secret scanning alert", coreResultError(result))
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
		if result := core.JSONUnmarshal(item, &repository); !result.OK {
			return nil, coreerr.E("security", "parse GitHub repository", coreResultError(result))
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

func combineSecurityCollectorErrors(target string, collectorErrors map[string]error) error {
	type collectorFailure struct {
		name string
		err  error
	}

	failures := make([]collectorFailure, 0, len(collectorErrors))
	for name, err := range collectorErrors {
		if err == nil {
			continue
		}
		failures = append(failures, collectorFailure{name: name, err: err})
	}

	if len(failures) == 0 {
		return nil
	}

	slices.SortFunc(failures, func(a, b collectorFailure) int {
		return strings.Compare(a.name, b.name)
	})

	missingCollectors := make([]string, 0, len(failures))
	messages := make([]string, 0, len(failures))
	for _, failure := range failures {
		missingCollectors = append(missingCollectors, failure.name)
		messages = append(messages, core.Sprintf("%s: %v", failure.name, failure.err))
	}

	return coreerr.E("security", core.Sprintf("failed to fetch %s for %s: %s",
		core.Join(", ", missingCollectors...),
		target,
		core.Join("; ", messages...),
	), nil)
}

func combineSecurityTargetErrors(commandName string, targetErrors map[string]error) error {
	if len(targetErrors) == 0 {
		return nil
	}

	targetNames := make([]string, 0, len(targetErrors))
	for targetName := range targetErrors {
		targetNames = append(targetNames, targetName)
	}
	slices.Sort(targetNames)

	messages := make([]string, 0, len(targetNames))
	for _, targetName := range targetNames {
		messages = append(messages, core.Sprintf("%s: %v", targetName, targetErrors[targetName]))
	}

	return coreerr.E("security", core.Sprintf("%s failed for %d target(s): %s",
		commandName,
		len(targetNames),
		core.Join("; ", messages...),
	), nil)
}

func buildSecurityMetricsEvent(eventType string, startedAt time.Time, repository string, data map[string]any) ai.Event {
	return ai.Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Repo:      repository,
		Duration:  time.Since(startedAt),
		Data:      data,
	}
}

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

// summary.Add("critical")
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

// summary.String() // "1 critical | 2 high"
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

// summary.PlainString() // "1 critical | 2 high"
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
