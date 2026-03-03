// SPDX-License-Identifier: EUPL-1.2

// embed-bench compares embedding models for OpenBrain by testing how well
// they separate semantically related vs unrelated agent memory pairs.
//
// Usage:
//
//	go run ./cmd/embed-bench
//	go run ./cmd/embed-bench -ollama http://localhost:11434
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

var ollamaURL = flag.String("ollama", "http://localhost:11434", "Ollama base URL")

// models to benchmark
var models = []string{
	"nomic-embed-text",
	"embeddinggemma",
}

// Test corpus: real-ish agent memories grouped by topic.
// Memories within a group should be similar; across groups should be distant.
var memoryGroups = []struct {
	topic    string
	memories []string
}{
	{
		topic: "scoring-calibration",
		memories: []string{
			"LEM emotional_register was blind to negative emotions. Fixed by adding 8 weighted pattern groups covering sadness, anger, fear, disgust, and frustration.",
			"The EaaS scoring service had a verdict cross-check bug where the LEK composite and individual heuristic scores could disagree on the final verdict classification.",
			"Scoring calibration: emotional_register vocabulary expanded from 12 positive-only patterns to 20 patterns covering both positive and negative emotional markers.",
		},
	},
	{
		topic: "openbrain-architecture",
		memories: []string{
			"OpenBrain uses MariaDB for relational metadata and Qdrant for vector embeddings. Four MCP tools in php-agentic. Go bridge in go-ai for CLI agents.",
			"Brain memories have a supersession chain — newer memories can supersede older ones, creating version history. The getSupersessionDepth method walks the chain capped at 50.",
			"The brain_recall tool embeds the query via Ollama, searches Qdrant with workspace-scoped filters, then hydrates results from MariaDB with active() and latestVersions() scopes.",
		},
	},
	{
		topic: "deployment-infrastructure",
		memories: []string{
			"Production fleet: noc (Helsinki HCloud) + de1 (Falkenstein HRobot). Port 22 runs Endlessh. Real SSH on 4819. All operations through Ansible.",
			"Traefik handles reverse proxy on de1. Services exposed on ports 8000-8090. Dragonfly on 6379, Galera on 3306, PG on 5432.",
			"Forgejo runner on noc with DinD isolation. Labels: ubuntu-latest + docker. CI deploys to BunnyCDN on push to main.",
		},
	},
	{
		topic: "lem-training",
		memories: []string{
			"LEM training uses sandwich format: system prompt wraps around user/assistant turns. Curriculum has 5 phases from foundation to specialisation.",
			"MLX-LM fine-tuning on Apple Silicon. LoRA adapters for efficient training. Qwen3-8B as base model for chat inference in LEM Lab.",
			"LEM Lab is a native Mac app using Core Go framework with Wails v3. Chat UI is vanilla Web Components, 22KB, zero dependencies.",
		},
	},
}

// Queries to test recall quality — each has a target topic it should match best.
var queries = []struct {
	query       string
	targetTopic string
}{
	{"How does the emotional scoring work?", "scoring-calibration"},
	{"What database does the brain use?", "openbrain-architecture"},
	{"How do I deploy to production?", "deployment-infrastructure"},
	{"How is LEM trained?", "lem-training"},
	{"What is the supersession chain?", "openbrain-architecture"},
	{"Where is the Forgejo runner?", "deployment-infrastructure"},
	{"What patterns detect sycophancy?", "scoring-calibration"},
	{"What framework does the chat UI use?", "lem-training"},
}

func main() {
	flag.Parse()

	fmt.Println("OpenBrain Embedding Model Benchmark")
	fmt.Println(strings.Repeat("=", 60))

	for _, model := range models {
		fmt.Printf("\n## Model: %s\n", model)
		fmt.Println(strings.Repeat("-", 40))

		// Check model is available
		if !modelAvailable(model) {
			fmt.Printf("  SKIPPED — model not pulled (run: ollama pull %s)\n", model)
			continue
		}

		// 1. Embed all memories
		allMemories := []string{}
		allTopics := []string{}
		for _, group := range memoryGroups {
			for _, mem := range group.memories {
				allMemories = append(allMemories, mem)
				allTopics = append(allTopics, group.topic)
			}
		}

		fmt.Printf("  Embedding %d memories...\n", len(allMemories))
		start := time.Now()
		memVectors := make([][]float64, len(allMemories))
		for i, mem := range allMemories {
			vec, err := embed(model, mem)
			if err != nil {
				fmt.Printf("  ERROR embedding memory %d: %v\n", i, err)
				break
			}
			memVectors[i] = vec
		}
		embedTime := time.Since(start)
		fmt.Printf("  Embedded in %v (%.0fms/memory)\n", embedTime, float64(embedTime.Milliseconds())/float64(len(allMemories)))
		fmt.Printf("  Vector dimension: %d\n", len(memVectors[0]))

		// 2. Intra-group vs inter-group similarity
		var intraSims, interSims []float64
		for i := 0; i < len(allMemories); i++ {
			for j := i + 1; j < len(allMemories); j++ {
				sim := cosine(memVectors[i], memVectors[j])
				if allTopics[i] == allTopics[j] {
					intraSims = append(intraSims, sim)
				} else {
					interSims = append(interSims, sim)
				}
			}
		}

		intraAvg := avg(intraSims)
		interAvg := avg(interSims)
		separation := intraAvg - interAvg

		fmt.Printf("\n  Cluster separation:\n")
		fmt.Printf("    Intra-group similarity (same topic):  %.4f\n", intraAvg)
		fmt.Printf("    Inter-group similarity (diff topic):  %.4f\n", interAvg)
		fmt.Printf("    Separation gap:                       %.4f  %s\n", separation, qualityLabel(separation))

		// 3. Query recall accuracy
		fmt.Printf("\n  Query recall (top-1 accuracy):\n")
		correct := 0
		for _, q := range queries {
			qVec, err := embed(model, q.query)
			if err != nil {
				fmt.Printf("    ERROR: %v\n", err)
				continue
			}

			// Find best match
			bestIdx := 0
			bestSim := -1.0
			for i, mv := range memVectors {
				sim := cosine(qVec, mv)
				if sim > bestSim {
					bestSim = sim
					bestIdx = i
				}
			}

			matchTopic := allTopics[bestIdx]
			hit := matchTopic == q.targetTopic
			if hit {
				correct++
			}
			marker := "✓"
			if !hit {
				marker = "✗"
			}
			fmt.Printf("    %s %.4f  %q → %s (want: %s)\n", marker, bestSim, truncate(q.query, 40), matchTopic, q.targetTopic)
		}

		accuracy := float64(correct) / float64(len(queries)) * 100
		fmt.Printf("\n  Top-1 accuracy: %.0f%% (%d/%d)\n", accuracy, correct, len(queries))

		// 4. Top-3 recall
		correct3 := 0
		for _, q := range queries {
			qVec, _ := embed(model, q.query)

			type scored struct {
				idx int
				sim float64
			}
			var ranked []scored
			for i, mv := range memVectors {
				ranked = append(ranked, scored{i, cosine(qVec, mv)})
			}
			sort.Slice(ranked, func(a, b int) bool { return ranked[a].sim > ranked[b].sim })

			for _, r := range ranked[:3] {
				if allTopics[r.idx] == q.targetTopic {
					correct3++
					break
				}
			}
		}
		accuracy3 := float64(correct3) / float64(len(queries)) * 100
		fmt.Printf("  Top-3 accuracy: %.0f%% (%d/%d)\n", accuracy3, correct3, len(queries))
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Done.")
}

// -- Ollama helpers --

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float64 `json:"embedding"`
}

func embed(model, text string) ([]float64, error) {
	body, _ := json.Marshal(embedRequest{Model: model, Prompt: text})
	resp, err := http.Post(*ollamaURL+"/api/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	return result.Embedding, nil
}

func modelAvailable(model string) bool {
	resp, err := http.Get(*ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	for _, m := range result.Models {
		// Match "nomic-embed-text:latest" against "nomic-embed-text"
		if m.Name == model || strings.HasPrefix(m.Name, model+":") {
			return true
		}
	}
	return false
}

// -- Math helpers --

func cosine(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func qualityLabel(gap float64) string {
	switch {
	case gap > 0.15:
		return "(excellent)"
	case gap > 0.10:
		return "(good)"
	case gap > 0.05:
		return "(fair)"
	default:
		return "(poor)"
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func init() {
	// Ensure stderr doesn't buffer
	os.Stderr.Sync()
}
