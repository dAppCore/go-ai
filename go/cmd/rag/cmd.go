// Package rag re-exports go-rag's CLI commands for use in the core CLI.
package rag

import ragcmd "dappco.re/go/rag/cmd/rag"

// AddRAGSubcommands(parent) re-exports the go-rag CLI under `core ai rag`.
//
//	core ai rag query --question "What changed?"
var AddRAGSubcommands = ragcmd.AddRAGSubcommands
