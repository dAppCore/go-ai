package mcp

import (
	"context"

	core "dappco.re/go"
	"dappco.re/go/inference"
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

type capabilityBackend struct {
	name string
}

func (backend capabilityBackend) Name() string { return backend.name }

func (backend capabilityBackend) Available() bool { return true }

func (backend capabilityBackend) LoadModel(string, ...inference.LoadOption) (inference.TextModel, error) {
	return nil, core.AnError
}

func (backend capabilityBackend) Capabilities() inference.CapabilityReport {
	return inference.CapabilityReport{
		Runtime:   inference.RuntimeIdentity{Backend: backend.name, NativeRuntime: true},
		Available: true,
		Capabilities: []inference.Capability{
			inference.SupportedCapability(inference.CapabilityGenerate, inference.CapabilityGroupModel),
			inference.SupportedCapability(inference.CapabilityProbeEvents, inference.CapabilityGroupProbe),
		},
	}
}

func TestToolsExternal_MLBackendsUsesInferenceCapabilities_Good(t *core.T) {
	name := "ai-capability-test-" + t.Name()
	inference.Register(capabilityBackend{name: name})

	result := (&Service{}).mlBackends(context.Background(), MLBackendsInput{})
	output := result.Value.(MLBackendsOutput)

	var found *MLBackendInfo
	for i := range output.Backends {
		if output.Backends[i].Name == name {
			found = &output.Backends[i]
			break
		}
	}

	core.AssertNotNil(t, found)
	core.AssertTrue(t, found.Available)
	core.AssertTrue(t, found.Native)
	core.AssertContains(t, found.Capabilities, string(inference.CapabilityGenerate))
	core.AssertContains(t, found.Capabilities, string(inference.CapabilityProbeEvents))
}
