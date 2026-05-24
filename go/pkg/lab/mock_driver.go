// SPDX-Licence-Identifier: EUPL-1.2

package lab

import (
	"math"
	"sync"
	"time"

	core "dappco.re/go"
)

// MockTrainingDriver simulates a LoRA training driver — StartRun
// returns a handle and spawns a goroutine that emits RunEvent
// frames on a per-run channel until the run completes, errors, or
// is cancelled. Loss + throughput follow realistic curves so the
// frontend chart panels render meaningful shapes.
//
// Defaults: 200 total iterations, 200ms per tick → ~40s per run.
// Loss decays exponentially with jitter; throughput plateaus
// around 64 tokens/sec with mild noise. Override per-StartRun via
// req.TotalIters / req.Hyperparams["tick_ms"] when the driver
// boots into a faster cadence for tests.
//
// Thread-safe: per-run state lives behind one RWMutex. Channels
// are buffered (32 frames) so a slow SSE consumer doesn't
// backpressure the tick loop — overflow drops the oldest frame
// rather than blocking the goroutine.
type MockTrainingDriver struct {
	mu     sync.RWMutex
	runs   map[string]*mockRunState
	nextID int
}

// mockRunState carries one run's runtime — cancel + tick control,
// the SSE-facing channel, and a rolling snapshot the registry
// surfaces through ListRuns. Status mutations happen under the
// driver's mu; tick-loop reads of the snapshot happen on the
// goroutine, so mu protects against control vs tick races.
type mockRunState struct {
	snapshot LoRARun
	events   chan RunEvent
	stop     chan struct{}
	pause    chan bool
	paused   bool
	done     bool
	startAt  time.Time
}

// NewMockDriver returns an empty MockTrainingDriver. Default
// wiring when no real driver is plugged into NewService.
//
// Usage example:
//
//	d := lab.NewMockDriver()
//	h, _ := d.StartRun(lab.StartRunRequest{BaseModel: "lemma-12b-v4-p6", Mode: lab.LoRAModeSFT})
//	evs, _ := d.SubscribeRunEvents(h.RunID)
//	for ev := range evs { /* render tick */ }
func NewMockDriver() *MockTrainingDriver {
	return &MockTrainingDriver{
		runs: make(map[string]*mockRunState),
	}
}

// StartRun launches a simulated run and spawns the tick goroutine.
// Defaults: 200 iterations, 200ms per tick. Hyperparam overrides:
// "tick_ms" (int) and req.TotalIters control the cadence /
// horizon. Returns a RunHandle the frontend uses for control +
// subscribe calls.
func (d *MockTrainingDriver) StartRun(req StartRunRequest) (RunHandle, error) {
	d.mu.Lock()
	d.nextID++
	id := "mock-run-" + core.Sprintf("%04d-%d", d.nextID, time.Now().Unix())
	totalIters := req.TotalIters
	if totalIters <= 0 {
		totalIters = 200
	}
	tickMS := 200
	if v, ok := req.Hyperparams["tick_ms"]; ok {
		if n, ok := v.(int); ok && n > 0 {
			tickMS = n
		}
		if n, ok := v.(float64); ok && n > 0 {
			tickMS = int(n)
		}
	}
	mode := req.Mode
	if mode == "" {
		mode = LoRAModeSFT
	}
	baseModel := req.BaseModel
	if baseModel == "" {
		baseModel = "lemma-12b-v4-p6"
	}
	now := time.Now().UTC()
	state := &mockRunState{
		snapshot: LoRARun{
			ID:            id,
			BaseModel:     baseModel,
			Dataset:       req.Dataset,
			Mode:          mode,
			Status:        LoRAStatusActive,
			Iteration:     0,
			TotalIters:    totalIters,
			LastLoss:      mockLoss(0, totalIters),
			ThroughputTPS: 0,
			ETASeconds:    totalIters * tickMS / 1000,
			Started:       now.Format(time.RFC3339),
			TrainPath:     "/Users/agent/Lethean/data/train/" + id + ".train",
		},
		events:  make(chan RunEvent, 32),
		stop:    make(chan struct{}),
		pause:   make(chan bool, 1),
		startAt: now,
	}
	d.runs[id] = state
	d.mu.Unlock()

	go d.tickLoop(id, state, time.Duration(tickMS)*time.Millisecond)
	return RunHandle{RunID: id}, nil
}

// ControlRun applies the requested action to the named run and
// returns the post-action status. Unknown ids return an error.
func (d *MockTrainingDriver) ControlRun(req RunControl) (RunStatus, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.runs[req.RunID]
	if !ok {
		return RunStatus{}, core.E("lab.MockDriver.ControlRun", "unknown run: "+req.RunID, nil)
	}
	if state.done {
		return RunStatus{RunID: req.RunID, Status: state.snapshot.Status}, nil
	}
	switch req.Action {
	case RunControlPause:
		state.paused = true
		state.snapshot.Status = LoRAStatusPaused
		select {
		case state.pause <- true:
		default:
		}
	case RunControlResume:
		state.paused = false
		state.snapshot.Status = LoRAStatusActive
		select {
		case state.pause <- false:
		default:
		}
	case RunControlCancel:
		state.snapshot.Status = LoRAStatusCancelled
		state.done = true
		close(state.stop)
	case RunControlCheckpoint:
		// Emit a checkpoint frame without changing status.
		ev := RunEvent{
			RunID:         req.RunID,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			Kind:          "checkpoint",
			Iteration:     state.snapshot.Iteration,
			Loss:          state.snapshot.LastLoss,
			ThroughputTPS: state.snapshot.ThroughputTPS,
			ETASeconds:    state.snapshot.ETASeconds,
			Message:       "manual checkpoint",
		}
		select {
		case state.events <- ev:
		default:
		}
	default:
		return RunStatus{}, core.E("lab.MockDriver.ControlRun", "unknown action: "+string(req.Action), nil)
	}
	return RunStatus{RunID: req.RunID, Status: state.snapshot.Status}, nil
}

// SubscribeRunEvents returns the per-run event channel. The
// channel is closed when the run terminates. Unknown ids return
// an error.
func (d *MockTrainingDriver) SubscribeRunEvents(runID string) (<-chan RunEvent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	state, ok := d.runs[runID]
	if !ok {
		return nil, core.E("lab.MockDriver.SubscribeRunEvents", "unknown run: "+runID, nil)
	}
	return state.events, nil
}

// ListRuns returns a snapshot of every known run, including
// completed / cancelled / failed ones. Order: most-recent first.
// Includes seeded "history" runs so the LoRA tab has content
// even before the user starts a fresh run.
func (d *MockTrainingDriver) ListRuns() ([]LoRARun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]LoRARun, 0, len(d.runs)+3)
	for _, state := range d.runs {
		out = append(out, state.snapshot)
	}
	// Seed history rows so the panel feels populated on a fresh
	// boot. Drop these once a real driver / history table backs
	// the registry.
	out = append(out, seedHistoryRuns()...)
	// Stable sort: started descending.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Started > out[j-1].Started; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// Close signals every active tick loop to terminate. Idempotent.
func (d *MockTrainingDriver) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, state := range d.runs {
		if !state.done {
			state.done = true
			close(state.stop)
		}
	}
	return nil
}

// tickLoop emits one RunEvent per interval until the run finishes
// (iteration hits total) or stop fires. Pause-aware: when paused,
// the loop blocks on pause until a resume signal lands.
func (d *MockTrainingDriver) tickLoop(id string, state *mockRunState, interval time.Duration) {
	defer func() {
		d.mu.Lock()
		state.done = true
		close(state.events)
		d.mu.Unlock()
	}()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-state.stop:
			return
		case <-t.C:
		}
		d.mu.Lock()
		paused := state.paused
		d.mu.Unlock()
		if paused {
			// Drain pause signal then re-check; lets resume re-enter
			// the tick loop without losing a tick.
			select {
			case <-state.stop:
				return
			case resumed := <-state.pause:
				if !resumed {
					// Resume signal received — continue to next tick.
				}
			}
			continue
		}
		d.mu.Lock()
		state.snapshot.Iteration++
		state.snapshot.LastLoss = mockLoss(state.snapshot.Iteration, state.snapshot.TotalIters)
		state.snapshot.ThroughputTPS = mockThroughput(state.snapshot.Iteration)
		remaining := state.snapshot.TotalIters - state.snapshot.Iteration
		if remaining < 0 {
			remaining = 0
		}
		state.snapshot.ETASeconds = remaining * int(interval/time.Millisecond) / 1000
		iter := state.snapshot.Iteration
		loss := state.snapshot.LastLoss
		tps := state.snapshot.ThroughputTPS
		eta := state.snapshot.ETASeconds
		total := state.snapshot.TotalIters
		ev := RunEvent{
			RunID:         id,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			Kind:          "tick",
			Iteration:     iter,
			Loss:          loss,
			ThroughputTPS: tps,
			ETASeconds:    eta,
		}
		if iter >= total {
			ev.Kind = "complete"
			ev.Message = "training complete"
			state.snapshot.Status = LoRAStatusCompleted
		}
		d.mu.Unlock()
		select {
		case state.events <- ev:
		default:
			// Channel full — drop oldest, push new. The Lit view
			// receiving the SSE stream is the bottleneck; the
			// alternative (block) would freeze the tick loop.
			select {
			case <-state.events:
			default:
			}
			select {
			case state.events <- ev:
			default:
			}
		}
		if ev.Kind == "complete" {
			return
		}
	}
}

// mockLoss returns a realistic decaying-loss value at the given
// iteration. Same envelope LoRA fine-tunes produce in practice:
// fast initial drop, slower tail, mild noise from the modulo.
func mockLoss(iter, total int) float64 {
	if total <= 0 {
		total = 200
	}
	progress := float64(iter) / float64(total)
	base := 2.4*math.Exp(-3.2*progress) + 0.18
	jitter := 0.04 * math.Sin(float64(iter)*0.31)
	return round4(base + jitter)
}

// mockThroughput returns a noisy plateau around 64 tokens/sec —
// realistic for an M3 Ultra running a 12B LoRA SFT.
func mockThroughput(iter int) float64 {
	base := 62.0 + 8.0*math.Cos(float64(iter)*0.17)
	return round4(base)
}

// seedHistoryRuns returns the fixture history rows the LoRA panel
// shows when no live run is in flight.
func seedHistoryRuns() []LoRARun {
	return []LoRARun{
		{
			ID:            "run-2026-05-21-3-1b-cl-bpl",
			BaseModel:     "lemma-3-1b-cl-bpl",
			Dataset:       "datasets/cl-bpl-watts-allen-v3.jsonl",
			Mode:          LoRAModeDistill,
			Status:        LoRAStatusCompleted,
			Iteration:     1024,
			TotalIters:    1024,
			LastLoss:      0.142,
			ThroughputTPS: 81.4,
			ETASeconds:    0,
			Started:       "2026-05-21T19:48:51Z",
			TrainPath:     "/Users/agent/Lethean/data/train/lemma-3-1b-cl-bpl.train",
		},
		{
			ID:            "run-2026-05-20-qwen-grpo",
			BaseModel:     "qwen-3-32b-think",
			Dataset:       "datasets/qwen-grpo-think-pairs-v2.jsonl",
			Mode:          LoRAModeGRPO,
			Status:        LoRAStatusPaused,
			Iteration:     332,
			TotalIters:    800,
			LastLoss:      0.587,
			ThroughputTPS: 24.8,
			ETASeconds:    1248,
			Started:       "2026-05-20T13:02:31Z",
			TrainPath:     "/Users/agent/Lethean/data/train/qwen-grpo-think.train",
		},
		{
			ID:            "run-2026-05-18-12b-sft-failed",
			BaseModel:     "lemma-12b-v4-p5",
			Dataset:       "datasets/lethean-prep-v1.jsonl",
			Mode:          LoRAModeSFT,
			Status:        LoRAStatusFailed,
			Iteration:     14,
			TotalIters:    200,
			LastLoss:      0,
			ThroughputTPS: 0,
			ETASeconds:    0,
			Started:       "2026-05-18T08:14:19Z",
			TrainPath:     "/Users/agent/Lethean/data/train/lemma-12b-v4-p5-attempt-2.train",
		},
	}
}
