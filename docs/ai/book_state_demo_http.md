<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# ai/book_state_demo_http.go — HTTP handler for the demo

**Package**: `dappco.re/go/ai/ai`
**File**: `go/ai/book_state_demo_http.go`

## What this is

The **HTTP wrapper** around `BookStateDemo`. Exposes a minimal JSON API that the Gradio Python UI (`demos/book-state-teacher-student/app.py`) calls from the browser.

Three endpoints, one type per role, no SDK required on the Python side — just `requests.post(url, json=...)`.

## Endpoints

| Method | Path | Body | Response |
|--------|------|------|----------|
| `GET` | `/health` | — | `{"status": "ok"}` |
| `GET` | `/state` | — | the `BookState` JSON |
| `POST` | `/ask` | `BookStateAskRequest` | `BookStateAskResponse` |

That's the whole surface.

## Construction

```go
demo := ai.NewBookStateDemo(cfg).Value.(*ai.BookStateDemo)
handler := ai.NewBookStateDemoHandler(demo)
http.ListenAndServe("127.0.0.1:8787", handler)
```

## Error shape

Errors return a plain JSON object:

```json
{"error": "student failed: provider exhausted all attempts"}
```

HTTP status codes:

- `400` — bad request body, invalid JSON, validation failure inside the demo
- `404` — unknown path
- `405` — wrong HTTP method
- `500` — demo is nil (handler misconstructed)

## Why HTTP not Unix socket

The demo is meant to be poked from any tool that speaks HTTP. The Gradio UI lives in Python; future demos may live in Wails / WebView / Lit components in core/ide; ad-hoc testing happens via `curl`. HTTP+JSON wins on portability vs Unix-socket for this surface.

For high-throughput automated workloads, the Violet sidecar's Unix socket remains the production path.

## Wire example

```bash
curl -s http://127.0.0.1:8787/state | jq
# →
# {
#   "title": "Meditations",
#   "excerpt": "Whatever may happen to thee...",
#   "uri": "memvid://aurelius/meditations",
#   ...
# }

curl -s -X POST http://127.0.0.1:8787/ask \
     -H "Content-Type: application/json" \
     -d '{"question": "What does Aurelius say about patience?"}' | jq
# →
# {
#   "question": "What does Aurelius say about patience?",
#   "state":   {...},
#   "student_answer": "...",
#   "teacher_answer": "...",
#   "student": {...full ProviderChatResponse...},
#   "teacher": {...},
#   "created_at_unix": 1715438400
# }
```

## Used by

- `cmd/book-state-demo/main.go` — mounts this handler
- `demos/book-state-teacher-student/app.py` — Gradio UI POSTs `/ask`
- ad-hoc curl + jq for sanity checking

## Related

- [book_state_demo.md](book_state_demo.md) — the demo this wraps
- [provider_router.md](provider_router.md) — what produces the response objects
- `../../cmd/book-state-demo.md` — the binary entry
- `../../../demos/book-state-teacher-student/README.md` — Gradio UI flow
