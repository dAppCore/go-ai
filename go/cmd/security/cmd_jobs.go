package security

import (
	"cmp"
	"slices"
	"time"

	"dappco.re/go"
	"dappco.re/go/ai/ai"
	"dappco.re/go/cli/pkg/cli"
	coreerr "dappco.re/go/log"
	"dappco.re/go/scm/repos"
	execabs "golang.org/x/sys/execabs"
)

var (
	collectDependabotAlertsForJobs     = collectDepAlerts
	collectCodeScanningAlertsForJobs   = collectScanAlerts
	collectSecretScanningAlertsForJobs = collectSecretAlerts
)

const maxSecurityJobWorkers = 32

type jobRepoResult struct {
	Repo     string
	Summary  AlertSummary
	Findings []string
}

type jobResult struct {
	repo jobRepoResult
	err  error
}

func addJobsCommand(parent *cli.Command) {
	commandOptions := &JobsCommandOptions{
		WorkerCount: 1,
	}

	cmd := &cli.Command{
		Use:   "jobs",
		Short: cli.T("cmd.security.jobs.short"),
		Long:  cli.T("cmd.security.jobs.long"),
		RunE: func(c *cli.Command, args []string) error {
			r := runJobs(*commandOptions)
			if !r.OK {
				err, _ := coreResultError(r).(error)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&commandOptions.RegistryPath, "registry", "", cli.T("common.flag.registry"))
	cmd.Flags().StringVar(&commandOptions.Targets, "targets", "", cli.T("cmd.security.jobs.flag.targets"))
	cmd.Flags().StringVar(&commandOptions.IssueRepository, "issue-repo", "", cli.T("cmd.security.jobs.flag.issue_repo"))
	cmd.Flags().BoolVar(&commandOptions.DryRun, "dry-run", false, cli.T("cmd.security.jobs.flag.dry_run"))
	cmd.Flags().IntVar(&commandOptions.WorkerCount, "copies", commandOptions.WorkerCount, cli.T("cmd.security.jobs.flag.copies"))

	parent.AddCommand(cmd)
}

func runJobs(commandOptions JobsCommandOptions) core.Result {
	startedAt := time.Now()

	if commandOptions.WorkerCount < 1 {
		return core.Fail(cli.Err("--copies must be at least 1"))
	}

	issueRepoTargetResult := validateJobsIssueRepository(commandOptions.IssueRepository)
	if !issueRepoTargetResult.OK {
		return issueRepoTargetResult
	}
	issueRepoTarget := issueRepoTargetResult.Value.(SecurityTarget)

	registryResult := loadRegistryForJobs(commandOptions)
	if !registryResult.OK {
		return registryResult
	}
	registry, _ := registryResult.Value.(*repos.Registry)

	if commandOptions.DryRun {
		plannedTargetsResult := resolveJobTargetsForDryRun(commandOptions.Targets, registry)
		if !plannedTargetsResult.OK {
			return plannedTargetsResult
		}
		plannedTargets := plannedTargetsResult.Value.([]string)
		workerCount := normalizeJobWorkerCount(commandOptions.WorkerCount, len(plannedTargets))

		// Dry-run only needs target resolution; it should not require `gh` to be installed or call the GitHub API.
		cli.Blank()
		cli.Print("%s\n", cli.DimStyle.Render(core.Sprintf("Workers: %d", workerCount)))
		for _, target := range plannedTargets {
			cli.Print("%s\n", cli.DimStyle.Render(core.Sprintf("[dry-run] Would scan: %s", target)))
		}
		if issueRepoTarget.FullName != "" {
			cli.Print("%s\n", cli.DimStyle.Render(core.Sprintf("[dry-run] Would create summary issue in: %s", issueRepoTarget.FullName)))
		}
		cli.Blank()
		return core.Ok(nil)
	}

	// Validate the target specification before any gh invocation.
	if r := resolveJobTargetsForDryRun(commandOptions.Targets, registry); !r.OK {
		return r
	}

	if r := checkGitHubCLI(); !r.OK {
		return r
	}

	targetsResult := resolveJobTargets(commandOptions.Targets, registry)
	if !targetsResult.OK {
		return targetsResult
	}
	targets := targetsResult.Value.([]string)
	workerCount := normalizeJobWorkerCount(commandOptions.WorkerCount, len(targets))

	results := runJobWorkers(targets, workerCount)
	var successful []jobRepoResult
	overall := &AlertSummary{}
	targetErrors := map[string]error{}
	for _, result := range results {
		if result.err != nil {
			targetErrors[result.repo.Repo] = result.err
			continue
		}

		successful = append(successful, result.repo)
		mergeAlertSummary(overall, &result.repo.Summary)
	}
	if r := combineSecurityTargetErrors("security jobs", targetErrors); !r.OK {
		return r
	}
	if len(successful) == 0 {
		return core.Fail(cli.Err("all targets failed to process"))
	}

	cli.Blank()
	cli.Print("%s %s\n", cli.DimStyle.Render("Security jobs summary:"), overall.String())
	for _, repo := range successful {
		cli.Print("  %-32s %s\n", repo.Repo, repo.Summary.PlainString())
	}
	cli.Blank()

	if issueRepoTarget.FullName != "" {
		title := "Security scan summary: " + time.Now().Format("2006-01-02")
		body := buildJobsIssueBody(overall, successful)
		issueURLResult := createJobsIssue(issueRepoTarget.FullName, title, body)
		if !issueURLResult.OK {
			return issueURLResult
		}
		issueURL := issueURLResult.Value.(string)

		cli.Print("%s %s\n", cli.SuccessStyle.Render(">>"), issueURL)
		event := buildJobsMetricsEvent(commandOptions, overall, successful, issueURL)
		event.Duration = time.Since(startedAt)
		recordSecurityMetricsEvent(event)
		return core.Ok(nil)
	}

	event := buildJobsMetricsEvent(commandOptions, overall, successful, "")
	event.Duration = time.Since(startedAt)
	recordSecurityMetricsEvent(event)
	return core.Ok(nil)
}

func validateJobsIssueRepository(issueRepository string) core.Result {
	if core.Trim(issueRepository) == "" {
		return core.Ok(SecurityTarget{})
	}

	targetResult := parseSecurityTarget(issueRepository)
	if !targetResult.OK {
		return core.Fail(cli.Err("invalid --issue-repo format: use owner/repo"))
	}
	return targetResult
}

func normalizeJobWorkerCount(requested, targetCount int) int {
	workerCount := requested
	if workerCount > targetCount {
		workerCount = targetCount
	}
	if workerCount > maxSecurityJobWorkers {
		workerCount = maxSecurityJobWorkers
	}
	if workerCount < 1 {
		return 1
	}
	return workerCount
}

func loadRegistryForJobs(commandOptions JobsCommandOptions) core.Result {
	if core.Trim(commandOptions.Targets) == "" {
		return core.Ok((*repos.Registry)(nil))
	}
	if !jobsNeedRegistry(commandOptions.Targets) {
		return core.Ok((*repos.Registry)(nil))
	}
	registryResult := loadRegistry(commandOptions.RegistryPath)
	if !registryResult.OK {
		return registryResult
	}
	return registryResult
}

func resolveJobTargetsForDryRun(targets string, registry *repos.Registry) core.Result {
	trimmed := core.Trim(targets)
	if trimmed == "" {
		return core.Fail(cli.Err("at least one --targets value required (comma-separated repo list or all)"))
	}

	seen := map[string]struct{}{}
	var resolved []string
	addTarget := func(target string) {
		if _, ok := seen[target]; ok {
			return
		}
		seen[target] = struct{}{}
		resolved = append(resolved, target)
	}

	if trimmed == "all" {
		if registry == nil {
			return core.Fail(cli.Err("--targets=all requires a repository registry for dry-run"))
		}
		if len(registry.List()) == 0 {
			return core.Fail(cli.Err("no repositories found for GitHub org: %s", registry.Org))
		}
		for _, repo := range registry.List() {
			addTarget(core.Sprintf("%s/%s", registry.Org, repo.Name))
		}
		return core.Ok(resolved)
	}

	for _, part := range core.Split(trimmed, ",") {
		token := core.Trim(part)
		if token == "" {
			continue
		}
		if core.Contains(token, "/") {
			targetResult := parseSecurityTarget(token)
			if !targetResult.OK {
				return core.Fail(cli.Err("invalid target format: use owner/repo"))
			}
			addTarget(targetResult.Value.(SecurityTarget).FullName)
			continue
		}
		if registry == nil {
			return core.Fail(cli.Err("registry-backed target %q requires a repository registry", token))
		}
		repo, ok := registry.Get(token)
		if !ok {
			return core.Fail(cli.Err("repo not found: %s", token))
		}
		addTarget(core.Sprintf("%s/%s", registry.Org, repo.Name))
	}

	if len(resolved) == 0 {
		return core.Fail(cli.Err("no targets resolved from --targets"))
	}
	return core.Ok(resolved)
}

func jobsNeedRegistry(targets string) bool {
	trimmed := core.Trim(targets)
	if trimmed == "" || trimmed == "all" {
		return true
	}

	for _, part := range core.Split(trimmed, ",") {
		token := core.Trim(part)
		if token == "" {
			continue
		}
		if !core.Contains(token, "/") {
			return true
		}
	}
	return false
}

func resolveJobTargets(targets string, registry *repos.Registry) core.Result {
	trimmed := core.Trim(targets)
	if trimmed == "" {
		return core.Fail(cli.Err("at least one --targets value required (comma-separated repo list or all)"))
	}

	seen := map[string]struct{}{}
	var resolved []string
	addTarget := func(target string) {
		if _, ok := seen[target]; ok {
			return
		}
		seen[target] = struct{}{}
		resolved = append(resolved, target)
	}

	if trimmed == "all" {
		if registry == nil {
			return core.Fail(cli.Err("--targets=all requires a repository registry"))
		}
		liveTargetsResult := listGitHubOrgTargets(registry.Org)
		if !liveTargetsResult.OK {
			return liveTargetsResult
		}
		liveTargets := liveTargetsResult.Value.([]string)
		if len(liveTargets) == 0 {
			return core.Fail(cli.Err("no repositories found for GitHub org: %s", registry.Org))
		}
		return core.Ok(liveTargets)
	}

	for _, part := range core.Split(trimmed, ",") {
		token := core.Trim(part)
		if token == "" {
			continue
		}
		if core.Contains(token, "/") {
			targetResult := parseSecurityTarget(token)
			if !targetResult.OK {
				return core.Fail(cli.Err("invalid target format: use owner/repo"))
			}
			addTarget(targetResult.Value.(SecurityTarget).FullName)
			continue
		}
		if registry == nil {
			return core.Fail(cli.Err("registry-backed target %q requires a repository registry", token))
		}
		repo, ok := registry.Get(token)
		if !ok {
			return core.Fail(cli.Err("repo not found: %s", token))
		}
		addTarget(core.Sprintf("%s/%s", registry.Org, repo.Name))
	}

	if len(resolved) == 0 {
		return core.Fail(cli.Err("no targets resolved from --targets"))
	}
	return core.Ok(resolved)
}

func runJobWorkers(targets []string, workers int) []jobResult {
	jobCh := make(chan string)
	resultCh := make(chan jobResult, len(targets))

	for range workers {
		go func() {
			for target := range jobCh {
				repoResult := collectJobRepoResult(target)
				if !repoResult.OK {
					err, _ := coreResultError(repoResult).(error)
					resultCh <- jobResult{repo: jobRepoResult{Repo: target}, err: err}
					continue
				}
				resultCh <- jobResult{repo: repoResult.Value.(jobRepoResult)}
			}
		}()
	}

	for _, target := range targets {
		jobCh <- target
	}
	close(jobCh)

	results := make([]jobResult, 0, len(targets))
	for range targets {
		results = append(results, <-resultCh)
	}

	slices.SortFunc(results, func(a, b jobResult) int {
		return cmp.Compare(a.repo.Repo, b.repo.Repo)
	})
	return results
}

func collectJobRepoResult(target string) core.Result {
	securityTargetResult := parseSecurityTarget(target)
	if !securityTargetResult.OK {
		return core.Fail(coreerr.E("security", "invalid target format: use owner/repo", nil))
	}
	securityTarget := securityTargetResult.Value.(SecurityTarget)

	repo := jobRepoResult{Repo: target}
	dependabotResult := collectDependabotAlertsForJobs(securityTarget, "")
	codeScanningResult := collectCodeScanningAlertsForJobs(securityTarget, ScanCommandOptions{})
	secretScanningResult := collectSecretScanningAlertsForJobs(securityTarget)

	if !dependabotResult.OK || !codeScanningResult.OK || !secretScanningResult.OK {
		dependabotError, _ := coreResultError(dependabotResult).(error)
		codeScanningError, _ := coreResultError(codeScanningResult).(error)
		secretScanningError, _ := coreResultError(secretScanningResult).(error)
		r := combineSecurityCollectorErrors(target, map[string]error{
			"dependabot":      dependabotError,
			"code-scanning":   codeScanningError,
			"secret-scanning": secretScanningError,
		})
		return r
	}

	dependabotAlerts := dependabotResult.Value.([]DepAlert)
	codeScanningAlerts := codeScanningResult.Value.([]ScanAlert)
	secretScanningAlerts := secretScanningResult.Value.([]SecretAlert)
	for _, alert := range buildAlertOutputs(dependabotAlerts, codeScanningAlerts, secretScanningAlerts, "") {
		repo.Summary.Add(alert.Severity)
	}

	for _, alert := range codeScanningAlerts {
		repo.Findings = append(repo.Findings, core.Sprintf("[%s] code-scanning: %s (%s:%d)",
			core.Upper(alert.Severity),
			alert.Description,
			alert.Path,
			alert.Line,
		))
	}

	for _, alert := range dependabotAlerts {
		repo.Findings = append(repo.Findings, core.Sprintf("[%s] dependabot: %s (%s)",
			core.Upper(alert.Severity),
			alert.Summary,
			alert.CVE,
		))
	}

	for _, alert := range secretScanningAlerts {
		repo.Findings = append(repo.Findings, core.Sprintf("[HIGH] secret-scanning: %s (#%d)", alert.SecretType, alert.Number))
	}

	return core.Ok(repo)
}

func mergeAlertSummary(dst, src *AlertSummary) {
	dst.Critical += src.Critical
	dst.High += src.High
	dst.Medium += src.Medium
	dst.Low += src.Low
	dst.Unknown += src.Unknown
	dst.Total += src.Total
}

func buildJobsMetricsEvent(commandOptions JobsCommandOptions, summary *AlertSummary, repos []jobRepoResult, issueURL string) ai.Event {
	repositoryNames := make([]string, 0, len(repos))
	for _, repository := range repos {
		repositoryNames = append(repositoryNames, repository.Repo)
	}

	eventRepository := ""
	switch {
	case commandOptions.IssueRepository != "":
		eventRepository = commandOptions.IssueRepository
	case len(repositoryNames) == 1:
		eventRepository = repositoryNames[0]
	}

	data := map[string]any{
		"target_spec": commandOptions.Targets,
		"targets":     len(repos),
		"repos":       repositoryNames,
		"total":       summary.Total,
		"critical":    summary.Critical,
		"high":        summary.High,
		"medium":      summary.Medium,
		"low":         summary.Low,
		"unknown":     summary.Unknown,
	}
	if commandOptions.IssueRepository != "" {
		data["issue_repo"] = commandOptions.IssueRepository
	}
	if issueURL != "" {
		data["issue_url"] = issueURL
	}

	return ai.Event{
		Type: "security.jobs",
		Repo: eventRepository,
		Data: data,
	}
}

func createJobsIssue(issueRepo, title, body string) core.Result {
	cmd := execabs.Command("gh",
		"issue", "create",
		"--repo", issueRepo,
		"--title", title,
		"--body", body,
		"--label", "type:security-scan",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := "create summary issue"
		if text := core.Trim(string(output)); text != "" {
			message += ": " + text
		}
		return core.Fail(cli.Wrap(err, message))
	}
	return core.Ok(core.Trim(string(output)))
}

func buildJobsIssueBody(summary *AlertSummary, repos []jobRepoResult) string {
	builder := core.NewBuilder()

	builder.WriteString("## Security Scan Summary\n\n")
	builder.WriteString("Summary: " + summary.PlainString() + "\n\n")
	builder.WriteString("### Repositories\n\n")
	for _, repository := range repos {
		builder.WriteString("- " + repository.Repo + " — " + repository.Summary.PlainString() + "\n")
		for findingIndex, finding := range repository.Findings {
			if findingIndex == 3 {
				builder.WriteString("  - ...\n")
				break
			}
			builder.WriteString("  - " + finding + "\n")
		}
	}

	builder.WriteString("\n### Checklist\n\n")
	builder.WriteString("- [ ] Triage critical and high findings first\n")
	builder.WriteString("- [ ] Create fix PRs for affected repositories\n")
	builder.WriteString("- [ ] Re-run security scans after remediation\n")

	return builder.String()
}
