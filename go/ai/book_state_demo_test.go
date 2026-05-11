// SPDX-License-Identifier: EUPL-1.2

package ai

import (
	"context"
	"testing"

	core "dappco.re/go"
	"dappco.re/go/inference"
	inferstate "dappco.re/go/inference/state"
)

func TestBookStateDemo_Ask_Good_TeacherUsesBookState(t *testing.T) {
	student := &routerFakeModel{modelType: "student", output: "Verus taught discipline."}
	teacher := &routerFakeModel{modelType: "teacher", output: "The book says gentleness and meekness."}
	demo := mustBookStateDemo(t, BookStateDemoConfig{
		State: BookState{
			Title:        "Meditations",
			Excerpt:      "From my grandfather Verus I learned good morals and the government of my temper.",
			EntryURI:     "mlx://aurelius/full-book/chapter-001",
			PrefixTokens: 1448,
		},
		StudentRoutes: []ProviderRoute{{Name: "student", ModelID: "student-small", Model: student}},
		TeacherRoutes: []ProviderRoute{{Name: "teacher", ModelID: "teacher-state", Model: teacher}},
	})

	result := demo.Ask(context.Background(), BookStateAskRequest{
		Question:  "What did Marcus learn from Verus?",
		MaxTokens: 24,
	})

	if !result.OK {
		t.Fatalf("Ask() error = %s", result.Error())
	}
	response := result.Value.(BookStateAskResponse)
	if response.StudentAnswer != "Verus taught discipline." || response.TeacherAnswer != "The book says gentleness and meekness." {
		t.Fatalf("Ask() = %+v, want student and teacher outputs", response)
	}
	if response.State.Title != "Meditations" || response.State.PrefixTokens != 1448 {
		t.Fatalf("State = %+v, want book state metadata", response.State)
	}
	if len(student.lastMessages) != 1 || core.Contains(student.lastMessages[0].Content, "grandfather Verus") {
		t.Fatalf("student messages = %+v, want unaided student question", student.lastMessages)
	}
	if len(teacher.lastMessages) < 2 || !core.Contains(teacher.lastMessages[0].Content, "grandfather Verus") {
		t.Fatalf("teacher messages = %+v, want book-state context", teacher.lastMessages)
	}
	if !core.Contains(teacher.lastMessages[len(teacher.lastMessages)-1].Content, "Student answer") {
		t.Fatalf("teacher prompt = %+v, want student answer included", teacher.lastMessages)
	}
	if response.Student.ModelID != "student-small" || response.Teacher.ModelID != "teacher-state" {
		t.Fatalf("routes = %+v/%+v, want provider metadata", response.Student, response.Teacher)
	}
}

func TestBookStateDemo_Ask_Good_StudentCanUseBookState(t *testing.T) {
	student := &routerFakeModel{modelType: "student", output: "Gentleness."}
	teacher := &routerFakeModel{modelType: "teacher", output: "Correct."}
	demo := mustBookStateDemo(t, BookStateDemoConfig{
		State:                BookState{Title: "Meditations", Excerpt: "gentleness and meekness"},
		StudentUsesBookState: true,
		StudentRoutes:        []ProviderRoute{{Name: "student", ModelID: "student", Model: student}},
		TeacherRoutes:        []ProviderRoute{{Name: "teacher", ModelID: "teacher", Model: teacher}},
	})

	result := demo.Ask(context.Background(), BookStateAskRequest{Question: "What lesson?", MaxTokens: 8})

	if !result.OK {
		t.Fatalf("Ask() error = %s", result.Error())
	}
	if len(student.lastMessages) < 2 || !core.Contains(student.lastMessages[0].Content, "gentleness and meekness") {
		t.Fatalf("student messages = %+v, want book-state context", student.lastMessages)
	}
}

func TestBookStateDemo_Ask_Bad_RejectsMissingQuestion(t *testing.T) {
	demo := mustBookStateDemo(t, BookStateDemoConfig{
		State:         BookState{Title: "Meditations"},
		TeacherRoutes: []ProviderRoute{{Name: "teacher", ModelID: "teacher", Model: &routerFakeModel{}}},
	})

	result := demo.Ask(context.Background(), BookStateAskRequest{})

	if result.OK {
		t.Fatal("Ask() OK = true, want missing question failure")
	}
	if !core.Contains(result.Error(), "question is required") {
		t.Fatalf("Ask() error = %q, want question validation", result.Error())
	}
}

func TestBookStateDemo_NewBookStateDemo_Ugly_RejectsMissingTeacher(t *testing.T) {
	result := NewBookStateDemo(BookStateDemoConfig{State: BookState{Title: "Meditations"}})

	if result.OK {
		t.Fatal("NewBookStateDemo() OK = true, want missing teacher failure")
	}
	if !core.Contains(result.Error(), "teacher route") {
		t.Fatalf("NewBookStateDemo() error = %q, want teacher route validation", result.Error())
	}
}

func TestBookStateContextAssembler_Good_FormatsState(t *testing.T) {
	assembler := BookStateContextAssembler{State: BookState{
		Title:        "Meditations",
		Excerpt:      "Verus taught gentleness.",
		EntryURI:     "mlx://entry",
		BundleURI:    "mlx://bundle",
		PrefixTokens: 12,
		Labels:       map[string]string{"source": "state"},
	}}

	text, err := assembler.AssembleContext(context.Background(), []inference.Message{{Role: "user", Content: "question"}})

	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	for _, want := range []string{"Meditations", "Verus taught gentleness", "mlx://entry", "prefix_tokens: 12", "source=state"} {
		if !core.Contains(text, want) {
			t.Fatalf("AssembleContext() = %q, want %q", text, want)
		}
	}
}

func TestBookStateFromWakeResult_Good_CopiesInferenceStateMetadata(t *testing.T) {
	wake := inferstate.WakeResult{
		Entry:        inferstate.Ref{URI: "memvid://entry", Title: "Meditations", Labels: map[string]string{"chapter": "one"}},
		Bundle:       inferstate.StateRef{URI: "memvid://bundle"},
		Index:        inferstate.StateRef{URI: "memvid://index"},
		PrefixTokens: 1448,
		BundleTokens: 91732,
		BlockSize:    2048,
		BlocksRead:   45,
		Labels:       map[string]string{"source": "wake"},
	}

	state := BookStateFromWakeResult(wake)

	if state.Title != "Meditations" || state.EntryURI != "memvid://entry" || state.BundleURI != "memvid://bundle" || state.IndexURI != "memvid://index" {
		t.Fatalf("BookStateFromWakeResult() = %+v, want URIs and title copied", state)
	}
	if state.PrefixTokens != 1448 || state.BundleTokens != 91732 || state.BlockSize != 2048 || state.BlocksRead != 45 {
		t.Fatalf("BookStateFromWakeResult() = %+v, want state counters copied", state)
	}
	if state.Labels["source"] != "wake" || state.Labels["chapter"] != "one" {
		t.Fatalf("Labels = %+v, want wake and entry labels merged", state.Labels)
	}
}

func TestBookStateFromRef_Good_CopiesDurableRefMetadata(t *testing.T) {
	ref := inferstate.Ref{
		URI:        "memvid://entry",
		BundleURI:  "memvid://bundle",
		Title:      "Meditations",
		Kind:       "book",
		Hash:       "sha256:test",
		TokenStart: 10,
		TokenCount: 20,
		ByteStart:  30,
		ByteCount:  40,
		Labels:     map[string]string{"source": "ref"},
	}

	state := BookStateFromRef(ref)

	if state.EntryURI != "memvid://entry" || state.BundleURI != "memvid://bundle" || state.PrefixTokens != 20 {
		t.Fatalf("BookStateFromRef() = %+v, want ref URIs and token count", state)
	}
	for _, want := range []string{"book", "sha256:test", "10", "30", "40"} {
		found := false
		for _, value := range state.Metadata {
			if value == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("Metadata = %+v, want value %q", state.Metadata, want)
		}
	}
}

func mustBookStateDemo(t *testing.T, cfg BookStateDemoConfig) *BookStateDemo {
	t.Helper()
	result := NewBookStateDemo(cfg)
	if !result.OK {
		t.Fatalf("NewBookStateDemo() error = %s", result.Error())
	}
	return result.Value.(*BookStateDemo)
}
