// SPDX-Licence-Identifier: EUPL-1.2

// Package lab is the ML Lab Workbench backend — the canonical Wails
// service the frontend ml-lab/* Lit views talk to over Wails bindings
// (in-process) AND over HTTP (/v1/ml-lab/*). Per
// plans/project/lthn/desktop/RFC.ml-lab.md §3.
//
// Architecture in one paragraph:
//
//   - LabService is a thin coordinator. Real work is delegated to
//     four pluggable interfaces — InferenceProvider, TrainingDriver,
//     ModelLibrary, SavedQueryStore.
//   - Default wiring uses in-process mocks (mock_*.go) so the panels
//     light up against deterministic data with no model loaded and
//     no training running. The mocks are the testability story.
//   - Real implementations land later via NewService(Options{...})
//     from cmd/lthn — go-mlx for inference, pkg/training for the
//     LoRA driver, pkg/ml for the saved-query store, filesystem
//     scan for the model library.
//
// Why this shape: per the RFC, the workbench is the unified surface
// over four independent substrates (Influx + DuckDB + .model files +
// LoRA training). Interface seams keep each substrate swap-in-place
// — mock today, real-mlx tomorrow, dispatched-remote later. The
// service stays stable across all three.

package lab

import (
	core "dappco.re/go"
)

// Service is the ML Lab coordinator. Methods are the wire surface
// the Lit views consume; each delegates to a pluggable interface.
// Construct via NewService; default options install mocks.
type Service struct {
	provider InferenceProvider
	driver   TrainingDriver
	library  ModelLibrary
	saved    SavedQueryStore
}

// InferenceProvider handles surfaces that need to touch model
// weights or call into the LQL engine. Mock today, real-mlx /
// go-inference later. All methods accept the request DTO and
// return the result DTO with an error — the service wraps to
// core.Result at the boundary.
type InferenceProvider interface {
	InspectModelPack(path string) (ModelPackInspection, error)
	RunLQL(req LQLRequest) (LQLResult, error)
	RewriteAsk(req AskRequest) (RewrittenQuery, error)
}

// TrainingDriver owns the LoRA tab's control plane and the run
// registry. StartRun returns a handle; ControlRun acts on a
// running id; SubscribeRunEvents returns a channel the SSE handler
// reads frames from; ListRuns is the registry snapshot.
type TrainingDriver interface {
	StartRun(req StartRunRequest) (RunHandle, error)
	ControlRun(req RunControl) (RunStatus, error)
	SubscribeRunEvents(runID string) (<-chan RunEvent, error)
	ListRuns() ([]LoRARun, error)
}

// ModelLibrary lists the .model / .train / .vindex artefacts the
// Models tab renders. Default mock returns a fixture set; the real
// implementation walks ~/Lethean/data/models/ + per-config extra
// paths.
type ModelLibrary interface {
	ListModels() ([]ModelRow, error)
}

// SavedQueryStore is the CRUD surface backing the per-tab saved-
// query sidebar. Default mock holds entries in memory; real
// implementation persists to the lab_saved_queries DuckDB table
// in pkg/ml.
type SavedQueryStore interface {
	SaveQuery(q SavedQuery) error
	ListSavedQueries(tab string) ([]SavedQuery, error)
	DeleteSavedQuery(id string) error
}

// Options carries pluggable dependencies. Any nil field gets a
// mock — so NewService(Options{}) produces a fully-mocked service
// the UI can render against immediately.
type Options struct {
	Provider InferenceProvider
	Driver   TrainingDriver
	Library  ModelLibrary
	Saved    SavedQueryStore
}

// NewService builds a LabService with the given options, filling
// any nil with the default mock so the result is always wired.
//
// Usage example:
//
//	svc := lab.NewService(lab.Options{})  // fully mocked
//	rows, _ := svc.ListModels()
func NewService(opts Options) *Service {
	if opts.Provider == nil {
		opts.Provider = NewMockProvider()
	}
	if opts.Driver == nil {
		opts.Driver = NewMockDriver()
	}
	if opts.Library == nil {
		opts.Library = NewMockLibrary()
	}
	if opts.Saved == nil {
		opts.Saved = NewMemorySavedQueryStore()
	}
	return &Service{
		provider: opts.Provider,
		driver:   opts.Driver,
		library:  opts.Library,
		saved:    opts.Saved,
	}
}

// Register is the core.WithName-compatible factory for app.go's
// Services array. Returns Ok wrapping a fully-mocked *Service —
// the desktop binary can replace mocks later via a second
// constructor call.
//
// Usage example (cmd/lthn/app.go):
//
//	core.WithName("lab", lab.Register)
func Register(_ *core.Core) core.Result {
	return core.Ok(NewService(Options{}))
}

// ServiceName satisfies the Wails3 Service contract. Surfaces to
// the frontend as `@desktop/lab/service` — the binding name the
// Lit views import when they bypass HTTP and call directly.
func (s *Service) ServiceName() string { return "Lab" }

// ServiceStartup runs once at Wails boot. No state to warm up
// today; reserved for future per-boot work (load saved-query
// cache, scan model library, etc).
func (s *Service) ServiceStartup(_ core.Context, _ any) core.Result {
	return core.Ok(nil)
}

// ServiceShutdown runs once at Wails teardown. Closes the
// training driver to drain in-flight goroutines cleanly.
func (s *Service) ServiceShutdown() core.Result {
	if closer, ok := s.driver.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			return core.Fail(core.E("lab.ServiceShutdown", "driver close", err))
		}
	}
	return core.Ok(nil)
}

// ListModels returns the current model artefact library snapshot.
// Used by the Models tab + the topbar selected-model dropdown.
func (s *Service) ListModels() core.Result {
	rows, err := s.library.ListModels()
	if err != nil {
		return core.Fail(core.E("lab.ListModels", "library list", err))
	}
	return core.Ok(ModelsQueryResponse{Models: rows})
}

// ListRuns returns the current LoRA run registry snapshot. Used
// by the LoRA tab's run picker + the topbar selected-run dropdown.
func (s *Service) ListRuns() core.Result {
	runs, err := s.driver.ListRuns()
	if err != nil {
		return core.Fail(core.E("lab.ListRuns", "driver list", err))
	}
	return core.Ok(RunsQueryResponse{Runs: runs})
}

// StartRun launches a new LoRA training run via the driver.
// Returns RunHandle{RunID} so the frontend can subscribe to events
// immediately.
func (s *Service) StartRun(req StartRunRequest) core.Result {
	h, err := s.driver.StartRun(req)
	if err != nil {
		return core.Fail(core.E("lab.StartRun", "driver start", err))
	}
	return core.Ok(h)
}

// ControlRun applies a verb (pause / resume / cancel / checkpoint)
// to a running run and returns the post-action status.
func (s *Service) ControlRun(req RunControl) core.Result {
	st, err := s.driver.ControlRun(req)
	if err != nil {
		return core.Fail(core.E("lab.ControlRun", "driver control", err))
	}
	return core.Ok(st)
}

// SubscribeRunEvents returns a channel the SSE handler reads
// frames from until the run terminates. Closed by the driver on
// completion / cancellation.
func (s *Service) SubscribeRunEvents(runID string) (<-chan RunEvent, error) {
	return s.driver.SubscribeRunEvents(runID)
}

// QueryInflux executes the Flux / InfluxQL body and returns the
// shape-tagged result. Body is delegated to the provider's
// underlying client — mock returns a deterministic time-series.
func (s *Service) QueryInflux(req InfluxQuery) core.Result {
	// Mock today: the InfluxProvider interface is collapsed into
	// the InferenceProvider for now; expand if a real Influx
	// backend lands. See RFC.ml-lab.md §4.
	if mock, ok := s.provider.(InfluxQueryer); ok {
		result, err := mock.QueryInflux(req)
		if err != nil {
			return core.Fail(core.E("lab.QueryInflux", "provider query", err))
		}
		return core.Ok(result)
	}
	return core.Ok(InfluxResult{})
}

// QueryDuckDB executes the SQL body against the underlying DuckDB
// handle. Read-only enforcement happens at the handler entry —
// mutating statements are rejected before reaching this method.
func (s *Service) QueryDuckDB(req DuckDBQuery) core.Result {
	if mock, ok := s.provider.(DuckDBQueryer); ok {
		result, err := mock.QueryDuckDB(req)
		if err != nil {
			return core.Fail(core.E("lab.QueryDuckDB", "provider query", err))
		}
		return core.Ok(result)
	}
	return core.Ok(DuckDBResult{})
}

// InspectModelPack returns the read-only inspector summary for the
// named .model / .train / .vindex file.
func (s *Service) InspectModelPack(path string) core.Result {
	insp, err := s.provider.InspectModelPack(path)
	if err != nil {
		return core.Fail(core.E("lab.InspectModelPack", "provider inspect", err))
	}
	return core.Ok(insp)
}

// RunLQL executes one LQL statement against the active session.
// Returns a shape-tagged LQLResult — Kind discriminates the
// frontend render path.
func (s *Service) RunLQL(req LQLRequest) core.Result {
	result, err := s.provider.RunLQL(req)
	if err != nil {
		return core.Fail(core.E("lab.RunLQL", "provider lql", err))
	}
	return core.Ok(result)
}

// RewriteAsk turns a natural-language ASK box prompt into a target-
// tab + editor body the frontend pastes into the active editor.
func (s *Service) RewriteAsk(req AskRequest) core.Result {
	rew, err := s.provider.RewriteAsk(req)
	if err != nil {
		return core.Fail(core.E("lab.RewriteAsk", "provider ask", err))
	}
	return core.Ok(rew)
}

// SaveQuery persists q into the saved-query store. ID may be
// empty — the store generates one if so.
func (s *Service) SaveQuery(q SavedQuery) core.Result {
	if err := s.saved.SaveQuery(q); err != nil {
		return core.Fail(core.E("lab.SaveQuery", "store save", err))
	}
	return core.Ok(nil)
}

// ListSavedQueries returns the saved queries for the named tab.
func (s *Service) ListSavedQueries(tab string) core.Result {
	queries, err := s.saved.ListSavedQueries(tab)
	if err != nil {
		return core.Fail(core.E("lab.ListSavedQueries", "store list", err))
	}
	return core.Ok(SavedQueriesResponse{Queries: queries})
}

// DeleteSavedQuery removes the entry with the given id.
func (s *Service) DeleteSavedQuery(id string) core.Result {
	if err := s.saved.DeleteSavedQuery(id); err != nil {
		return core.Fail(core.E("lab.DeleteSavedQuery", "store delete", err))
	}
	return core.Ok(nil)
}

// InfluxQueryer is an opt-in extension a Provider may implement to
// serve QueryInflux. Default mock implements it; real wiring may
// split off into its own provider later.
type InfluxQueryer interface {
	QueryInflux(req InfluxQuery) (InfluxResult, error)
}

// DuckDBQueryer is the sibling for QueryDuckDB.
type DuckDBQueryer interface {
	QueryDuckDB(req DuckDBQuery) (DuckDBResult, error)
}
