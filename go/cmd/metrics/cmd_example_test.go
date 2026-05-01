package metrics

import (
	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func ExampleAddMetricsCommand() {
	root := &cli.Command{Use: "core"}
	AddMetricsCommand(root)
	cmd, _, err := root.Find([]string{"metrics"})

	core.Println(err == nil)
	core.Println(cmd.Name())
	// Output:
	// true
	// metrics
}
