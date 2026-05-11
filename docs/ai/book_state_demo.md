<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/book_state_demo.go — teacher / student book-state demo

**Package**: `dappco.re/go/ai/ai`
**File**: `go/ai/book_state_demo.go`

## What this is

The orchestration layer for **teacher / student question answering over a persisted book state**. The demo that closed the most recent CoreAgent sprint — proves the disaggregated inference loop end-to-end:

1. A `BookState` captures a knowledge pack (Aurelius's Meditations, in the canonical example) — title, excerpt, URI of the persisted state bundle.
2. A teacher route (large model with the book state injected) and an optional student route (small model, unaided by default) answer the same question.
3. The demo collects both responses + audit metadata into one `BookStateAskResponse`.

The point: a 26B teacher with a wake'd book-state bundle vs a 2B student with no context. Side-by-side, identical question. Headline measurement: the answer quality delta is the book-state contribution.

## BookState

```go
type BookState struct {
    Title        string             // "Meditations"
    Excerpt      string             // short snippet shown inline
    URI          string             // bundle root URI
    EntryURI     string             // specific entry within the bundle
    BundleURI    string             // bundle binary URI
    IndexURI     string             // bundle index URI
    StoreURI     string             // store root URI
    PrefixTokens int                // restored prefix length
    BundleTokens int                // total bundle tokens
    BlockSize    int                // KV block size
    BlocksRead   int                // blocks restored at wake
    Labels       map[string]string
    Metadata     map[string]string
}
```

Built directly or via:

```go
state := ai.BookStateFromWakeResult(wakeResult)   // from inference/state.WakeResult
state := ai.BookStateFromRef(stateRef)             // from inference/state.Ref
```

The two `From*` builders adapt from the portable shapes so the demo doesn't tie callers to one wake path. A test fixture can build a BookState manually; a production wake produces one via `BookStateFromWakeResult`.

## BookStateContextAssembler

```go
type BookStateContextAssembler struct { State BookState }

func (a BookStateContextAssembler) AssembleContext(ctx, []Message) (string, error)
```

Implements `ProviderContextAssembler` — the hook the router calls to inject context before sending the prompt to the provider. Renders the BookState as text (title + excerpt + URIs + token counts) and the router prepends it to messages with the configured `ContextPrefix`.

Teacher path gets this; student path gets it only if `StudentUsesBookState: true`.

## BookStateDemoConfig

```go
type BookStateDemoConfig struct {
    State              BookState
    TeacherRoutes      []ProviderRoute  // required (≥1)
    StudentRoutes      []ProviderRoute  // optional

    StudentUsesBookState bool            // default false (unaided baseline)
    MaxTokens            int
    TeacherMaxTokens     int             // separate cap — teachers tend to answer longer
    StudentMaxTokens     int             // typically smaller
    Temperature          float32
}
```

Defaults:

- `defaultBookStateMaxTokens = 256` — shared cap when per-role unset
- `defaultBookStateTeacherMaxTokens = 256`
- `defaultBookStateStudentMaxTokens = 128`

The smaller default student cap reflects "student is unaided; smaller answer". The user-supplied `StudentUsesBookState: true` lifts the student into the same context-injection as the teacher for direct apples-to-apples comparison.

## BookStateAskRequest / Response

```go
type BookStateAskRequest struct {
    Question             string
    MaxTokens            int     // overrides demo-wide
    TeacherMaxTokens     int
    StudentMaxTokens     int
    Temperature          float32
    StudentUsesBookState *bool   // per-request override (nullable)
}

type BookStateAskResponse struct {
    Question      string
    State         BookState               // echoed for audit
    StudentAnswer string                  // empty if no student configured
    TeacherAnswer string
    Student       ProviderChatResponse    // full provider response — attempts, metrics
    Teacher       ProviderChatResponse
    CreatedAtUnix int64
}
```

The Response carries both raw answers (string convenience) AND the full ProviderChatResponse (provider + model + per-attempt failure log + metrics) so audit/scoring can interrogate fallback behaviour.

## Ask flow

```
Ask(ctx, req):
   build BookStateContextAssembler
   ├── student configured?
   │      ├── student.Chat(ProviderChatRequest{
   │      │       Prompt:           question,
   │      │       ContextAssembler: assembler,
   │      │       DisableContext:   !studentUsesState,  // student gets context only when asked
   │      │       Labels:           {"role": "student"},
   │      │   })
   │      └── studentAnswer = result.Text
   │
   └── teacher.Chat(ProviderChatRequest{
           Messages:         [{Role: user, Content: teacherPrompt(question, studentAnswer)}],
           ContextAssembler: assembler,
           ContextPrefix:    "Book state:\n",
           Labels:           {"role": "teacher"},
       })

   return BookStateAskResponse{...}
```

`teacherPrompt(q, s)` formats the teacher prompt to include both the question AND the student's attempt — invites the teacher to address gaps rather than just answer cold.

## Why student is optional

The simplest demo is teacher-only. The student adds the side-by-side comparison view. Production scoring uses the student to establish baseline (no-book-state); the teacher's delta against the student measures the book-state contribution.

Run with `-student-uses-state` and both legs use the book state — the comparison then measures teacher capability delta, not book-state contribution.

## Used by

- `cmd/book-state-demo/` — Go entry serving the HTTP handler
- `demos/book-state-teacher-student/app.py` — Gradio UI that calls the HTTP handler

## Related

- [book_state_demo_http.md](book_state_demo_http.md) — HTTP wrapper
- [provider_router.md](provider_router.md) — the router both legs use
- [../../../go-mlx/docs/memory/agent_memory.md](../../../go-mlx/docs/memory/agent_memory.md) — what produces the WakeResult that becomes a BookState
- [../../cmd/book-state-demo.md](../cmd/book-state-demo.md) — entry binary
