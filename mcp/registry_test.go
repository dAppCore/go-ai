// SPDX-License-Identifier: EUPL-1.2

package mcp

import (
	"testing"
)

func TestToolRegistry_Good_RecordsTools(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	tools := svc.Tools()
	if len(tools) == 0 {
		t.Fatal("expected non-empty tool registry")
	}

	found := false
	for _, tr := range tools {
		if tr.Name == "file_read" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file_read in tool registry")
	}
}

func TestToolRegistry_Good_SchemaExtraction(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	var record ToolRecord
	for _, tr := range svc.Tools() {
		if tr.Name == "file_read" {
			record = tr
			break
		}
	}
	if record.Name == "" {
		t.Fatal("file_read not found in registry")
	}

	if record.InputSchema == nil {
		t.Fatal("expected non-nil InputSchema for file_read")
	}

	props, ok := record.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map in InputSchema")
	}

	if _, ok := props["path"]; !ok {
		t.Error("expected 'path' property in file_read InputSchema")
	}
}

func TestToolRegistry_Good_ToolCount(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	tools := svc.Tools()
	// Built-in tools: file_read, file_write, file_delete, file_rename,
	// file_exists, file_edit, dir_list, dir_create, lang_detect, lang_list
	const expectedCount = 10
	if len(tools) != expectedCount {
		t.Errorf("expected %d tools, got %d", expectedCount, len(tools))
		for _, tr := range tools {
			t.Logf("  - %s (%s)", tr.Name, tr.Group)
		}
	}
}

func TestToolRegistry_Good_GroupAssignment(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	fileTools := []string{"file_read", "file_write", "file_delete", "file_rename", "file_exists", "file_edit", "dir_list", "dir_create"}
	langTools := []string{"lang_detect", "lang_list"}

	byName := make(map[string]ToolRecord)
	for _, tr := range svc.Tools() {
		byName[tr.Name] = tr
	}

	for _, name := range fileTools {
		tr, ok := byName[name]
		if !ok {
			t.Errorf("tool %s not found in registry", name)
			continue
		}
		if tr.Group != "files" {
			t.Errorf("tool %s: expected group 'files', got %q", name, tr.Group)
		}
	}

	for _, name := range langTools {
		tr, ok := byName[name]
		if !ok {
			t.Errorf("tool %s not found in registry", name)
			continue
		}
		if tr.Group != "language" {
			t.Errorf("tool %s: expected group 'language', got %q", name, tr.Group)
		}
	}
}

func TestToolRegistry_Good_ToolRecordFields(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	var record ToolRecord
	for _, tr := range svc.Tools() {
		if tr.Name == "file_write" {
			record = tr
			break
		}
	}
	if record.Name == "" {
		t.Fatal("file_write not found in registry")
	}

	if record.Name != "file_write" {
		t.Errorf("expected Name 'file_write', got %q", record.Name)
	}
	if record.Description == "" {
		t.Error("expected non-empty Description")
	}
	if record.Group == "" {
		t.Error("expected non-empty Group")
	}
	if record.InputSchema == nil {
		t.Error("expected non-nil InputSchema")
	}
	if record.OutputSchema == nil {
		t.Error("expected non-nil OutputSchema")
	}
}
