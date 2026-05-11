<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# cmd/book-state-demo — book-state demo entry binary

**Package**: `dappco.re/go/ai/cmd/book-state-demo`
**Files**: `cmd/book-state-demo/main.go` + `main_test.go`

## What this is

The Go entry binary for the **teacher / student book-state demo**. Builds a `BookStateDemo` from CLI flags + env vars, mounts the JSON HTTP handler, listens on 127.0.0.1:8787 by default. The Gradio Python UI in `demos/book-state-teacher-student/app.py` is its companion frontend.

## CLI flags

```
-addr            127.0.0.1:8787       (CORE_BOOK_STATE_DEMO_ADDR)
-title           "Book state"          (CORE_BOOK_STATE_TITLE)
-excerpt         ""                   (CORE_BOOK_STATE_EXCERPT)
-excerpt-file    ""                   (CORE_BOOK_STATE_EXCERPT_FILE)
-uri             ""                   (CORE_BOOK_STATE_URI)
-entry-uri       ""                   (CORE_BOOK_STATE_ENTRY_URI)
-bundle-uri      ""                   (CORE_BOOK_STATE_BUNDLE_URI)
-index-uri       ""                   (CORE_BOOK_STATE_INDEX_URI)
-store-uri       ""                   (CORE_BOOK_STATE_STORE_URI)
-prefix-tokens   0
-bundle-tokens   0
-block-size      0
-blocks-read     0

-teacher-name    "teacher"            (CORE_BOOK_STATE_TEACHER_NAME)
-teacher-url     ""                   (CORE_BOOK_STATE_TEACHER_URL)
-teacher-model   ""                   (CORE_BOOK_STATE_TEACHER_MODEL)
-teacher-key     ""                   (CORE_BOOK_STATE_TEACHER_KEY)

-student-name    "student"            (CORE_BOOK_STATE_STUDENT_NAME)
-student-url     ""                   (CORE_BOOK_STATE_STUDENT_URL)
-student-model   ""                   (CORE_BOOK_STATE_STUDENT_MODEL)
-student-key     ""                   (CORE_BOOK_STATE_STUDENT_KEY)

-student-uses-state    bool    (default: false)
-mock                  bool    (default: false)
```

Every flag has an env var counterpart for systemd / launchd / Docker configuration without CLI.

## Modes

### Mock mode

```sh
go run ./go/cmd/book-state-demo \
  -mock \
  -title "Meditations" \
  -excerpt-file /Users/snider/Code/lthn/LEM/training/lem/composure/transparency-aurelius-meditations.txt
```

Mock mode replaces both teacher and student with `staticTextModel` instances that return fixed strings. Used for:

- Smoke testing the HTTP handler + Gradio UI without standing up real models
- Local development without Violet running
- CI integration tests

### Real-runtime mode

```sh
# Start a Violet sidecar (or any OpenAI-compatible local endpoint) for the teacher.
# Start another for the student (smaller model).

go run ./go/cmd/book-state-demo \
  -title "Meditations" \
  -excerpt-file /Users/snider/.../meditations.txt \
  -entry-uri memvid://aurelius/meditations \
  -teacher-url http://127.0.0.1:8080 \
  -teacher-model gemma4-e2b \
  -student-url http://127.0.0.1:8081 \
  -student-model gemma3-1b
```

The teacher / student `-url` flags point at any OpenAI-compatible endpoint — Violet, raw `go-inference/openai` handler, vLLM, llama.cpp server, Ollama. The demo dials them as outbound providers via `go-ai/providers/openai`.

`-student-uses-state` lifts the student into the same book-state context as the teacher — for A/B "did context help at all" comparison.

## Source structure

```go
func main() { core.Exit if !run().OK }

func run() core.Result {
    options := parseBookStateDemoOptions()
    demoResult := buildBookStateDemo(options)
    if !demoResult.OK { return demoResult }
    demo := demoResult.Value.(*ai.BookStateDemo)
    addr := defaultString(options.Addr, "127.0.0.1:8787")
    return ListenAndServe(addr, ai.NewBookStateDemoHandler(demo))
}
```

Small. The binary's job is parse + build + serve; all logic is in the `ai` package.

## staticTextModel (mock)

```go
type staticTextModel struct {
    modelType string
    output    string
    lastErr   error
    metrics   inference.GenerateMetrics
}
```

Implements `inference.TextModel` returning a fixed string from `Generate` / `Chat`, errors from `Classify`, sequential pass-through from `BatchGenerate`. Used only in mock mode.

## Used by

- The Gradio Python UI in `demos/book-state-teacher-student/`
- ad-hoc curl scripts for sanity testing the handler
- Eval pipelines comparing teacher/student behaviour

## Related

- [../ai/book_state_demo.md](../ai/book_state_demo.md) — the demo this serves
- [../ai/book_state_demo_http.md](../ai/book_state_demo_http.md) — the HTTP handler
- [../ai/provider_router.md](../ai/provider_router.md) — routing under the hood
- `../../../demos/book-state-teacher-student/README.md` — Gradio UI flow
- `../../../go-mlx/docs/cmd/violet.md` — Violet sidecar that often backs the teacher endpoint
