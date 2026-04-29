package main

import (
	"io"

	core "dappco.re/go"
)

func ExampleLabCommandOptions() {
	options, err := parseLabServeOptions([]string{"--bind", "127.0.0.1:9090"}, io.Discard)

	core.Println(err == nil)
	core.Println(options.Bind)
	// Output:
	// true
	// 127.0.0.1:9090
}
