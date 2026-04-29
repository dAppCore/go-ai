package mcp

import (
	core "dappco.re/go"
)

// --- AX-7 canonical triplets ---

func TestJsonrpc_Service_HandleFrame_Good(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	response, err := service.HandleFrame(core.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))

	core.AssertNoError(t, err)
	core.AssertContains(t, string(response), `"result"`)
}

func TestJsonrpc_Service_HandleFrame_Bad(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	response, err := service.HandleFrame(core.Background(), []byte(`{bad json`))

	core.AssertError(t, err)
	core.AssertContains(t, string(response), "parse error")
}

func TestJsonrpc_Service_HandleFrame_Ugly(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	response, err := service.HandleFrame(core.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))

	core.AssertNoError(t, err)
	core.AssertNil(t, response)
}
