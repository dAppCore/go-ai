package mcp

import (
	core "dappco.re/go"
)

// --- AX-7 canonical triplets ---

func TestToolsExternal_Buffer_Write_Good(t *core.T) {
	var buffer safeBuffer
	n, err := buffer.Write([]byte("agent"))
	got := buffer.String()

	core.AssertNoError(t, err)
	core.AssertEqual(t, 5, n)
	core.AssertEqual(t, "agent", got)
}

func TestToolsExternal_Buffer_Write_Bad(t *core.T) {
	var buffer safeBuffer
	n, err := buffer.Write(nil)
	got := buffer.String()

	core.AssertNoError(t, err)
	core.AssertEqual(t, 0, n)
	core.AssertEqual(t, "", got)
}

func TestToolsExternal_Buffer_Write_Ugly(t *core.T) {
	var buffer safeBuffer
	first, firstErr := buffer.Write([]byte("agent"))
	second, secondErr := buffer.Write([]byte("-ready"))

	core.AssertNoError(t, firstErr)
	core.AssertNoError(t, secondErr)
	core.AssertEqual(t, 11, first+second)
}

func TestToolsExternal_Buffer_String_Good(t *core.T) {
	var buffer safeBuffer
	_, err := buffer.Write([]byte("agent"))
	got := buffer.String()

	core.AssertNoError(t, err)
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
	_, err := buffer.Write([]byte("agent"))
	first := buffer.String()

	core.AssertNoError(t, err)
	core.AssertEqual(t, first, buffer.String())
}
