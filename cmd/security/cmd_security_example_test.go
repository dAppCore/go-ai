package security

import (
	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

type RawMessage = githubRawMessage

func ExampleAddSecurityCommands() {
	root := &cli.Command{Use: "core"}
	AddSecurityCommands(root)
	cmd, _, err := root.Find([]string{"security"})

	core.Println(err == nil)
	core.Println(cmd.Name())
	// Output:
	// true
	// security
}

func ExampleRawMessage_UnmarshalJSON() {
	var raw githubRawMessage
	err := raw.UnmarshalJSON([]byte(`{"ok":true}`))

	core.Println(err == nil)
	core.Println(string(raw))
	// Output:
	// true
	// {"ok":true}
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
