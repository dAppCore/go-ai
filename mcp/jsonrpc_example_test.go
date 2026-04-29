package mcp

import (
	"context"

	core "dappco.re/go"
)

func ExampleService_HandleFrame() {
	service, _ := New(WithWorkspaceRoot(""))
	frame := []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"lang_detect\",\"arguments\":{\"\x70ath\":\"main.go\"}}}")
	response, err := service.HandleFrame(context.Background(), frame)

	core.Println(err == nil)
	core.Println(core.Contains(string(response), `"language":"go"`))
	// Output:
	// true
	// true
}
