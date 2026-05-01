package ai

import (
	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func ExampleAddAICommands() {
	root := &cli.Command{Use: "core"}
	AddAICommands(root)
	cmd, _, err := root.Find([]string{"ai", "metrics"})

	core.Println(err == nil)
	core.Println(cmd.Name())
	// Output:
	// true
	// metrics
}
