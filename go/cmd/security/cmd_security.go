package security

import (
	"cmp"
	"context"
	"slices"
	"time"

	"dappco.re/go"
	"dappco.re/go/ai/ai"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/io"
	"dappco.re/go/scm/repos"
	execabs "golang.org/x/sys/execabs"
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
	if r := ai.Record(event); !r.OK {
		return
	}
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
func AddSecurityCommands(c *core.Core) core.Result {
	if r := registerSecurityCommand(c, "security", core.Command{Description: cli.T("cmd.security.long")}); !r.OK {
		return r
	}
	if r := addAlertsCommand(c, "security/alerts"); !r.OK {
		return r
	}
	if r := addDepsCommand(c, "security/deps"); !r.OK {
		return r
	}
	if r := addScanCommand(c, "security/scan"); !r.OK {
		return r
	}
	if r := addSecretsCommand(c, "security/secrets"); !r.OK {
		return r
	}
	if r := addJobsCommand(c, "security/jobs"); !r.OK {
		return r
	}
	return core.Ok(nil)
}

func registerSecurityCommand(c *core.Core, path string, command core.Command) core.Result {
	if c.Command(path).OK {
		return core.Ok(nil)
	}
	return c.Command(path, command)
}

func securitySelectionFlags() core.Options {
	return core.NewOptions(
		core.Option{Key: "registry", Value: ""},
		core.Option{Key: "repo", Value: ""},
		core.Option{Key: "severity", Value: ""},
		core.Option{Key: "json", Value: false},
		core.Option{Key: "target", Value: ""},
	)
}

func securitySelectionFromOptions(opts core.Options) SecuritySelectionOptions {
	return SecuritySelectionOptions{
		RegistryPath:   opts.String("registry"),
		RepositoryName: opts.String("repo"),
		SeverityFilter: opts.String("severity"),
		JSONOutput:     opts.Bool("json"),
		ExternalTarget: opts.String("target"),
	}
}

func scanCommandFlags() core.Options {
	flags := securitySelectionFlags()
	flags.Set("tool", "")
	return flags
}

func scanCommandFromOptions(opts core.Options) ScanCommandOptions {
	return ScanCommandOptions{
		Selection: securitySelectionFromOptions(opts),
		ToolName:  opts.String("tool"),
	}
}

func jobsCommandFlags() core.Options {
	return core.NewOptions(
		core.Option{Key: "registry", Value: ""},
		core.Option{Key: "targets", Value: ""},
		core.Option{Key: "issue-repo", Value: ""},
		core.Option{Key: "dry-run", Value: false},
		core.Option{Key: "copies", Value: 1},
	)
}

func jobsCommandFromOptions(opts core.Options) JobsCommandOptions {
	workerCount := opts.Int("copies")
	if workerCount == 0 {
		workerCount = 1
	}
	return JobsCommandOptions{
		RegistryPath:    opts.String("registry"),
		Targets:         opts.String("targets"),
		IssueRepository: opts.String("issue-repo"),
		DryRun:          opts.Bool("dry-run"),
		WorkerCount:     workerCount,
	}
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
			Path      string `json:"\x70ath"`
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

func loadRegistry(registryPath string) core.Result {
	if registryPath != "" {
		if !io.Local.Exists(registryPath) {
			return core.Fail(cli.Err("registry not found: %s", registryPath))
		}
		registry, err := repos.LoadRegistry(io.Local, registryPath)
		if err != nil {
			return core.Fail(cli.Wrap(err, "load registry"))
		}
		return core.Ok(registry)
	}

	path, err := repos.FindRegistry(io.Local)
	if err != nil {
		return core.Fail(cli.Wrap(err, "find registry"))
	}
	registry, err := repos.LoadRegistry(io.Local, path)
	if err != nil {
		return core.Fail(cli.Wrap(err, "load registry"))
	}
	return core.Ok(registry)
}

func checkGitHubCLI() core.Result {
	if _, err := execabs.LookPath("gh"); err != nil {
		return core.Fail(core.E("security", cli.T("error.gh_not_found"), nil))
	}
	return core.Ok(nil)
}

func runGitHubAPI(endpoint string) core.Result {
	return runGitHubAPIWithMode(endpoint, true)
}

func runGitHubAPIStrict(endpoint string) core.Result {
	return runGitHubAPIWithMode(endpoint, false)
}

func runGitHubAPIWithMode(endpoint string, allowMissingEndpoint bool) core.Result {
	var lastErr error
	for attempt := 0; attempt < githubAPIMaxAttempts; attempt++ {
		outputResult := runGitHubAPIRequest(endpoint)
		if outputResult.OK {
			return outputResult
		}

		err, _ := coreResultError(outputResult).(error)
		lastErr = err
		if allowMissingEndpoint && core.Is(err, errGitHubAPIEndpointNotFound) {
			return core.Ok([]byte("[]"))
		}

		if core.Is(err, errGitHubAPIAccessDenied) {
			return core.Fail(err)
		}

		if attempt == githubAPIMaxAttempts-1 || !isRetryableGitHubAPIError(err) {
			return core.Fail(cli.Wrap(lastErr, "run gh api"))
		}

		time.Sleep(githubAPIBaseBackoff << attempt)
	}

	return core.Fail(cli.Wrap(lastErr, "run gh api"))
}

func runGitHubAPIRequest(endpoint string) core.Result {
	ctx, cancel := context.WithTimeout(context.Background(), githubAPITimeout)
	defer cancel()

	cmd := execabs.CommandContext(ctx, "gh", "api", endpoint, "--paginate", "--slurp")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if core.Is(ctx.Err(), context.DeadlineExceeded) {
			return core.Fail(core.E("security.github.api", "GitHub API request timed out", errGitHubAPITimeout))
		}
		stderr := string(output)
		if core.Contains(stderr, "404") || core.Contains(stderr, "Not Found") {
			return core.Fail(errGitHubAPIEndpointNotFound)
		}
		if core.Contains(stderr, "403") || core.Contains(stderr, "Forbidden") {
			return core.Fail(core.E("security.github.api", "check token permissions", errGitHubAPIAccessDenied))
		}
		return core.Fail(err)
	}
	return core.Ok(trimGitHubJSONBytes(output))
}

func isRetryableGitHubAPIError(err error) bool {
	return !core.Is(err, errGitHubAPIEndpointNotFound) &&
		!core.Is(err, errGitHubAPIAccessDenied)
}

type githubRepoResponse struct {
	FullName string `json:"full_name"`
}

type githubRawMessage []byte

func trimGitHubJSONBytes(data []byte) []byte {
	return []byte(core.Trim(string(data)))
}

func coreResultError(value any) any {
	if r, ok := value.(core.Result); ok {
		if r.OK {
			return nil
		}
		value = r.Value
	}
	if err, ok := value.(error); ok {
		return err
	}
	return core.E("security.core.result", "operation failed", nil)
}

func decodeGitHubArrayItems(output []byte) core.Result {
	trimmed := trimGitHubJSONBytes(output)
	if len(trimmed) == 0 || string(trimmed) == "[]" {
		return core.Ok([]githubRawMessage(nil))
	}

	var pages []any
	if result := core.JSONUnmarshal(trimmed, &pages); !result.OK {
		err, _ := coreResultError(result).(error)
		return core.Fail(core.E("security", "parse GitHub API response", err))
	}

	items := make([]githubRawMessage, 0, len(pages))
	for _, page := range pages {
		rawPage := core.JSONMarshal(page)
		if !rawPage.OK {
			return rawPage
		}
		pageRaw := githubRawMessage(rawPage.Value.([]byte))
		pageData := trimGitHubJSONBytes(pageRaw)
		if len(pageData) == 0 || string(pageData) == "[]" {
			continue
		}

		if pageData[0] != '[' {
			items = append(items, githubRawMessage(pageData))
			continue
		}

		var pageItems []any
		if result := core.JSONUnmarshal(pageData, &pageItems); !result.OK {
			err, _ := coreResultError(result).(error)
			return core.Fail(core.E("security", "parse GitHub API page", err))
		}
		for _, pageItem := range pageItems {
			rawItem := core.JSONMarshal(pageItem)
			if !rawItem.OK {
				return rawItem
			}
			items = append(items, githubRawMessage(rawItem.Value.([]byte)))
		}
	}

	return core.Ok(items)
}

func decodeDependabotAlerts(output []byte) core.Result {
	itemsResult := decodeGitHubArrayItems(output)
	if !itemsResult.OK {
		return itemsResult
	}
	items := itemsResult.Value.([]githubRawMessage)

	alerts := make([]DependabotAlert, 0, len(items))
	for _, item := range items {
		var alert DependabotAlert
		if result := core.JSONUnmarshal(item, &alert); !result.OK {
			err, _ := coreResultError(result).(error)
			return core.Fail(core.E("security", "parse dependabot alert", err))
		}
		alerts = append(alerts, alert)
	}
	return core.Ok(alerts)
}

func decodeCodeScanningAlerts(output []byte) core.Result {
	itemsResult := decodeGitHubArrayItems(output)
	if !itemsResult.OK {
		return itemsResult
	}
	items := itemsResult.Value.([]githubRawMessage)

	alerts := make([]CodeScanningAlert, 0, len(items))
	for _, item := range items {
		var alert CodeScanningAlert
		if result := core.JSONUnmarshal(item, &alert); !result.OK {
			err, _ := coreResultError(result).(error)
			return core.Fail(core.E("security", "parse code scanning alert", err))
		}
		alerts = append(alerts, alert)
	}
	return core.Ok(alerts)
}

func decodeSecretScanningAlerts(output []byte) core.Result {
	itemsResult := decodeGitHubArrayItems(output)
	if !itemsResult.OK {
		return itemsResult
	}
	items := itemsResult.Value.([]githubRawMessage)

	alerts := make([]SecretScanningAlert, 0, len(items))
	for _, item := range items {
		var alert SecretScanningAlert
		if result := core.JSONUnmarshal(item, &alert); !result.OK {
			err, _ := coreResultError(result).(error)
			return core.Fail(core.E("security", "parse secret scanning alert", err))
		}
		alerts = append(alerts, alert)
	}
	return core.Ok(alerts)
}

func decodeGitHubRepositoryNames(output []byte) core.Result {
	itemsResult := decodeGitHubArrayItems(output)
	if !itemsResult.OK {
		return itemsResult
	}
	items := itemsResult.Value.([]githubRawMessage)

	names := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		var repository githubRepoResponse
		if result := core.JSONUnmarshal(item, &repository); !result.OK {
			err, _ := coreResultError(result).(error)
			return core.Fail(core.E("security", "parse GitHub repository", err))
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
	return core.Ok(names)
}

func combineSecurityCollectorErrors(target string, collectorErrors map[string]error) core.Result {
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
		return core.Ok(nil)
	}

	slices.SortFunc(failures, func(a, b collectorFailure) int {
		return cmp.Compare(a.name, b.name)
	})

	missingCollectors := make([]string, 0, len(failures))
	messages := make([]string, 0, len(failures))
	for _, failure := range failures {
		missingCollectors = append(missingCollectors, failure.name)
		messages = append(messages, core.Sprintf("%s: %v", failure.name, failure.err))
	}

	return core.Fail(core.E("security", core.Sprintf("failed to fetch %s for %s: %s",
		core.Join(", ", missingCollectors...),
		target,
		core.Join("; ", messages...),
	), nil))
}

func combineSecurityTargetErrors(commandName string, targetErrors map[string]error) core.Result {
	if len(targetErrors) == 0 {
		return core.Ok(nil)
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

	return core.Fail(core.E("security", core.Sprintf("%s failed for %d target(s): %s",
		commandName,
		len(targetNames),
		core.Join("; ", messages...),
	), nil))
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
