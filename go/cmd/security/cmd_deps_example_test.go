package security

import core "dappco.re/go"

func ExampleDepAlert() {
	alert := DepAlert{Repo: "api", Severity: "high", CVE: "CVE-2026-0001"}

	core.Println(alert.Repo)
	core.Println(alert.Severity)
	// Output:
	// api
	// high
}
