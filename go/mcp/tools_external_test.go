package mcp

import (
	core "dappco.re/go"
)

// --- AX-7 canonical triplets ---

func TestToolsExternal_Buffer_String_Good(t *core.T) {
	var buffer safeBuffer
	buffer.append([]byte("agent"))
	got := buffer.String()

	core.AssertEqual(t, "agent", got)
}

func TestToolsExternal_Buffer_String_Bad(t *core.T) {
	var buffer safeBuffer
	got := buffer.String()
	want := ""

	core.AssertEqual(t, want, got)
	core.AssertEmpty(t, got)
}

func TestToolsExternal_Buffer_String_Ugly(t *core.T) {
	var buffer safeBuffer
	buffer.append([]byte("agent"))
	first := buffer.String()

	core.AssertEqual(t, first, buffer.String())
}
