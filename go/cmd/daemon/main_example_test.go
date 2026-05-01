package main

import core "dappco.re/go"

func ExampleConfig() {
	cfg := Config{MCPTransport: "all"}
	transports := configuredTransports(cfg)

	core.Println(core.Join(",", transports...))
	// Output:
	// tcp,socket
}
