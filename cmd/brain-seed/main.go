// SPDX-License-Identifier: EUPL-1.2

// brain-seed imports Claude Code MEMORY.md files into the OpenBrain knowledge
// store via the MCP HTTP API (brain_remember tool). The Laravel app handles
// embedding, Qdrant storage, and MariaDB dual-write internally.
//
// Usage:
//
//	go run ./cmd/brain-seed -api-key YOUR_KEY
//	go run ./cmd/brain-seed -api-key YOUR_KEY -api https://lthn.sh/api/v1/mcp
//	go run ./cmd/brain-seed -api-key YOUR_KEY -dry-run
//	go run ./cmd/brain-seed -api-key YOUR_KEY -plans
//	go run ./cmd/brain-seed -api-key YOUR_KEY -claude-md  # Also import CLAUDE.md files
package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	apiURL     = flag.String("api", "https://lthn.sh/api/v1/mcp", "MCP API base URL")
	apiKey     = flag.String("api-key", "", "MCP API key (Bearer token)")
	server     = flag.String("server", "hosthub-agent", "MCP server ID")
	agent      = flag.String("agent", "charon", "Agent ID for attribution")
	dryRun     = flag.Bool("dry-run", false, "Preview without storing")
	plans      = flag.Bool("plans", false, "Also import plan documents")
	claudeMd   = flag.Bool("claude-md", false, "Also import CLAUDE.md files")
	memoryPath = flag.String("memory-path", "", "Override memory scan path (default: ~/.claude/projects/*/memory/)")
	planPath   = flag.String("plan-path", "", "Override plan scan path (default: ~/Code/*/docs/plans/)")
	codePath   = flag.String("code-path", "", "Override code root for CLAUDE.md scan (default: ~/Code)")
	maxChars   = flag.Int("max-chars", 3800, "Max chars per section (embeddinggemma limit ~4000)")
)

// httpClient with TLS skip for non-public TLDs (.lthn.sh has real certs, but
// allow .lan/.local if someone has legacy config).
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	},
}

func main() {
	flag.Parse()

	fmt.Println("OpenBrain Seed — MCP API Client")
	fmt.Println(strings.Repeat("=", 55))

	if *apiKey == "" && !*dryRun {
		fmt.Println("ERROR: -api-key is required (or use -dry-run)")
		fmt.Println("  Generate one at: https://lthn.sh/admin/mcp/api-keys")
		os.Exit(1)
	}

	if *dryRun {
		fmt.Println("[DRY RUN] — no data will be stored")
	}

	fmt.Printf("API: %s\n", *apiURL)
	fmt.Printf("Server: %s | Agent: %s\n", *server, *agent)

	// Discover memory files
	memPath := *memoryPath
	if memPath == "" {
		home, _ := os.UserHomeDir()
		memPath = filepath.Join(home, ".claude", "projects", "*", "memory")
	}
	memFiles, _ := filepath.Glob(filepath.Join(memPath, "*.md"))
	fmt.Printf("\nFound %d memory files\n", len(memFiles))

	// Discover plan files
	var planFiles []string
	if *plans {
		pPath := *planPath
		if pPath == "" {
			home, _ := os.UserHomeDir()
			pPath = filepath.Join(home, "Code", "*", "docs", "plans")
		}
		planFiles, _ = filepath.Glob(filepath.Join(pPath, "*.md"))
		// Also check nested dirs (completed/, etc.)
		nested, _ := filepath.Glob(filepath.Join(pPath, "*", "*.md"))
		planFiles = append(planFiles, nested...)

		// Also check host-uk nested repos
		home, _ := os.UserHomeDir()
		hostUkPath := filepath.Join(home, "Code", "host-uk", "*", "docs", "plans")
		hostUkFiles, _ := filepath.Glob(filepath.Join(hostUkPath, "*.md"))
		planFiles = append(planFiles, hostUkFiles...)
		hostUkNested, _ := filepath.Glob(filepath.Join(hostUkPath, "*", "*.md"))
		planFiles = append(planFiles, hostUkNested...)

		fmt.Printf("Found %d plan files\n", len(planFiles))
	}

	// Discover CLAUDE.md files
	var claudeFiles []string
	if *claudeMd {
		cPath := *codePath
		if cPath == "" {
			home, _ := os.UserHomeDir()
			cPath = filepath.Join(home, "Code")
		}
		claudeFiles = discoverClaudeMdFiles(cPath)
		fmt.Printf("Found %d CLAUDE.md files\n", len(claudeFiles))
	}

	imported := 0
	skipped := 0
	errors := 0

	// Process memory files
	fmt.Println("\n--- Memory Files ---")
	for _, f := range memFiles {
		project := extractProject(f)
		sections := parseMarkdownSections(f)
		filename := strings.TrimSuffix(filepath.Base(f), ".md")

		if len(sections) == 0 {
			fmt.Printf("  skip %s/%s (no sections)\n", project, filename)
			skipped++
			continue
		}

		for _, sec := range sections {
			content := sec.heading + "\n\n" + sec.content
			if strings.TrimSpace(sec.content) == "" {
				skipped++
				continue
			}

			memType := inferType(sec.heading, sec.content, "memory")
			tags := buildTags(filename, "memory", project)
			confidence := confidenceForSource("memory")

			// Truncate to embedding model limit
			content = truncate(content, *maxChars)

			if *dryRun {
				fmt.Printf("  [DRY] %s/%s :: %s (%s) — %d chars\n",
					project, filename, sec.heading, memType, len(content))
				imported++
				continue
			}

			if err := callBrainRemember(content, memType, tags, project, confidence); err != nil {
				fmt.Printf("  FAIL %s/%s :: %s — %v\n", project, filename, sec.heading, err)
				errors++
				continue
			}
			fmt.Printf("  ok   %s/%s :: %s (%s)\n", project, filename, sec.heading, memType)
			imported++
		}
	}

	// Process plan files
	if *plans && len(planFiles) > 0 {
		fmt.Println("\n--- Plan Documents ---")
		for _, f := range planFiles {
			project := extractProjectFromPlan(f)
			sections := parseMarkdownSections(f)
			filename := strings.TrimSuffix(filepath.Base(f), ".md")

			if len(sections) == 0 {
				skipped++
				continue
			}

			for _, sec := range sections {
				content := sec.heading + "\n\n" + sec.content
				if strings.TrimSpace(sec.content) == "" {
					skipped++
					continue
				}

				tags := buildTags(filename, "plans", project)
				content = truncate(content, *maxChars)

				if *dryRun {
					fmt.Printf("  [DRY] %s :: %s / %s (plan) — %d chars\n",
						project, filename, sec.heading, len(content))
					imported++
					continue
				}

				if err := callBrainRemember(content, "plan", tags, project, 0.6); err != nil {
					fmt.Printf("  FAIL %s :: %s / %s — %v\n", project, filename, sec.heading, err)
					errors++
					continue
				}
				fmt.Printf("  ok   %s :: %s / %s (plan)\n", project, filename, sec.heading)
				imported++
			}
		}
	}

	// Process CLAUDE.md files
	if *claudeMd && len(claudeFiles) > 0 {
		fmt.Println("\n--- CLAUDE.md Files ---")
		for _, f := range claudeFiles {
			project := extractProjectFromClaudeMd(f)
			sections := parseMarkdownSections(f)

			if len(sections) == 0 {
				skipped++
				continue
			}

			for _, sec := range sections {
				content := sec.heading + "\n\n" + sec.content
				if strings.TrimSpace(sec.content) == "" {
					skipped++
					continue
				}

				tags := buildTags("CLAUDE", "claude-md", project)
				content = truncate(content, *maxChars)

				if *dryRun {
					fmt.Printf("  [DRY] %s :: CLAUDE.md / %s (convention) — %d chars\n",
						project, sec.heading, len(content))
					imported++
					continue
				}

				if err := callBrainRemember(content, "convention", tags, project, 0.9); err != nil {
					fmt.Printf("  FAIL %s :: CLAUDE.md / %s — %v\n", project, sec.heading, err)
					errors++
					continue
				}
				fmt.Printf("  ok   %s :: CLAUDE.md / %s (convention)\n", project, sec.heading)
				imported++
			}
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 55))
	prefix := ""
	if *dryRun {
		prefix = "[DRY RUN] "
	}
	fmt.Printf("%sImported: %d | Skipped: %d | Errors: %d\n", prefix, imported, skipped, errors)
}

// callBrainRemember sends a memory to the MCP API via brain_remember tool.
func callBrainRemember(content, memType string, tags []string, project string, confidence float64) error {
	args := map[string]any{
		"content":    content,
		"type":       memType,
		"tags":       tags,
		"confidence": confidence,
	}
	if project != "" && project != "unknown" {
		args["project"] = project
	}

	payload := map[string]any{
		"server":    *server,
		"tool":      "brain_remember",
		"arguments": args,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", *apiURL+"/tools/call", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+*apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("API: %s", result.Error)
	}

	return nil
}

// truncate caps content to maxLen chars, appending an ellipsis if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find last space before limit to avoid splitting mid-word
	cut := maxLen
	if idx := strings.LastIndex(s[:maxLen], " "); idx > maxLen-200 {
		cut = idx
	}
	return s[:cut] + "…"
}

// discoverClaudeMdFiles finds CLAUDE.md files across a code directory.
func discoverClaudeMdFiles(codePath string) []string {
	var files []string

	// Walk up to 4 levels deep, skip node_modules/vendor/.claude
	_ = filepath.WalkDir(codePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == "vendor" || name == ".claude" {
				return filepath.SkipDir
			}
			// Limit depth
			rel, _ := filepath.Rel(codePath, path)
			if strings.Count(rel, string(os.PathSeparator)) > 3 {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "CLAUDE.md" {
			files = append(files, path)
		}
		return nil
	})

	return files
}

// section is a parsed markdown section.
type section struct {
	heading string
	content string
}

var headingRe = regexp.MustCompile(`^#{1,3}\s+(.+)$`)

// parseMarkdownSections splits a markdown file by headings.
func parseMarkdownSections(path string) []section {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}

	var sections []section
	lines := strings.Split(string(data), "\n")
	var curHeading string
	var curContent []string

	for _, line := range lines {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			if curHeading != "" && len(curContent) > 0 {
				text := strings.TrimSpace(strings.Join(curContent, "\n"))
				if text != "" {
					sections = append(sections, section{curHeading, text})
				}
			}
			curHeading = strings.TrimSpace(m[1])
			curContent = nil
		} else {
			curContent = append(curContent, line)
		}
	}

	// Flush last section
	if curHeading != "" && len(curContent) > 0 {
		text := strings.TrimSpace(strings.Join(curContent, "\n"))
		if text != "" {
			sections = append(sections, section{curHeading, text})
		}
	}

	// If no headings found, treat entire file as one section
	if len(sections) == 0 && strings.TrimSpace(string(data)) != "" {
		sections = append(sections, section{
			heading: strings.TrimSuffix(filepath.Base(path), ".md"),
			content: strings.TrimSpace(string(data)),
		})
	}

	return sections
}

// extractProject derives a project name from a Claude memory path.
// ~/.claude/projects/-Users-snider-Code-eaas/memory/MEMORY.md → "eaas"
func extractProject(path string) string {
	re := regexp.MustCompile(`projects/[^/]*-([^-/]+)/memory/`)
	if m := re.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	return "unknown"
}

// extractProjectFromPlan derives a project name from a plan path.
// ~/Code/eaas/docs/plans/foo.md → "eaas"
// ~/Code/host-uk/core/docs/plans/foo.md → "core"
func extractProjectFromPlan(path string) string {
	// Check host-uk nested repos first
	re := regexp.MustCompile(`Code/host-uk/([^/]+)/docs/plans/`)
	if m := re.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	re = regexp.MustCompile(`Code/([^/]+)/docs/plans/`)
	if m := re.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	return "unknown"
}

// extractProjectFromClaudeMd derives a project name from a CLAUDE.md path.
// ~/Code/host-uk/core/CLAUDE.md → "core"
// ~/Code/eaas/CLAUDE.md → "eaas"
func extractProjectFromClaudeMd(path string) string {
	re := regexp.MustCompile(`Code/host-uk/([^/]+)/`)
	if m := re.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	re = regexp.MustCompile(`Code/([^/]+)/`)
	if m := re.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	return "unknown"
}

// inferType guesses the memory type from heading + content keywords.
func inferType(heading, content, source string) string {
	// Source-specific defaults (match PHP BrainIngestCommand behaviour)
	if source == "plans" {
		return "plan"
	}
	if source == "claude-md" {
		return "convention"
	}

	lower := strings.ToLower(heading + " " + content)
	patterns := map[string][]string{
		"architecture": {"architecture", "stack", "infrastructure", "layer", "service mesh"},
		"convention":   {"convention", "standard", "naming", "pattern", "rule", "coding"},
		"decision":     {"decision", "chose", "strategy", "approach", "domain"},
		"bug":          {"bug", "fix", "broken", "error", "issue", "lesson"},
		"plan":         {"plan", "todo", "roadmap", "milestone", "phase", "task"},
		"research":     {"research", "finding", "discovery", "analysis", "rfc"},
	}
	for t, keywords := range patterns {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return t
			}
		}
	}
	return "observation"
}

// buildTags creates the tag list for a memory.
func buildTags(filename, source, project string) []string {
	tags := []string{"source:" + source}
	if project != "" && project != "unknown" {
		tags = append(tags, "project:"+project)
	}
	if filename != "MEMORY" && filename != "CLAUDE" {
		tags = append(tags, strings.ReplaceAll(strings.ReplaceAll(filename, "-", " "), "_", " "))
	}
	return tags
}

// confidenceForSource returns a default confidence level matching the PHP ingest command.
func confidenceForSource(source string) float64 {
	switch source {
	case "claude-md":
		return 0.9
	case "memory":
		return 0.8
	case "plans":
		return 0.6
	default:
		return 0.5
	}
}
