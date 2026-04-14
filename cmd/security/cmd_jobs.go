package security

import (
	"cmp"
	"os/exec"
	"slices"
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/ai/pkg/ai"
	"dappco.re/go/core/i18n"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/scm/repos"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var (
	jobsTargets   string
	jobsIssueRepo string
	jobsDryRun    bool
	jobsCopies    int
)

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
	cmd := &cli.Command{
		Use:   "jobs",
		Short: i18n.T("cmd.security.jobs.short"),
		Long:  i18n.T("cmd.security.jobs.long"),
		RunE: func(c *cli.Command, args []string) error {
			return runJobs()
		},
	}

	cmd.Flags().StringVar(&securityRegistryPath, "registry", "", i18n.T("common.flag.registry"))
	cmd.Flags().StringVar(&jobsTargets, "targets", "", i18n.T("cmd.security.jobs.flag.targets"))
	cmd.Flags().StringVar(&jobsIssueRepo, "issue-repo", "", i18n.T("cmd.security.jobs.flag.issue_repo"))
	cmd.Flags().BoolVar(&jobsDryRun, "dry-run", false, i18n.T("cmd.security.jobs.flag.dry_run"))
	cmd.Flags().IntVar(&jobsCopies, "copies", 1, i18n.T("cmd.security.jobs.flag.copies"))

	parent.AddCommand(cmd)
}

func runJobs() error {
	if err := checkGH(); err != nil {
		return err
	}
	if jobsCopies < 1 {
		return cli.Err("--copies must be at least 1")
	}

	reg, err := loadRegistryForJobs(jobsTargets)
	if err != nil {
		return err
	}

	targets, err := resolveJobTargets(jobsTargets, reg)
	if err != nil {
		return err
	}
	if jobsDryRun {
		cli.Blank()
		cli.Print("%s %d\n", cli.DimStyle.Render("Workers:"), jobsCopies)
		for _, target := range targets {
			cli.Print("%s %s\n", cli.DimStyle.Render("[dry-run] Would scan:"), target)
		}
		if jobsIssueRepo != "" {
			cli.Print("%s %s\n", cli.DimStyle.Render("[dry-run] Would create summary issue in:"), jobsIssueRepo)
		}
		cli.Blank()
		return nil
	}

	results := runJobWorkers(targets, jobsCopies)
	var successful []jobRepoResult
	overall := &AlertSummary{}
	for _, result := range results {
		if result.err != nil {
			cli.Print("%s %v\n", cli.WarningStyle.Render(">>"), result.err)
			continue
		}

		successful = append(successful, result.repo)
		mergeAlertSummary(overall, &result.repo.Summary)
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

	if jobsIssueRepo != "" {
		title := "Security scan summary: " + time.Now().Format("2006-01-02")
		body := buildJobsIssueBody(overall, successful)
		issueURL, err := createJobsIssue(jobsIssueRepo, title, body)
		if err != nil {
			return err
		}

		cli.Print("%s %s\n", cli.SuccessStyle.Render(">>"), issueURL)
		_ = ai.Record(ai.Event{
			Type:      "security.jobs",
			Timestamp: time.Now(),
			Repo:      jobsIssueRepo,
			Data: map[string]any{
				"issue_repo": jobsIssueRepo,
				"issue_url":  issueURL,
				"targets":    len(successful),
				"total":      overall.Total,
				"critical":   overall.Critical,
				"high":       overall.High,
				"medium":     overall.Medium,
				"low":        overall.Low,
			},
		})
	}

	return nil
}

func loadRegistryForJobs(targets string) (*repos.Registry, error) {
	if !jobsNeedRegistry(targets) {
		return nil, nil
	}
	reg, err := loadRegistry(securityRegistryPath)
	if err != nil {
		return nil, err
	}
	return reg, nil
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

func resolveJobTargets(targets string, reg *repos.Registry) ([]string, error) {
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
		if reg == nil {
			return nil, cli.Err("--targets=all requires a repository registry")
		}
		for _, repo := range reg.List() {
			addTarget(core.Sprintf("%s/%s", reg.Org, repo.Name))
		}
		return resolved, nil
	}

	for _, part := range core.Split(trimmed, ",") {
		token := core.Trim(part)
		if token == "" {
			continue
		}
		if core.Contains(token, "/") {
			if _, fullName := buildTargetRepo(token); fullName == "" {
				return nil, cli.Err("invalid target format: use owner/repo")
			}
			addTarget(token)
			continue
		}
		if reg == nil {
			return nil, cli.Err("registry-backed target %q requires a repository registry", token)
		}
		repo, ok := reg.Get(token)
		if !ok {
			return nil, cli.Err("repo not found: %s", token)
		}
		addTarget(core.Sprintf("%s/%s", reg.Org, repo.Name))
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
	if _, fullName := buildTargetRepo(target); fullName == "" {
		return jobRepoResult{}, coreerr.E("security.jobs", "invalid target format: use owner/repo", nil)
	}

	repo := jobRepoResult{Repo: target}
	var fetchErrors int

	codeAlerts, err := fetchCodeScanningAlerts(target)
	if err != nil {
		fetchErrors++
	} else {
		for _, alert := range codeAlerts {
			if alert.State != "open" {
				continue
			}
			severity := alert.Rule.Severity
			if severity == "" {
				severity = "medium"
			}
			repo.Summary.Add(severity)
			repo.Findings = append(repo.Findings, core.Sprintf("[%s] code-scanning: %s (%s:%d)",
				core.Upper(severity),
				alert.Rule.Description,
				alert.MostRecentInstance.Location.Path,
				alert.MostRecentInstance.Location.StartLine,
			))
		}
	}

	depAlerts, err := fetchDependabotAlerts(target)
	if err != nil {
		fetchErrors++
	} else {
		for _, alert := range depAlerts {
			if alert.State != "open" {
				continue
			}
			repo.Summary.Add(alert.Advisory.Severity)
			repo.Findings = append(repo.Findings, core.Sprintf("[%s] dependabot: %s (%s)",
				core.Upper(alert.Advisory.Severity),
				alert.Advisory.Summary,
				alert.Advisory.CVEID,
			))
		}
	}

	secretAlerts, err := fetchSecretScanningAlerts(target)
	if err != nil {
		fetchErrors++
	} else {
		for _, alert := range secretAlerts {
			if alert.State != "open" {
				continue
			}
			repo.Summary.Add("high")
			repo.Findings = append(repo.Findings, core.Sprintf("[HIGH] secret-scanning: %s (#%d)", alert.SecretType, alert.Number))
		}
	}

	if fetchErrors == 3 {
		return jobRepoResult{}, coreerr.E("security.jobs", "failed to fetch any alerts for "+target, nil)
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

func createJobsIssue(issueRepo, title, body string) (string, error) {
	cmd := exec.Command("gh", "issue", "create",
		"--repo", issueRepo,
		"--title", title,
		"--body", body,
		"--label", "type:security-scan",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", cli.Wrap(err, "create summary issue: "+string(output))
	}
	return core.Trim(string(output)), nil
}

func buildJobsIssueBody(summary *AlertSummary, repos []jobRepoResult) string {
	sb := core.NewBuilder()

	sb.WriteString("## Security Scan Summary\n\n")
	sb.WriteString("Summary: " + summary.PlainString() + "\n\n")
	sb.WriteString("### Repositories\n\n")
	for _, repo := range repos {
		sb.WriteString("- " + repo.Repo + " — " + repo.Summary.PlainString() + "\n")
		for i, finding := range repo.Findings {
			if i == 3 {
				sb.WriteString("  - ...\n")
				break
			}
			sb.WriteString("  - " + finding + "\n")
		}
	}

	sb.WriteString("\n### Checklist\n\n")
	sb.WriteString("- [ ] Triage critical and high findings first\n")
	sb.WriteString("- [ ] Create fix PRs for affected repositories\n")
	sb.WriteString("- [ ] Re-run security scans after remediation\n")

	return sb.String()
}
