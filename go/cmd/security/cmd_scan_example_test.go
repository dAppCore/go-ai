package security

import core "dappco.re/go"

func ExampleScanAlert() {
	alert := ScanAlert{Repo: "api", RuleID: "gosec/G401", Path: "main.go", Line: 14}

	core.Println(alert.RuleID)
	core.Println(alert.Path)
	// Output:
	// gosec/G401
	// main.go
}
