package metrics

import (
	"time"

	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

type DurationFlagValue = sinceDurationFlagValue

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

func ExampleDurationFlagValue_String() {
	value := 2 * time.Hour
	flag := &sinceDurationFlagValue{target: &value}

	core.Println(flag.String())
	// Output:
	// 2h0m0s
}

func ExampleDurationFlagValue_Set() {
	value := time.Duration(0)
	flag := &sinceDurationFlagValue{target: &value}
	err := flag.Set("2d")

	core.Println(err == nil)
	core.Println(value == 48*time.Hour)
	// Output:
	// true
	// true
}

func ExampleDurationFlagValue_Type() {
	value := time.Minute
	flag := &sinceDurationFlagValue{target: &value}

	core.Println(flag.Type())
	// Output:
	// duration
}
