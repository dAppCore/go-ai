// SPDX-Licence-Identifier: EUPL-1.2

package lab

import (
	core "dappco.re/go"
)

// --- AX-7 canonical triplets ---

func TestService_NewService_Good(t *core.T) {
	svc := NewService(Options{})

	core.AssertTrue(t, svc.provider != nil)
	core.AssertTrue(t, svc.driver != nil)
	core.AssertTrue(t, svc.library != nil)
	core.AssertTrue(t, svc.saved != nil)
	core.AssertEqual(t, "Lab", svc.ServiceName())
}

func TestService_NewService_Bad(t *core.T) {
	// Passing a real provider but leaving driver / library / saved
	// nil — defaults still fill in, the explicit provider stays.
	custom := &MockInferenceProvider{}
	svc := NewService(Options{Provider: custom})

	core.AssertTrue(t, svc.provider == custom)
	core.AssertTrue(t, svc.driver != nil)
	core.AssertTrue(t, svc.library != nil)
	core.AssertTrue(t, svc.saved != nil)
}

func TestService_NewService_Ugly(t *core.T) {
	// All fields supplied — none get replaced.
	prov := NewMockProvider()
	drv := NewMockDriver()
	lib := NewMockLibrary()
	store := NewMemorySavedQueryStore()
	svc := NewService(Options{Provider: prov, Driver: drv, Library: lib, Saved: store})

	core.AssertTrue(t, svc.provider == prov)
	core.AssertTrue(t, svc.driver == drv)
	core.AssertTrue(t, svc.library == lib)
	core.AssertTrue(t, svc.saved == store)
}

func TestService_ListModels_Good(t *core.T) {
	svc := NewService(Options{})
	r := svc.ListModels()

	core.AssertTrue(t, r.OK)
	resp, ok := r.Value.(ModelsQueryResponse)
	core.AssertTrue(t, ok)
	core.AssertTrue(t, len(resp.Models) > 0)
	core.AssertEqual(t, "lemma-12b-v4-p6.model", resp.Models[0].Filename)
}

func TestService_ListModels_Bad(t *core.T) {
	// Library that returns an error — surface should fail cleanly.
	svc := NewService(Options{Library: errLibrary{}})
	r := svc.ListModels()

	core.AssertFalse(t, r.OK)
	core.AssertContains(t, r.Error(), "library list")
}

func TestService_ListModels_Ugly(t *core.T) {
	// Empty library — Ok with empty list, not an error.
	svc := NewService(Options{Library: emptyLibrary{}})
	r := svc.ListModels()

	core.AssertTrue(t, r.OK)
	resp := r.Value.(ModelsQueryResponse)
	core.AssertEqual(t, 0, len(resp.Models))
}

func TestService_ListRuns_Good(t *core.T) {
	svc := NewService(Options{})
	r := svc.ListRuns()

	core.AssertTrue(t, r.OK)
	resp, ok := r.Value.(RunsQueryResponse)
	core.AssertTrue(t, ok)
	// Driver seeds three history rows even with no active run.
	core.AssertTrue(t, len(resp.Runs) >= 3)
}

func TestService_StartRun_Good(t *core.T) {
	svc := NewService(Options{})
	r := svc.StartRun(StartRunRequest{
		BaseModel:  "lemma-12b-v4-p6",
		Mode:       LoRAModeSFT,
		TotalIters: 4,
		Hyperparams: map[string]any{"tick_ms": 10},
	})

	core.AssertTrue(t, r.OK)
	h := r.Value.(RunHandle)
	core.AssertTrue(t, h.RunID != "")
}

// --- helpers ---

type errLibrary struct{}

func (errLibrary) ListModels() ([]ModelRow, error) {
	return nil, core.E("test.errLibrary", "synthetic failure", nil)
}

type emptyLibrary struct{}

func (emptyLibrary) ListModels() ([]ModelRow, error) { return nil, nil }
