package security

import core "dappco.re/go"

func ExampleAlertOutput() {
	alert := AlertOutput{Repo: "api", Type: "code-scanning", Location: "main.go:14"}

	core.Println(alert.Type)
	core.Println(alert.Location)
	// Output:
	// code-scanning
	// main.go:14
}
