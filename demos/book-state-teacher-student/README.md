# go-ai book-state teacher/student demo

This demo keeps inference in Go. The Gradio app is only a browser UI that calls
the `go-ai` JSON handler exposed by `go/cmd/book-state-demo`.

## Mock smoke test

```sh
go run ./go/cmd/book-state-demo \
  -mock \
  -title "Meditations" \
  -excerpt-file /Users/snider/Code/lthn/LEM/training/lem/composure/transparency-aurelius-meditations.txt
```

Then, from this directory:

```sh
python3 app.py
```

Open the Gradio URL and ask a question about the book state.

## With real local runtimes

Start any OpenAI-compatible local endpoint backed by `go-mlx`, `go-rocm`, or
another `go-inference` driver, then point the demo host at those endpoints:

```sh
go run ./go/cmd/book-state-demo \
  -title "Meditations" \
  -excerpt-file /Users/snider/Code/lthn/LEM/training/lem/composure/transparency-aurelius-meditations.txt \
  -entry-uri memvid://aurelius/meditations \
  -teacher-url http://127.0.0.1:8080 \
  -teacher-model gemma4-e2b \
  -student-url http://127.0.0.1:8081 \
  -student-model gemma3-1b
```

The student is unaided by default. Pass `-student-uses-state` to inject the same
book-state context into both student and teacher calls.
