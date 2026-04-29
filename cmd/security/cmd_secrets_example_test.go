package security

import core "dappco.re/go"

func ExampleSecretAlert() {
	alert := SecretAlert{Repo: "api", Number: 9, SecretType: "token"}

	core.Println(alert.Repo)
	core.Println(alert.SecretType)
	// Output:
	// api
	// token
}
