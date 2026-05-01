package main

import (
	"io"

	core "dappco.re/go"
)

func ExampleLabCommandOptions() {
	result := parseLabServeOptions([]string{"--bind", "127.0.0.1:9090"}, io.Discard)
	options := result.Value.(LabCommandOptions)

	core.Println(result.OK)
	core.Println(options.Bind)
	// Output:
	// true
	// 127.0.0.1:9090
}
