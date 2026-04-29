package security

import core "dappco.re/go"

func ExampleSecurityTarget() {
	target, err := parseSecurityTarget("acme/api")

	core.Println(err == nil)
	core.Println(target.DisplayName)
	// Output:
	// true
	// api
}
