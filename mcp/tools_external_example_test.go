package mcp

import core "dappco.re/go"

type Buffer = safeBuffer

func ExampleBuffer_Write() {
	var buffer Buffer
	n, err := buffer.Write([]byte("agent"))

	core.Println(err == nil)
	core.Println(n)
	// Output:
	// true
	// 5
}

func ExampleBuffer_String() {
	var buffer Buffer
	buffer.Write([]byte("agent"))

	core.Println(buffer.String())
	// Output:
	// agent
}
