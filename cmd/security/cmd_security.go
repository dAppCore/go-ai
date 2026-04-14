package security

import (
	"os/exec"
	"slices"

	"dappco.re/go/core"
	"dappco.re/go/core/i18n"
	"dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/scm/repos"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var (
	// Command flags
	securityRegistryPath string
	securityRepo         string
	securitySeverity     string
	securityJSON         bool
	securityTarget       string // External repo target (e.g. "wailsapp/wails")
)

// AddSecurityCommands(root) registers the top-level security alerts, deps, scan, secrets, and jobs commands.
func AddSecurityCommands(root *cli.Command) {
	if hasCommand(root, "security") {
		return
	}

	secCmd := &cli.Command{
		Use:   "security",
		Short: i18n.T("cmd.security.short"),
		Long:  i18n.T("cmd.security.long"),
	}

	addAlertsCommand(secCmd)
	addDepsCommand(secCmd)
	addScanCommand(secCmd)
	addSecretsCommand(secCmd)
	addJobsCommand(secCmd)

	root.AddCommand(secCmd)
}

func hasCommand(parent *cli.Command, name string) bool {
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

// loadRegistry loads the repository registry.
func loadRegistry(registryPath string) (*repos.Registry, error) {
	if registryPath != "" {
		reg, err := repos.LoadRegistry(io.Local, registryPath)
		if err != nil {
			return nil, cli.Wrap(err, "load registry")
		}
		return reg, nil
	}

	path, err := repos.FindRegistry(io.Local)
	if err != nil {
		return nil, cli.Wrap(err, "find registry")
	}
	reg, err := repos.LoadRegistry(io.Local, path)
	if err != nil {
		return nil, cli.Wrap(err, "load registry")
	}
	return reg, nil
}

// checkGitHubCLI() verifies that the GitHub CLI is installed before a command tries to call the GitHub API.
func checkGitHubCLI() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return coreerr.E("security.checkGitHubCLI", i18n.T("error.gh_not_found"), nil)
	}
	return nil
}

// runGitHubAPI("repos/core/go-ai/dependabot/alerts?state=open") returns the paginated GitHub API response body.
func runGitHubAPI(endpoint string) ([]byte, error) {
	cmd := exec.Command("gh", "api", endpoint, "--paginate")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if core.Contains(stderr, "404") || core.Contains(stderr, "Not Found") {
				return []byte("[]"), nil
			}
			if core.Contains(stderr, "403") {
				return nil, coreerr.E("security.runGitHubAPI", "access denied (check token permissions)", nil)
			}
		}
		return nil, cli.Wrap(err, "run gh api")
	}
	return output, nil
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
