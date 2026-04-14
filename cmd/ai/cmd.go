// Package ai provides the unified AI command surface for the core CLI.
package ai

import (
	metricscmd "dappco.re/go/core/ai/cmd/metrics"
	ragcmd "dappco.re/go/core/ai/cmd/rag"
	mlcmd "dappco.re/go/core/ml/cmd"
	"forge.lthn.ai/core/cli/pkg/cli"
)

func init() {
	cli.RegisterCommands(AddAICommands)
}

// AddAICommands registers the 'ai' command and delegated subcommands.
func AddAICommands(root *cli.Command) {
	if hasCommand(root, "ai") {
		return
	}

	aiCmd := cli.NewGroup(
		"ai",
		"AI facade and delegated tooling",
		"Unified AI commands for metrics, RAG, and ML workflows.",
	)

	metricscmd.AddMetricsCommand(aiCmd)
	ragcmd.AddRAGSubcommands(aiCmd)
	mlcmd.AddMLCommands(aiCmd)

	root.AddCommand(aiCmd)
}

func hasCommand(parent *cli.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}
