// SPDX-License-Identifier: EUPL-1.2

package brain

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// --- Nil bridge tests (headless mode) ---

func TestBrainRemember_Bad_NilBridge(t *testing.T) {
	sub := New(nil)
	_, _, err := sub.brainRemember(context.Background(), nil, RememberInput{
		Content: "test memory",
		Type:    "observation",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

func TestBrainRecall_Bad_NilBridge(t *testing.T) {
	sub := New(nil)
	_, _, err := sub.brainRecall(context.Background(), nil, RecallInput{
		Query: "how does scoring work?",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

func TestBrainForget_Bad_NilBridge(t *testing.T) {
	sub := New(nil)
	_, _, err := sub.brainForget(context.Background(), nil, ForgetInput{
		ID: "550e8400-e29b-41d4-a716-446655440000",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

func TestBrainList_Bad_NilBridge(t *testing.T) {
	sub := New(nil)
	_, _, err := sub.brainList(context.Background(), nil, ListInput{
		Project: "eaas",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// --- Subsystem interface tests ---

func TestSubsystem_Good_Name(t *testing.T) {
	sub := New(nil)
	if sub.Name() != "brain" {
		t.Errorf("expected Name() = 'brain', got %q", sub.Name())
	}
}

func TestSubsystem_Good_ShutdownNoop(t *testing.T) {
	sub := New(nil)
	if err := sub.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// --- Struct round-trip tests ---

func TestRememberInput_Good_RoundTrip(t *testing.T) {
	in := RememberInput{
		Content:    "LEM scoring was blind to negative emotions",
		Type:       "bug",
		Tags:       []string{"scoring", "lem"},
		Project:    "eaas",
		Confidence: 0.95,
		Supersedes: "550e8400-e29b-41d4-a716-446655440000",
		ExpiresIn:  24,
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out RememberInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Content != in.Content || out.Type != in.Type {
		t.Errorf("round-trip mismatch: content or type")
	}
	if len(out.Tags) != 2 || out.Tags[0] != "scoring" {
		t.Errorf("round-trip mismatch: tags")
	}
	if out.Confidence != 0.95 {
		t.Errorf("round-trip mismatch: confidence %f != 0.95", out.Confidence)
	}
}

func TestRememberOutput_Good_RoundTrip(t *testing.T) {
	in := RememberOutput{
		Success:   true,
		MemoryID:  "550e8400-e29b-41d4-a716-446655440000",
		Timestamp: time.Now().Truncate(time.Second),
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out RememberOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !out.Success || out.MemoryID != in.MemoryID {
		t.Errorf("round-trip mismatch: %+v != %+v", out, in)
	}
}

func TestRecallInput_Good_RoundTrip(t *testing.T) {
	in := RecallInput{
		Query: "how does verdict classification work?",
		TopK:  5,
		Filter: RecallFilter{
			Project:       "eaas",
			MinConfidence: 0.5,
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out RecallInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Query != in.Query || out.TopK != 5 {
		t.Errorf("round-trip mismatch: query or topK")
	}
	if out.Filter.Project != "eaas" || out.Filter.MinConfidence != 0.5 {
		t.Errorf("round-trip mismatch: filter")
	}
}

func TestMemory_Good_RoundTrip(t *testing.T) {
	in := Memory{
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		AgentID:    "virgil",
		Type:       "decision",
		Content:    "Use Qdrant for vector search",
		Tags:       []string{"architecture", "openbrain"},
		Project:    "php-agentic",
		Confidence: 0.9,
		CreatedAt:  "2026-03-03T12:00:00+00:00",
		UpdatedAt:  "2026-03-03T12:00:00+00:00",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out Memory
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.ID != in.ID || out.AgentID != "virgil" || out.Type != "decision" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

func TestForgetInput_Good_RoundTrip(t *testing.T) {
	in := ForgetInput{
		ID:     "550e8400-e29b-41d4-a716-446655440000",
		Reason: "Superseded by new approach",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out ForgetInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.ID != in.ID || out.Reason != in.Reason {
		t.Errorf("round-trip mismatch: %+v != %+v", out, in)
	}
}

func TestListInput_Good_RoundTrip(t *testing.T) {
	in := ListInput{
		Project: "eaas",
		Type:    "decision",
		AgentID: "charon",
		Limit:   20,
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out ListInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Project != "eaas" || out.Type != "decision" || out.AgentID != "charon" || out.Limit != 20 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

func TestListOutput_Good_RoundTrip(t *testing.T) {
	in := ListOutput{
		Success: true,
		Count:   2,
		Memories: []Memory{
			{ID: "id-1", AgentID: "virgil", Type: "decision", Content: "memory 1", Confidence: 0.9, CreatedAt: "2026-03-03T12:00:00+00:00", UpdatedAt: "2026-03-03T12:00:00+00:00"},
			{ID: "id-2", AgentID: "charon", Type: "bug", Content: "memory 2", Confidence: 0.8, CreatedAt: "2026-03-03T13:00:00+00:00", UpdatedAt: "2026-03-03T13:00:00+00:00"},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out ListOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !out.Success || out.Count != 2 || len(out.Memories) != 2 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}
