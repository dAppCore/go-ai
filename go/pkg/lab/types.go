// SPDX-Licence-Identifier: EUPL-1.2

// Package lab DTOs — wire types shared by the Wails service, the
// HTTP route group, and the frontend Lit views (ml-lab/*). Names
// + json tags are load-bearing: the Lit view interfaces declare
// the same shapes byte-for-byte so backend rounds-trip directly
// into the component's render path. See
// plans/project/lthn/desktop/RFC.ml-lab.md §3 + §6 + §7 + §10.
package lab

// ModelRow describes one .model / .train / .vindex artefact the
// Models tab lists. Mirrors the ModelRow TS interface in
// frontend/src/lit/views/ml-lab/models.ts — fingerprint is the
// sha256 hex of the pack identity projection per
// go-inference/go/model/pack.Fingerprint.
type ModelRow struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	Arch        string `json:"arch"`
	QuantBits   int    `json:"quant_bits"`
	Ctx         int    `json:"ctx"`
	SizeBytes   int64  `json:"size_bytes"`
	Fingerprint string `json:"fingerprint"`
	Created     string `json:"created"`
	Producer    string `json:"producer"`
}

// ModelsQueryResponse wraps a ModelRow list under the "models" key.
// The frontend treats an absent / empty "models" field as the
// degraded-empty case (zero artefacts known locally).
type ModelsQueryResponse struct {
	Models []ModelRow `json:"models"`
}

// LoRARunStatus enumerates the lifecycle states the LoRA tab
// renders. Mirrors the TS union in ml-lab/lora.ts. queued / active
// run colour cool-indigo; paused warm-amber; completed cool-green;
// cancelled / failed warm-red.
type LoRARunStatus string

const (
	LoRAStatusQueued    LoRARunStatus = "queued"
	LoRAStatusActive    LoRARunStatus = "active"
	LoRAStatusPaused    LoRARunStatus = "paused"
	LoRAStatusCompleted LoRARunStatus = "completed"
	LoRAStatusCancelled LoRARunStatus = "cancelled"
	LoRAStatusFailed    LoRARunStatus = "failed"
)

// LoRAMode is the training algorithm. SFT = supervised fine-tune,
// GRPO = group relative policy optimisation, distill = teacher /
// student distillation.
type LoRAMode string

const (
	LoRAModeSFT     LoRAMode = "sft"
	LoRAModeGRPO    LoRAMode = "grpo"
	LoRAModeDistill LoRAMode = "distill"
)

// LoRARun captures one training run's current state. Mirrors the
// TS LoRARun interface in ml-lab/lora.ts. iteration / total_iters
// drive the progress bar; last_loss + throughput_tps + eta_seconds
// drive the status strip; train_path locates the .train file the
// run is writing into.
type LoRARun struct {
	ID            string        `json:"id"`
	BaseModel     string        `json:"base_model"`
	Dataset       string        `json:"dataset"`
	Mode          LoRAMode      `json:"mode"`
	Status        LoRARunStatus `json:"status"`
	Iteration     int           `json:"iteration"`
	TotalIters    int           `json:"total_iters"`
	LastLoss      float64       `json:"last_loss"`
	ThroughputTPS float64       `json:"throughput_tps"`
	ETASeconds    int           `json:"eta_seconds"`
	Started       string        `json:"started"`
	TrainPath     string        `json:"train_path"`
}

// RunsQueryResponse wraps a LoRARun list under the "runs" key.
type RunsQueryResponse struct {
	Runs []LoRARun `json:"runs"`
}

// TimeRange brackets an Influx query window. Both ends ISO-8601.
// Defaults sit on the topbar; per-query overrides come from the
// Flux body template.
type TimeRange struct {
	Start string `json:"start"`
	Stop  string `json:"stop"`
}

// InfluxQuery is the body the Influx tab POSTs. body is raw Flux /
// InfluxQL; time_range binds as ${start} / ${stop} template vars;
// saved_query_id lets the backend resolve a stored body if the
// frontend prefers a reference over inlining.
type InfluxQuery struct {
	Body         string    `json:"body"`
	TimeRange    TimeRange `json:"time_range"`
	SavedQueryID string    `json:"saved_query_id,omitempty"`
}

// ColumnSpec is the per-column type tag the frontend uses to pick
// a renderer (chart vs table vs key-value). Kinds: "string", "int",
// "float", "time", "bool".
type ColumnSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// InfluxResult is the shape the Influx tab renders. rows[i][j]
// maps to columns[j]. chart_hint nominates a chart kind ("line",
// "bar", "heatmap") or "" when the result is best as a table.
type InfluxResult struct {
	Columns   []ColumnSpec `json:"columns"`
	Rows      [][]any      `json:"rows"`
	ChartHint string       `json:"chart_hint"`
}

// DuckDBQuery is the body the DuckDB tab POSTs. sql is SELECT-only
// (the backend rejects mutating statements at handler entry).
// bind_vars carries named parameters resolved before execution.
type DuckDBQuery struct {
	SQL          string         `json:"sql"`
	BindVars     map[string]any `json:"bind_vars,omitempty"`
	SavedQueryID string         `json:"saved_query_id,omitempty"`
}

// DuckDBResult mirrors InfluxResult's tabular surface. row_count
// totals the underlying SELECT, pagination_cursor advances the
// frontend through large results without re-querying.
type DuckDBResult struct {
	Columns          []ColumnSpec `json:"columns"`
	Rows             [][]any      `json:"rows"`
	RowCountTotal    int          `json:"row_count_total"`
	PaginationCursor string       `json:"pagination_cursor,omitempty"`
}

// StartRunRequest configures a new LoRA training run. hyperparams
// is a free-form map so the wizard can carry any tuning knob
// without DTO churn; the driver validates keys it recognises and
// surfaces unknowns as warnings on the returned handle.
type StartRunRequest struct {
	BaseModel       string         `json:"base_model"`
	Dataset         string         `json:"dataset"`
	Mode            LoRAMode       `json:"mode"`
	Hyperparams     map[string]any `json:"hyperparams,omitempty"`
	CheckpointEvery int            `json:"checkpoint_every,omitempty"`
	TotalIters      int            `json:"total_iters,omitempty"`
}

// RunHandle identifies a started run; the frontend stores this and
// uses it for subsequent control / event-subscription calls.
type RunHandle struct {
	RunID string `json:"run_id"`
}

// RunControlAction enumerates the control-bar verbs.
type RunControlAction string

const (
	RunControlPause      RunControlAction = "pause"
	RunControlResume     RunControlAction = "resume"
	RunControlCancel     RunControlAction = "cancel"
	RunControlCheckpoint RunControlAction = "checkpoint"
)

// RunControl is the body for /ml-lab/runs/:id/control.
type RunControl struct {
	RunID  string           `json:"run_id"`
	Action RunControlAction `json:"action"`
}

// RunStatus is the response to a control action — the post-action
// run state so the frontend can confirm without polling list.
type RunStatus struct {
	RunID  string        `json:"run_id"`
	Status LoRARunStatus `json:"status"`
}

// RunEvent is one frame of the run-event stream. SSE handler emits
// these as data: <json> per tick. Kind discriminates "tick" (in-
// progress sample), "complete" (terminal), "error" (terminal with
// message), "checkpoint" (mid-run checkpoint landed).
type RunEvent struct {
	RunID         string  `json:"run_id"`
	Timestamp     string  `json:"ts"`
	Kind          string  `json:"kind"`
	Iteration     int     `json:"iteration"`
	Loss          float64 `json:"loss"`
	ThroughputTPS float64 `json:"throughput_tps"`
	ETASeconds    int     `json:"eta_seconds"`
	Message       string  `json:"message,omitempty"`
}

// SavedQuery persists into the lab_saved_queries DuckDB table
// (see RFC §10). tab is one of "influx" / "duckdb" / "models" /
// "lora"; body is the raw query / recipe JSON.
type SavedQuery struct {
	ID         string `json:"id"`
	Tab        string `json:"tab"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at"`
	UseCount   int    `json:"use_count"`
}

// SavedQueriesResponse wraps a SavedQuery list under "queries".
type SavedQueriesResponse struct {
	Queries []SavedQuery `json:"queries"`
}

// LQLRequest is the body the Models tab REPL POSTs. use_target,
// when set, overrides the session's USE; otherwise the topbar's
// selected model is the default.
type LQLRequest struct {
	Statement string `json:"statement"`
	UseTarget string `json:"use_target,omitempty"`
}

// LQLResult is the shape-tagged response the Models tab renders
// per kind. Frontend dispatches on Kind ("table" / "stats" /
// "tree" / "tokens" / "trace" / "confirmation" / "progress").
// Fields not relevant to the selected kind stay zero / nil.
type LQLResult struct {
	Kind    string         `json:"kind"`
	Columns []ColumnSpec   `json:"columns,omitempty"`
	Rows    [][]any        `json:"rows,omitempty"`
	Stats   map[string]any `json:"stats,omitempty"`
	Tree    any            `json:"tree,omitempty"`
	Tokens  []TokenScore   `json:"tokens,omitempty"`
	Trace   []LayerTrace   `json:"trace,omitempty"`
	Message string         `json:"message,omitempty"`
}

// TokenScore is one entry in the INFER result. rank starts at 1.
type TokenScore struct {
	Token       string  `json:"token"`
	Probability float64 `json:"probability"`
	Logit       float64 `json:"logit"`
	Rank        int     `json:"rank"`
}

// LayerTrace is one row of a TRACE result. layer indexes into the
// model's transformer layer stack; rank is the target token's
// rank at that layer's projection.
type LayerTrace struct {
	Layer       int     `json:"layer"`
	Probability float64 `json:"probability"`
	Rank        int     `json:"rank"`
	AttnContrib float64 `json:"attn_contrib"`
	FFNContrib  float64 `json:"ffn_contrib"`
}

// AskRequest is the body the topbar ASK box POSTs. active_tab tells
// the rewriter where the user is (so the rewrite can stay on that
// tab unless the request only makes sense elsewhere).
type AskRequest struct {
	Prompt    string `json:"prompt"`
	ActiveTab string `json:"active_tab"`
}

// RewrittenQuery is the rewriter's reply. target_tab may differ
// from the request's active_tab — the frontend switches tabs in
// that case. editor_body populates the target tab's editor;
// explanation renders inline so the user sees the rationale.
type RewrittenQuery struct {
	TargetTab   string `json:"target_tab"`
	EditorBody  string `json:"editor_body"`
	Explanation string `json:"explanation"`
}

// ModelPackInspection is the read-only shape the Models tab's
// inspector panel renders. Stub today — the canonical type lives
// in go-inference/contracts and re-exported once that import is
// wired; this local copy keeps the route shape stable in the
// interim. Per RFC.ml-lab.md §3.
type ModelPackInspection struct {
	Path         string   `json:"path"`
	Format       string   `json:"format"`
	Arch         string   `json:"arch"`
	Quant        string   `json:"quant"`
	Ctx          int      `json:"ctx"`
	Layers       int      `json:"layers"`
	Vocab        int      `json:"vocab"`
	Tokenizer    string   `json:"tokenizer"`
	ChatTemplate string   `json:"chat_template"`
	Capabilities []string `json:"capabilities"`
	SizeBytes    int64    `json:"size_bytes"`
	Hash         string   `json:"hash"`
	Notes        string   `json:"notes,omitempty"`
}
