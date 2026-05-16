// SPDX-License-Identifier: EUPL-1.2

// book-state-demo serves a tiny Go API for teacher/student demos backed by
// go-ai provider routes. Python/Gradio clients call this process; model
// inference remains behind Go inference contracts.
package main

import (
	"context"
	"flag"
	"iter"
	"net/http"
	"time"

	core "dappco.re/go"
	ai "dappco.re/go/ai/ai"
	openaiprovider "dappco.re/go/ai/providers/openai"
	"dappco.re/go/inference"
)

type bookStateDemoOptions struct {
	Addr string

	Title        string
	Excerpt      string
	ExcerptFile  string
	URI          string
	EntryURI     string
	BundleURI    string
	IndexURI     string
	StoreURI     string
	PrefixTokens int
	BundleTokens int
	BlockSize    int
	BlocksRead   int

	TeacherName  string
	TeacherURL   string
	TeacherModel string
	TeacherKey   string

	StudentName          string
	StudentURL           string
	StudentModel         string
	StudentKey           string
	StudentUsesBookState bool

	Mock bool
}

func main() {
	if result := run(); !result.OK {
		core.Print(core.Stderr(), "%s\n", result.Error())
		core.Exit(1)
	}
}

func run() core.Result {
	options := parseBookStateDemoOptions()
	demoResult := buildBookStateDemo(options)
	if !demoResult.OK {
		return demoResult
	}
	addr := core.Trim(options.Addr)
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	demo := demoResult.Value.(*ai.BookStateDemo)
	core.Print(nil, "book-state demo listening on http://%s", addr)
	err := http.ListenAndServe(addr, ai.NewBookStateDemoHandler(demo))
	if err != nil {
		return core.Fail(core.E("book-state-demo.run", "serve HTTP", err))
	}
	return core.Ok(nil)
}

func parseBookStateDemoOptions() bookStateDemoOptions {
	options := bookStateDemoOptions{
		Addr:        defaultString(core.Getenv("CORE_BOOK_STATE_DEMO_ADDR"), "127.0.0.1:8787"),
		Title:       defaultString(core.Getenv("CORE_BOOK_STATE_TITLE"), "Book state"),
		TeacherName: defaultString(core.Getenv("CORE_BOOK_STATE_TEACHER_NAME"), "teacher"),
		StudentName: defaultString(core.Getenv("CORE_BOOK_STATE_STUDENT_NAME"), "student"),
	}
	flag.StringVar(&options.Addr, "addr", options.Addr, "HTTP listen address")
	flag.StringVar(&options.Title, "title", options.Title, "book/state title")
	flag.StringVar(&options.Excerpt, "excerpt", core.Getenv("CORE_BOOK_STATE_EXCERPT"), "short book/state excerpt")
	flag.StringVar(&options.ExcerptFile, "excerpt-file", core.Getenv("CORE_BOOK_STATE_EXCERPT_FILE"), "file to read as the book/state excerpt")
	flag.StringVar(&options.URI, "uri", core.Getenv("CORE_BOOK_STATE_URI"), "state URI")
	flag.StringVar(&options.EntryURI, "entry-uri", core.Getenv("CORE_BOOK_STATE_ENTRY_URI"), "state entry URI")
	flag.StringVar(&options.BundleURI, "bundle-uri", core.Getenv("CORE_BOOK_STATE_BUNDLE_URI"), "state bundle URI")
	flag.StringVar(&options.IndexURI, "index-uri", core.Getenv("CORE_BOOK_STATE_INDEX_URI"), "state index URI")
	flag.StringVar(&options.StoreURI, "store-uri", core.Getenv("CORE_BOOK_STATE_STORE_URI"), "state store URI")
	flag.IntVar(&options.PrefixTokens, "prefix-tokens", 0, "restored prefix token count")
	flag.IntVar(&options.BundleTokens, "bundle-tokens", 0, "bundle token count")
	flag.IntVar(&options.BlockSize, "block-size", 0, "state block size")
	flag.IntVar(&options.BlocksRead, "blocks-read", 0, "restored/read block count")
	flag.StringVar(&options.TeacherName, "teacher-name", options.TeacherName, "teacher provider name")
	flag.StringVar(&options.TeacherURL, "teacher-url", core.Getenv("CORE_BOOK_STATE_TEACHER_URL"), "OpenAI-compatible teacher base URL")
	flag.StringVar(&options.TeacherModel, "teacher-model", core.Getenv("CORE_BOOK_STATE_TEACHER_MODEL"), "teacher model id")
	flag.StringVar(&options.TeacherKey, "teacher-key", core.Getenv("CORE_BOOK_STATE_TEACHER_KEY"), "teacher API key")
	flag.StringVar(&options.StudentName, "student-name", options.StudentName, "student provider name")
	flag.StringVar(&options.StudentURL, "student-url", core.Getenv("CORE_BOOK_STATE_STUDENT_URL"), "OpenAI-compatible student base URL")
	flag.StringVar(&options.StudentModel, "student-model", core.Getenv("CORE_BOOK_STATE_STUDENT_MODEL"), "student model id")
	flag.StringVar(&options.StudentKey, "student-key", core.Getenv("CORE_BOOK_STATE_STUDENT_KEY"), "student API key")
	flag.BoolVar(&options.StudentUsesBookState, "student-uses-state", false, "inject book state into the student call as well as the teacher call")
	flag.BoolVar(&options.Mock, "mock", false, "serve with fixed mock models instead of OpenAI-compatible endpoints")
	flag.Parse()
	return options
}

func buildBookStateDemo(options bookStateDemoOptions) core.Result {
	stateResult := bookStateFromOptions(options)
	if !stateResult.OK {
		return stateResult
	}
	teacherResult := routeFromOptions(options.TeacherName, options.TeacherURL, options.TeacherModel, options.TeacherKey, "mock teacher answer", options.Mock, true)
	if !teacherResult.OK {
		return teacherResult
	}
	teacherRoute := teacherResult.Value.(ai.ProviderRoute)

	var studentRoutes []ai.ProviderRoute
	if options.Mock || core.Trim(options.StudentURL) != "" || core.Trim(options.StudentModel) != "" {
		studentResult := routeFromOptions(options.StudentName, options.StudentURL, options.StudentModel, options.StudentKey, "mock student answer", options.Mock, false)
		if !studentResult.OK {
			return studentResult
		}
		studentRoutes = []ai.ProviderRoute{studentResult.Value.(ai.ProviderRoute)}
	}

	return ai.NewBookStateDemo(ai.BookStateDemoConfig{
		State:                stateResult.Value.(ai.BookState),
		TeacherRoutes:        []ai.ProviderRoute{teacherRoute},
		StudentRoutes:        studentRoutes,
		StudentUsesBookState: options.StudentUsesBookState,
	})
}

func bookStateFromOptions(options bookStateDemoOptions) core.Result {
	excerpt := core.Trim(options.Excerpt)
	if excerpt == "" && core.Trim(options.ExcerptFile) != "" {
		dataResult := core.ReadFile(options.ExcerptFile)
		if !dataResult.OK {
			if err, ok := dataResult.Value.(error); ok {
				return core.Fail(core.E("book-state-demo.state", "read excerpt file", err))
			}
			return core.Fail(core.E("book-state-demo.state", dataResult.Error(), nil))
		}
		excerpt = core.Trim(string(dataResult.Value.([]byte)))
	}
	return core.Ok(ai.BookState{
		Title:        options.Title,
		Excerpt:      excerpt,
		URI:          options.URI,
		EntryURI:     options.EntryURI,
		BundleURI:    options.BundleURI,
		IndexURI:     options.IndexURI,
		StoreURI:     options.StoreURI,
		PrefixTokens: options.PrefixTokens,
		BundleTokens: options.BundleTokens,
		BlockSize:    options.BlockSize,
		BlocksRead:   options.BlocksRead,
		Labels:       map[string]string{"source": "book-state-demo"},
	})
}

func routeFromOptions(name, baseURL, modelID, apiKey, mockOutput string, mock bool, required bool) core.Result {
	name = defaultString(name, "provider")
	if mock {
		return core.Ok(ai.ProviderRoute{
			Name:    name,
			ModelID: defaultString(modelID, name),
			Model:   &staticTextModel{modelType: "mock", output: mockOutput},
			Labels:  map[string]string{"mode": "mock"},
		})
	}
	if core.Trim(baseURL) == "" {
		if required {
			return core.Fail(core.E("book-state-demo.route", "teacher-url is required unless -mock is set", nil))
		}
		return core.Ok(ai.ProviderRoute{})
	}
	if core.Trim(modelID) == "" {
		return core.Fail(core.E("book-state-demo.route", core.Concat(name, " model is required"), nil))
	}
	backend := openaiprovider.NewBackend(openaiprovider.Config{
		Name:         name,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: modelID,
	})
	modelResult := backend.LoadModel(modelID)
	if !modelResult.OK {
		if err, ok := modelResult.Value.(error); ok {
			return core.Fail(core.E("book-state-demo.route", "load provider model", err))
		}
		return core.Fail(core.E("book-state-demo.route", modelResult.Error(), nil))
	}
	model := modelResult.Value.(inference.TextModel)
	return core.Ok(ai.ProviderRoute{
		Name:    name,
		ModelID: modelID,
		Model:   model,
		Labels:  map[string]string{"mode": "openai-compatible"},
	})
}

func defaultString(value, fallback string) string {
	if core.Trim(value) == "" {
		return fallback
	}
	return value
}

type staticTextModel struct {
	modelType string
	output    string
	lastErr   error
	metrics   inference.GenerateMetrics
}

func (m *staticTextModel) Generate(ctx context.Context, prompt string, opts ...inference.GenerateOption) iter.Seq[inference.Token] {
	return m.Chat(ctx, []inference.Message{{Role: "user", Content: prompt}}, opts...)
}

func (m *staticTextModel) Chat(ctx context.Context, messages []inference.Message, opts ...inference.GenerateOption) iter.Seq[inference.Token] {
	return func(yield func(inference.Token) bool) {
		started := time.Now()
		m.lastErr = ctx.Err()
		if m.lastErr != nil {
			return
		}
		cfg := inference.ApplyGenerateOpts(opts)
		output := m.output
		if output == "" {
			output = "mock answer"
		}
		m.metrics = inference.GenerateMetrics{
			PromptTokens:    len(messages),
			GeneratedTokens: 1,
			TotalDuration:   time.Since(started),
		}
		if cfg.MaxTokens == 0 {
			return
		}
		yield(inference.Token{Text: output})
	}
}

func (m *staticTextModel) Classify(context.Context, []string, ...inference.GenerateOption) core.Result {
	return core.Fail(core.E("book-state-demo.static.Classify", "classification is not supported", nil))
}

func (m *staticTextModel) BatchGenerate(ctx context.Context, prompts []string, opts ...inference.GenerateOption) core.Result {
	results := make([]inference.BatchResult, 0, len(prompts))
	for _, prompt := range prompts {
		var tokens []inference.Token
		for token := range m.Generate(ctx, prompt, opts...) {
			tokens = append(tokens, token)
		}
		batch := inference.BatchResult{Tokens: tokens}
		if errResult := m.Err(); !errResult.OK {
			if err, ok := errResult.Value.(error); ok {
				batch.Err = err
			} else {
				batch.Err = core.E("book-state-demo.static.BatchGenerate", errResult.Error(), nil)
			}
		}
		results = append(results, batch)
	}
	return core.Ok(results)
}

func (m *staticTextModel) ModelType() string {
	if core.Trim(m.modelType) == "" {
		return "mock"
	}
	return m.modelType
}

func (m *staticTextModel) Info() inference.ModelInfo {
	return inference.ModelInfo{Architecture: m.ModelType()}
}

func (m *staticTextModel) Metrics() inference.GenerateMetrics {
	return m.metrics
}

func (m *staticTextModel) Err() core.Result {
	if m.lastErr != nil {
		return core.Fail(m.lastErr)
	}
	return core.Ok(nil)
}

func (m *staticTextModel) Close() core.Result {
	return core.Ok(nil)
}
