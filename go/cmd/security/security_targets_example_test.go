package security

import core "dappco.re/go"

func ExampleSecurityTarget() {
	result := parseSecurityTarget("acme/api")
	target := result.Value.(SecurityTarget)

	core.Println(result.OK)
	core.Println(target.DisplayName)
	// Output:
	// true
	// api
}
