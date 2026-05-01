package rag

import (
	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func ExampleAddRAGSubcommands() {
	root := &cli.Command{Use: "ai"}
	AddRAGSubcommands(root)

	core.Println(len(root.Commands()) > 0)
	// Output:
	// true
}
