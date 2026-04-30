package security

import (
	"cmp"
	"os/exec"
	"slices"
	"time"

	"dappco.re/go"
	"dappco.re/go/ai/ai"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/i18n"
	coreerr "dappco.re/go/log"
	"dappco.re/go/scm/repos"
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
		Short: i18n.T("cmd.security.jobs.short"),
		Long:  i18n.T("cmd.security.jobs.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runJobs(*commandOptions)
		},
	}

	cmd.Flags().StringVar(&commandOptions.RegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&commandOptions.Targets, "targets", "", i18n.T("cmd.security.jobs.flag.targets"))
	cmd.Flags().StringVar(&commandOptions.IssueRepository, "issue-repo", "", i18n.T("cmd.security.jobs.flag.issue_repo"))
	cmd.Flags().BoolVar(&commandOptions.DryRun, "dry-run", false, i18n.T("cmd.security.jobs.flag.dry_run"))
	cmd.Flags().IntVar(&commandOptions.WorkerCount, "copies", commandOptions.WorkerCount, i18n.T("cmd.security.jobs.flag.copies"))

	parent.AddCommand(cmd)
}

func runJobs(commandOptions JobsCommandOptions) error {
	startedAt := time.Now()

	if commandOptions.WorkerCount < 1 {
		return cli.Err("--copies must be at least 1")
	}

	issueRepoTarget, err := validateJobsIssueRepository(commandOptions.IssueRepository)
	if err != nil {
		return err
	}

	registry, err := loadRegistryForJobs(commandOptions)
	if err != nil {
		return err
	}

	if commandOptions.DryRun {
		plannedTargets, err := resolveJobTargetsForDryRun(commandOptions.Targets, registry)
		if err != nil {
			return err
		}
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
		return nil
	}

	// Validate the target specification before any gh invocation.
	if _, err := resolveJobTargetsForDryRun(commandOptions.Targets, registry); err != nil {
		return err
	}

	if err := checkGitHubCLI(); err != nil {
		return err
	}

	targets, err := resolveJobTargets(commandOptions.Targets, registry)
	if err != nil {
		return err
	}
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
	if err := combineSecurityTargetErrors("security jobs", targetErrors); err != nil {
		return err
	}
	if len(successful) == 0 {
		return cli.Err("all targets failed to process")
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
		issueURL, err := createJobsIssue(issueRepoTarget.FullName, title, body)
		if err != nil {
			return err
		}

		cli.Print("%s %s\n", cli.SuccessStyle.Render(">>"), issueURL)
		event := buildJobsMetricsEvent(commandOptions, overall, successful, issueURL)
		event.Duration = time.Since(startedAt)
		recordSecurityMetricsEvent(event)
		return nil
	}

	event := buildJobsMetricsEvent(commandOptions, overall, successful, "")
	event.Duration = time.Since(startedAt)
	recordSecurityMetricsEvent(event)
	return nil
}

func validateJobsIssueRepository(issueRepository string) (SecurityTarget, error) {
	if core.Trim(issueRepository) == "" {
		return SecurityTarget{}, nil
	}

	target, err := parseSecurityTarget(issueRepository)
	if err != nil {
		return SecurityTarget{}, cli.Err("invalid --issue-repo format: use owner/repo")
	}
	return target, nil
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

func loadRegistryForJobs(commandOptions JobsCommandOptions) (*repos.Registry, error) {
	if core.Trim(commandOptions.Targets) == "" {
		return nil, nil
	}
	if !jobsNeedRegistry(commandOptions.Targets) {
		return nil, nil
	}
	registry, err := loadRegistry(commandOptions.RegistryPath)
	if err != nil {
		return nil, err
	}
	return registry, nil
}

func resolveJobTargetsForDryRun(targets string, registry *repos.Registry) ([]string, error) {
	trimmed := core.Trim(targets)
	if trimmed == "" {
		return nil, cli.Err("at least one --targets value required (comma-separated repo list or all)")
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
			return nil, cli.Err("--targets=all requires a repository registry for dry-run")
		}
		if len(registry.List()) == 0 {
			return nil, cli.Err("no repositories found for GitHub org: %s", registry.Org)
		}
		for _, repo := range registry.List() {
			addTarget(core.Sprintf("%s/%s", registry.Org, repo.Name))
		}
		return resolved, nil
	}

	for _, part := range core.Split(trimmed, ",") {
		token := core.Trim(part)
		if token == "" {
			continue
		}
		if core.Contains(token, "/") {
			target, err := parseSecurityTarget(token)
			if err != nil {
				return nil, cli.Err("invalid target format: use owner/repo")
			}
			addTarget(target.FullName)
			continue
		}
		if registry == nil {
			return nil, cli.Err("registry-backed target %q requires a repository registry", token)
		}
		repo, ok := registry.Get(token)
		if !ok {
			return nil, cli.Err("repo not found: %s", token)
		}
		addTarget(core.Sprintf("%s/%s", registry.Org, repo.Name))
	}

	if len(resolved) == 0 {
		return nil, cli.Err("no targets resolved from --targets")
	}
	return resolved, nil
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

func resolveJobTargets(targets string, registry *repos.Registry) ([]string, error) {
	trimmed := core.Trim(targets)
	if trimmed == "" {
		return nil, cli.Err("at least one --targets value required (comma-separated repo list or all)")
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
			return nil, cli.Err("--targets=all requires a repository registry")
		}
		liveTargets, err := listGitHubOrgTargets(registry.Org)
		if err != nil {
			return nil, err
		}
		if len(liveTargets) == 0 {
			return nil, cli.Err("no repositories found for GitHub org: %s", registry.Org)
		}
		return liveTargets, nil
	}

	for _, part := range core.Split(trimmed, ",") {
		token := core.Trim(part)
		if token == "" {
			continue
		}
		if core.Contains(token, "/") {
			target, err := parseSecurityTarget(token)
			if err != nil {
				return nil, cli.Err("invalid target format: use owner/repo")
			}
			addTarget(target.FullName)
			continue
		}
		if registry == nil {
			return nil, cli.Err("registry-backed target %q requires a repository registry", token)
		}
		repo, ok := registry.Get(token)
		if !ok {
			return nil, cli.Err("repo not found: %s", token)
		}
		addTarget(core.Sprintf("%s/%s", registry.Org, repo.Name))
	}

	if len(resolved) == 0 {
		return nil, cli.Err("no targets resolved from --targets")
	}
	return resolved, nil
}

func runJobWorkers(targets []string, workers int) []jobResult {
	jobCh := make(chan string)
	resultCh := make(chan jobResult, len(targets))

	for range workers {
		go func() {
			for target := range jobCh {
				repo, err := collectJobRepoResult(target)
				resultCh <- jobResult{repo: repo, err: err}
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

func collectJobRepoResult(target string) (jobRepoResult, error) {
	securityTarget, err := parseSecurityTarget(target)
	if err != nil {
		return jobRepoResult{}, coreerr.E("security", "invalid target format: use owner/repo", nil)
	}

	repo := jobRepoResult{Repo: target}
	dependabotAlerts, dependabotError := collectDependabotAlertsForJobs(securityTarget, "")
	codeScanningAlerts, codeScanningError := collectCodeScanningAlertsForJobs(securityTarget, ScanCommandOptions{})
	secretScanningAlerts, secretScanningError := collectSecretScanningAlertsForJobs(securityTarget)

	if dependabotError != nil || codeScanningError != nil || secretScanningError != nil {
		return jobRepoResult{}, combineSecurityCollectorErrors(target, map[string]error{
			"dependabot":      dependabotError,
			"code-scanning":   codeScanningError,
			"secret-scanning": secretScanningError,
		})
	}

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

	return repo, nil
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

func createJobsIssue(issueRepo, title, body string) (string, error) {
	cmd := exec.Command("gh",
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
		return "", cli.Wrap(err, message)
	}
	return core.Trim(string(output)), nil
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
