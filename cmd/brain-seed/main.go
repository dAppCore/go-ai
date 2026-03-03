// SPDX-License-Identifier: EUPL-1.2

// brain-seed imports Claude Code MEMORY.md files into the OpenBrain knowledge
// store by embedding them via Ollama and storing vectors in Qdrant.
//
// Usage:
//
//	go run ./cmd/brain-seed
//	go run ./cmd/brain-seed -ollama https://ollama.lan -qdrant https://qdrant.lan
//	go run ./cmd/brain-seed -dry-run
//	go run ./cmd/brain-seed -plans  # Also import plan docs
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

	"github.com/google/uuid"
)

var (
	ollamaURL  = flag.String("ollama", "https://ollama.lan", "Ollama base URL")
	qdrantURL  = flag.String("qdrant", "https://qdrant.lan", "Qdrant base URL")
	collection = flag.String("collection", "openbrain", "Qdrant collection name")
	model      = flag.String("model", "embeddinggemma", "Embedding model")
	workspace  = flag.Int("workspace", 1, "Workspace ID")
	agent      = flag.String("agent", "virgil", "Agent ID")
	dryRun     = flag.Bool("dry-run", false, "Preview without storing")
	plans      = flag.Bool("plans", false, "Also import plan documents")
	memoryPath = flag.String("memory-path", "", "Override memory scan path (default: ~/.claude/projects/*/memory/)")
	planPath   = flag.String("plan-path", "", "Override plan scan path (default: ~/Code/*/docs/plans/)")
)

// httpClient trusts self-signed certs for .lan domains behind Traefik.
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // .lan only
	},
}

func main() {
	flag.Parse()

	fmt.Println("OpenBrain Seed — Claude Code Memory Import")
	fmt.Println(strings.Repeat("=", 55))

	if *dryRun {
		fmt.Println("[DRY RUN] — no data will be stored")
	}

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
		fmt.Printf("Found %d plan files\n", len(planFiles))
	}

	// Ensure collection exists
	if !*dryRun {
		if err := ensureCollection(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
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
			memType := inferType(sec.heading, sec.content)
			content := sec.heading + "\n\n" + sec.content
			tags := buildTags(filename, "memory-import")

			if *dryRun {
				fmt.Printf("  [DRY] %s/%s :: %s (%s) — %d chars\n",
					project, filename, sec.heading, memType, len(content))
				imported++
				continue
			}

			if err := storeMemory(content, project, memType, tags); err != nil {
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

			// Plans: take the whole doc as one memory (they're already cohesive)
			// But cap at ~4000 chars to stay within embedding context
			fullContent := ""
			for _, sec := range sections {
				fullContent += sec.heading + "\n\n" + sec.content + "\n\n"
			}
			if len(fullContent) > 4000 {
				fullContent = fullContent[:4000]
			}

			tags := buildTags(filename, "plan-import")

			if *dryRun {
				fmt.Printf("  [DRY] %s :: %s (plan) — %d chars\n", project, filename, len(fullContent))
				imported++
				continue
			}

			if err := storeMemory(fullContent, project, "plan", tags); err != nil {
				fmt.Printf("  FAIL %s :: %s — %v\n", project, filename, err)
				errors++
				continue
			}
			fmt.Printf("  ok   %s :: %s (plan)\n", project, filename)
			imported++
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 55))
	prefix := ""
	if *dryRun {
		prefix = "[DRY RUN] "
	}
	fmt.Printf("%sImported: %d | Skipped: %d | Errors: %d\n", prefix, imported, skipped, errors)
}

// storeMemory embeds content and upserts into Qdrant.
func storeMemory(content, project, memType string, tags []string) error {
	vec, err := embed(content)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}

	id := uuid.New().String()
	payload := map[string]any{
		"workspace_id": *workspace,
		"agent_id":     *agent,
		"type":         memType,
		"tags":         tags,
		"project":      project,
		"confidence":   0.7,
		"created_at":   time.Now().UTC().Format(time.RFC3339),
	}

	return qdrantUpsert(id, vec, payload)
}

// embed generates a vector via Ollama.
func embed(text string) ([]float64, error) {
	body, _ := json.Marshal(map[string]string{
		"model":  *model,
		"prompt": text,
	})

	resp, err := httpClient.Post(*ollamaURL+"/api/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	return result.Embedding, nil
}

// qdrantUpsert stores a point in Qdrant.
func qdrantUpsert(id string, vector []float64, payload map[string]any) error {
	body, _ := json.Marshal(map[string]any{
		"points": []map[string]any{
			{"id": id, "vector": vector, "payload": payload},
		},
	})

	req, _ := http.NewRequest("PUT",
		fmt.Sprintf("%s/collections/%s/points", *qdrantURL, *collection),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ensureCollection creates the Qdrant collection if it doesn't exist.
func ensureCollection() error {
	resp, err := httpClient.Get(fmt.Sprintf("%s/collections/%s", *qdrantURL, *collection))
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == 404 {
		body, _ := json.Marshal(map[string]any{
			"vectors": map[string]any{
				"size":     768,
				"distance": "Cosine",
			},
		})
		req, _ := http.NewRequest("PUT",
			fmt.Sprintf("%s/collections/%s", *qdrantURL, *collection),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("create collection: HTTP %d: %s", resp.StatusCode, string(b))
		}
		fmt.Println("Created Qdrant collection:", *collection)
	}
	return nil
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

	// Flush last
	if curHeading != "" && len(curContent) > 0 {
		text := strings.TrimSpace(strings.Join(curContent, "\n"))
		if text != "" {
			sections = append(sections, section{curHeading, text})
		}
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
func extractProjectFromPlan(path string) string {
	re := regexp.MustCompile(`Code/([^/]+)/docs/plans/`)
	if m := re.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	return "unknown"
}

// inferType guesses the memory type from heading + content keywords.
func inferType(heading, content string) string {
	lower := strings.ToLower(heading + " " + content)
	patterns := map[string][]string{
		"architecture": {"architecture", "stack", "infrastructure", "layer", "service mesh"},
		"convention":   {"convention", "standard", "naming", "pattern", "rule", "coding"},
		"decision":     {"decision", "chose", "strategy", "approach", "domain"},
		"bug":          {"bug", "fix", "broken", "error", "issue", "lesson"},
		"plan":         {"plan", "todo", "roadmap", "milestone", "phase"},
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
func buildTags(filename string, source string) []string {
	tags := []string{source}
	if filename != "MEMORY" {
		tags = append(tags, strings.ReplaceAll(strings.ReplaceAll(filename, "-", " "), "_", " "))
	}
	return tags
}
