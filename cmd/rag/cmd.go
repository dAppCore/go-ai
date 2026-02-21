// Package rag re-exports go-rag's CLI commands for use in the core CLI.
package rag

import ragcmd "forge.lthn.ai/core/go-rag/cmd/rag"

// AddRAGSubcommands registers RAG commands as subcommands of parent.
var AddRAGSubcommands = ragcmd.AddRAGSubcommands
