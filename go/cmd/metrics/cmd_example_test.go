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

func ExampleRegisterMetricsCommand() {
	root := core.New()
	r := RegisterMetricsCommand(root, "ai/metrics")
	cmd := root.Command("ai/metrics")

	core.Println(r.OK)
	core.Println(cmd.Value.(*core.Command).Path)
	// Output:
	// true
	// ai/metrics
}
