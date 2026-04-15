// Package security wires the GitHub security command group into the core CLI.
//
//	core security alerts --repo core/go-ai
//	core security jobs --targets all --copies 4
package security

import "forge.lthn.ai/core/cli/pkg/cli"

func init() {
	cli.RegisterCommands(AddSecurityCommands)
}
