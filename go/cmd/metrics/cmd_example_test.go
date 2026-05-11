package metrics

import (
	core "dappco.re/go"
)

func ExampleAddMetricsCommand() {
	root := core.New()
	r := AddMetricsCommand(root)
	cmd := root.Command("metrics")

	core.Println(r.OK && cmd.OK)
	core.Println(cmd.Value.(*core.Command).Name)
	// Output:
	// true
	// metrics
}
