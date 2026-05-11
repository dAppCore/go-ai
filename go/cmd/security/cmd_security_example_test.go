package security

import (
	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func ExampleAddSecurityCommands() {
	root := core.New()
	r := AddSecurityCommands(root)
	cmd := root.Command("security")

	core.Println(r.OK && cmd.OK)
	core.Println(cmd.Value.(*core.Command).Name)
	// Output:
	// true
	// security
}

func ExampleAlertSummary_Add() {
	var summary AlertSummary
	summary.Add("critical")
	summary.Add("high")

	core.Println(summary.Total)
	core.Println(summary.Critical)
	// Output:
	// 2
	// 1
}

func ExampleAlertSummary_String() {
	wasColor := cli.ColorEnabled()
	cli.SetColorEnabled(false)
	defer cli.SetColorEnabled(wasColor)
	summary := &AlertSummary{}

	core.Println(summary.String())
	// Output:
	// No alerts
}

func ExampleAlertSummary_PlainString() {
	summary := &AlertSummary{}
	summary.Add("low")

	core.Println(summary.PlainString())
	// Output:
	// 1 low
}
