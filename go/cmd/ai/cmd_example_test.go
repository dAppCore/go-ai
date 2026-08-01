package ai

import (
	core "dappco.re/go"
)

func ExampleAddAICommands() {
	root := core.New()
	r := AddAICommands(root)
	cmd := root.Command("ai/metrics")

	core.Println(r.OK && cmd.OK)
	core.Println(cmd.Value.(*core.Command).Name)
	// Output:
	// true
	// metrics
}
