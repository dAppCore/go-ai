// Package ai provides the unified AI command surface for the core CLI.
package ai

import (
	metricscmd "dappco.re/go/ai/cmd/metrics"
	ragcmd "dappco.re/go/ai/cmd/rag"
	"dappco.re/go/cli/pkg/cli"
)

func init() {
	cli.RegisterCommands(AddAICommands)
}

// core ai metrics --since 24h
// core ai rag query --question "What changed?"
func AddAICommands(root *cli.Command) {
	if commandExists(root, "ai") {
		return
	}

	aiCmd := cli.NewGroup(
		"ai",
		"AI facade and delegated tooling",
		"Unified AI commands for metrics and RAG workflows.",
	)

	metricscmd.AddMetricsCommand(aiCmd)
	ragcmd.AddRAGSubcommands(aiCmd)

	root.AddCommand(aiCmd)
}

func commandExists(parent *cli.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}
