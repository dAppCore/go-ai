// Package rag reserves go-rag's command surface for use in the core CLI.
package rag

import "dappco.re/go"

// AddRAGSubcommands(parent) re-exports the go-rag CLI under `core ai rag`.
//
//	core ai rag query --question "What changed?"
func AddRAGSubcommands(c *core.Core, path ...string) core.Result {
	commandPath := "rag"
	if len(path) > 0 && path[0] != "" {
		commandPath = path[0]
	}
	if c.Command(commandPath).OK {
		return core.Ok(nil)
	}
	return c.Command(commandPath, core.Command{
		Description: "Retrieval and context assembly commands backed by go-rag.",
	})
}
