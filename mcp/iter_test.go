// SPDX-License-Identifier: EUPL-1.2

package mcp

import (
	"slices"
	"testing"
)

func TestService_Iterators(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	// Test ToolsSeq
	tools := slices.Collect(svc.ToolsSeq())
	if len(tools) == 0 {
		t.Error("expected non-empty ToolsSeq")
	}
	if len(tools) != len(svc.Tools()) {
		t.Errorf("ToolsSeq length %d != Tools() length %d", len(tools), len(svc.Tools()))
	}

	// Test SubsystemsSeq
	subsystems := slices.Collect(svc.SubsystemsSeq())
	if len(subsystems) != len(svc.Subsystems()) {
		t.Errorf("SubsystemsSeq length %d != Subsystems() length %d", len(subsystems), len(svc.Subsystems()))
	}
}

func TestRegistry_SplitTagSeq(t *testing.T) {
	tag := "name,omitempty,json"
	parts := slices.Collect(splitTagSeq(tag))
	expected := []string{"name", "omitempty", "json"}

	if !slices.Equal(parts, expected) {
		t.Errorf("expected %v, got %v", expected, parts)
	}
}
