// Package ai provides the unified AI command surface for the core CLI.
package ai

import (
	"dappco.re/go"
	metricscmd "dappco.re/go/ai/cmd/metrics"
	ragcmd "dappco.re/go/ai/cmd/rag"
	_ "dappco.re/go/ai/cmd/security" // registers core security commands when cmd/ai is imported.
	_ "dappco.re/go/ai/pkg/lab"      // registers core lab commands when cmd/ai is imported.
	"dappco.re/go/cli/pkg/cli"
)

func init() {
	cli.RegisterCommands(AddAICommands)
}

// core ai metrics --since 24h
// core ai rag query --question "What changed?"
func AddAICommands(c *core.Core) core.Result {
	if r := registerAICommand(c, "ai", core.Command{Description: "Unified AI commands for metrics and RAG workflows."}); !r.OK {
		return r
	}
	if r := metricscmd.RegisterMetricsCommand(c, "ai/metrics"); !r.OK {
		return r
	}
	return ragcmd.AddRAGSubcommands(c, "ai/rag")
}

func registerAICommand(c *core.Core, path string, command core.Command) core.Result {
	if c.Command(path).OK {
		return core.Ok(nil)
	}
	return c.Command(path, command)
}
