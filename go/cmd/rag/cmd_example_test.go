package rag

import (
	core "dappco.re/go"
)

func ExampleAddRAGSubcommands() {
	root := core.New()
	AddRAGSubcommands(root)

	core.Println(len(root.Commands()) > 0)
	// Output:
	// true
}
