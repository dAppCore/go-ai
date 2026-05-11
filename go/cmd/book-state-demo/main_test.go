// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"context"
	"testing"

	core "dappco.re/go"
	ai "dappco.re/go/ai/ai"
)

func TestBookStateDemoCommand_buildBookStateDemo_Good_MockTeacher(t *testing.T) {
	result := buildBookStateDemo(bookStateDemoOptions{
		Mock:        true,
		Title:       "Meditations",
		Excerpt:     "gentleness and meekness",
		TeacherName: "teacher",
	})
	if !result.OK {
		t.Fatalf("buildBookStateDemo() error = %s", result.Error())
	}
	demo := result.Value.(*ai.BookStateDemo)

	answerResult := demo.Ask(context.Background(), ai.BookStateAskRequest{Question: "What lesson?"})
	if !answerResult.OK {
		t.Fatalf("Ask() error = %s", answerResult.Error())
	}
	response := answerResult.Value.(ai.BookStateAskResponse)
	if !core.Contains(response.TeacherAnswer, "mock teacher") {
		t.Fatalf("TeacherAnswer = %q, want mock teacher answer", response.TeacherAnswer)
	}
}

func TestBookStateDemoCommand_buildBookStateDemo_Bad_RejectsMissingTeacherURL(t *testing.T) {
	result := buildBookStateDemo(bookStateDemoOptions{
		Title:        "Meditations",
		TeacherModel: "gemma",
	})

	if result.OK {
		t.Fatal("buildBookStateDemo() OK = true, want missing teacher-url failure")
	}
	if !core.Contains(result.Error(), "teacher-url") {
		t.Fatalf("error = %q, want teacher-url validation", result.Error())
	}
}

func TestBookStateDemoCommand_buildBookStateDemo_Ugly_ReusesBookStateMetadata(t *testing.T) {
	result := buildBookStateDemo(bookStateDemoOptions{
		Mock:         true,
		Title:        "Meditations",
		EntryURI:     "memvid://entry",
		PrefixTokens: 1448,
	})
	if !result.OK {
		t.Fatalf("buildBookStateDemo() error = %s", result.Error())
	}
	demo := result.Value.(*ai.BookStateDemo)
	state := demo.State()

	if state.EntryURI != "memvid://entry" || state.PrefixTokens != 1448 {
		t.Fatalf("state = %+v, want configured metadata", state)
	}
}
