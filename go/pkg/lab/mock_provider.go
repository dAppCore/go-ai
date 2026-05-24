// SPDX-Licence-Identifier: EUPL-1.2

package lab

import (
	"math"
	"time"

	core "dappco.re/go"
)

// MockInferenceProvider satisfies InferenceProvider + InfluxQueryer
// + DuckDBQueryer with deterministic shaped data. Used by the
// default-mocked LabService so the panels render meaningful values
// without a real go-mlx / InfluxDB / DuckDB backend wired.
//
// All methods are pure functions of their inputs — the same query
// returns the same shape every call. Time-series methods use a
// stable epoch (the package init time) so frontend tests can assert
// frame counts without flakiness.
type MockInferenceProvider struct {
	epoch time.Time
}

// NewMockProvider returns a MockInferenceProvider stamped with the
// current time as its epoch. Subsequent calls return values
// relative to the epoch — restart the provider for a fresh window.
//
// Usage example:
//
//	p := lab.NewMockProvider()
//	result, _ := p.QueryInflux(lab.InfluxQuery{Body: "from(bucket:\"train\")"})
//	// result.Rows lights up the Influx tab's chart
func NewMockProvider() *MockInferenceProvider {
	return &MockInferenceProvider{epoch: time.Now().UTC().Add(-1 * time.Hour)}
}

// InspectModelPack returns a stub inspection for the given path.
// Synthesises arch / quant / ctx from the filename so the inspector
// panel reads consistently with the mock library's rows.
func (p *MockInferenceProvider) InspectModelPack(path string) (ModelPackInspection, error) {
	insp := ModelPackInspection{
		Path:         path,
		Format:       "trix",
		Arch:         "gemma-4",
		Quant:        "q4",
		Ctx:          131072,
		Layers:       62,
		Vocab:        262144,
		Tokenizer:    "sentencepiece",
		ChatTemplate: "gemma-4",
		Capabilities: []string{"reasoning", "tool-use", "chat-template"},
		SizeBytes:    7_834_124_288,
		Hash:         "a3d9c812f4567b8e92d145a6c70be8b1d23f4a5b6c7d8e9f0a1b2c3d4e5f6a7b",
		Notes:        "mock inspection — wire real go-mlx.InspectModelPack to replace",
	}
	if core.HasSuffix(path, ".vindex") {
		insp.Format = "vindex"
		insp.SizeBytes = 16_777_216
	}
	if core.HasSuffix(path, ".train") {
		insp.Format = "train"
		insp.SizeBytes = 18_212_491_264
	}
	return insp, nil
}

// RunLQL returns a shape-tagged stub result keyed on the statement
// prefix. Sufficient for the Models tab to render each result kind
// (table / stats / tree / tokens / trace) without a real vindex.
func (p *MockInferenceProvider) RunLQL(req LQLRequest) (LQLResult, error) {
	stmt := core.Trim(req.Statement)
	switch {
	case core.HasPrefix(core.Upper(stmt), "STATS"):
		return LQLResult{
			Kind: "stats",
			Stats: map[string]any{
				"edges":           1_482_309,
				"layers":          62,
				"features":        262_144,
				"patches_active":  3,
				"vindex_size_mb":  16,
				"extracted":       "2026-04-12T19:02:08Z",
				"extractor":       "go-mlx-vindex@dev",
			},
			Message: "mock STATS — replace with real vindex.STATS",
		}, nil
	case core.HasPrefix(core.Upper(stmt), "INFER"):
		return LQLResult{
			Kind: "tokens",
			Tokens: []TokenScore{
				{Token: " kindness", Probability: 0.214, Logit: -1.541, Rank: 1},
				{Token: " patience", Probability: 0.118, Logit: -2.135, Rank: 2},
				{Token: " grace", Probability: 0.087, Logit: -2.441, Rank: 3},
				{Token: " honesty", Probability: 0.063, Logit: -2.762, Rank: 4},
				{Token: " care", Probability: 0.051, Logit: -2.975, Rank: 5},
			},
		}, nil
	case core.HasPrefix(core.Upper(stmt), "TRACE"):
		out := make([]LayerTrace, 0, 8)
		for i := 0; i < 8; i++ {
			layer := i*8 + 1
			out = append(out, LayerTrace{
				Layer:       layer,
				Probability: 0.04 + float64(i)*0.06,
				Rank:        24 - i*3,
				AttnContrib: 0.30 - float64(i)*0.02,
				FFNContrib:  0.20 + float64(i)*0.03,
			})
		}
		return LQLResult{Kind: "trace", Trace: out}, nil
	case core.HasPrefix(core.Upper(stmt), "SHOW"):
		return LQLResult{
			Kind: "table",
			Columns: []ColumnSpec{
				{Name: "name", Kind: "string"},
				{Name: "kind", Kind: "string"},
				{Name: "size", Kind: "int"},
			},
			Rows: [][]any{
				{"lemma-12b-v4-p6", "model", 7834124288},
				{"lemma-12b-v4-p6", "train", 18212491264},
				{"lemma-3-1b-cl-bpl", "model", 1073741824},
			},
		}, nil
	default:
		return LQLResult{
			Kind:    "confirmation",
			Message: "mock RunLQL — statement parsed: " + stmt,
		}, nil
	}
}

// RewriteAsk produces a deterministic NL→target-tab rewrite. Keys
// on a few obvious prompt fragments; everything else falls through
// to the DuckDB tab with a passthrough SELECT.
func (p *MockInferenceProvider) RewriteAsk(req AskRequest) (RewrittenQuery, error) {
	low := core.Lower(req.Prompt)
	switch {
	case core.Contains(low, "loss") || core.Contains(low, "training"):
		return RewrittenQuery{
			TargetTab: "influx",
			EditorBody: `from(bucket: "train")
  |> range(start: ${start}, stop: ${stop})
  |> filter(fn: (r) => r._measurement == "loss")`,
			Explanation: "Asking about loss / training metrics — Flux query against the train bucket.",
		}, nil
	case core.Contains(low, "show models") || core.Contains(low, "list models"):
		return RewrittenQuery{
			TargetTab:   "models",
			EditorBody:  "SHOW MODELS;",
			Explanation: "Asking about local model artefacts — LQL SHOW MODELS against the active vindex.",
		}, nil
	case core.Contains(low, "infer") || core.Contains(low, "predict"):
		return RewrittenQuery{
			TargetTab:   "models",
			EditorBody:  `INFER "The model chose " TOP 5;`,
			Explanation: "Asking for a prediction — LQL INFER against the active vindex.",
		}, nil
	default:
		return RewrittenQuery{
			TargetTab:   "duckdb",
			EditorBody:  "SELECT * FROM lora_runs ORDER BY started_at DESC LIMIT 20;",
			Explanation: "Falling through to a recent-runs DuckDB query — refine the prompt for a more specific rewrite.",
		}, nil
	}
}

// QueryInflux returns a deterministic time-series shaped for the
// chart panel. Generates 60 points across the request's time
// range (defaulting to the provider epoch ↦ now) with a smoothly
// descending "loss" series mirrored by an ascending "throughput"
// series — the same envelope a real LoRA fine-tune produces.
func (p *MockInferenceProvider) QueryInflux(req InfluxQuery) (InfluxResult, error) {
	start, stop := p.epoch, time.Now().UTC()
	if req.TimeRange.Start != "" {
		if t, err := time.Parse(time.RFC3339, req.TimeRange.Start); err == nil {
			start = t
		}
	}
	if req.TimeRange.Stop != "" {
		if t, err := time.Parse(time.RFC3339, req.TimeRange.Stop); err == nil {
			stop = t
		}
	}
	const n = 60
	span := stop.Sub(start)
	if span <= 0 {
		span = time.Hour
		stop = start.Add(span)
	}
	step := span / time.Duration(n)
	rows := make([][]any, 0, n)
	for i := 0; i < n; i++ {
		t := start.Add(step * time.Duration(i))
		progress := float64(i) / float64(n-1)
		loss := 2.4*math.Exp(-3.2*progress) + 0.18 + 0.04*math.Sin(progress*7.0)
		throughput := 38.0 + 24.0*progress + 3.0*math.Cos(progress*5.0)
		rows = append(rows, []any{
			t.Format(time.RFC3339),
			round4(loss),
			round4(throughput),
		})
	}
	return InfluxResult{
		Columns: []ColumnSpec{
			{Name: "ts", Kind: "time"},
			{Name: "loss", Kind: "float"},
			{Name: "throughput_tps", Kind: "float"},
		},
		Rows:      rows,
		ChartHint: "line",
	}, nil
}

// QueryDuckDB returns a deterministic small result. Keys on a few
// obvious SQL prefixes; everything else returns an empty result
// with a "mock" note in the cursor field so the frontend can flag
// "not yet wired" cleanly.
func (p *MockInferenceProvider) QueryDuckDB(req DuckDBQuery) (DuckDBResult, error) {
	sql := core.Upper(core.Trim(req.SQL))
	switch {
	case core.Contains(sql, "LORA_RUNS"):
		return DuckDBResult{
			Columns: []ColumnSpec{
				{Name: "id", Kind: "string"},
				{Name: "base_model", Kind: "string"},
				{Name: "mode", Kind: "string"},
				{Name: "status", Kind: "string"},
				{Name: "last_loss", Kind: "float"},
				{Name: "started_at", Kind: "time"},
			},
			Rows: [][]any{
				{"run-2026-05-22-12b-v4-p6-rsft", "lemma-12b-v4-p6", "sft", "active", 0.318, "2026-05-22T08:14:02Z"},
				{"run-2026-05-21-3-1b-cl-bpl", "lemma-3-1b-cl-bpl", "distill", "completed", 0.142, "2026-05-21T19:48:51Z"},
				{"run-2026-05-20-qwen-grpo", "qwen-3-32b-think", "grpo", "paused", 0.587, "2026-05-20T13:02:31Z"},
			},
			RowCountTotal: 3,
		}, nil
	case core.Contains(sql, "LAB_SAVED_QUERIES"):
		return DuckDBResult{
			Columns: []ColumnSpec{
				{Name: "id", Kind: "string"},
				{Name: "tab", Kind: "string"},
				{Name: "name", Kind: "string"},
				{Name: "use_count", Kind: "int"},
			},
			Rows:          [][]any{},
			RowCountTotal: 0,
		}, nil
	default:
		return DuckDBResult{
			Columns: []ColumnSpec{
				{Name: "result", Kind: "string"},
			},
			Rows: [][]any{
				{"mock backend — wire real DuckDB to replace"},
			},
			RowCountTotal: 1,
		}, nil
	}
}

func round4(f float64) float64 { return math.Round(f*10000) / 10000 }
