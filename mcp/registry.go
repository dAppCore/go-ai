// SPDX-License-Identifier: EUPL-1.2

package mcp

import (
	"reflect"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolRecord captures metadata about a registered MCP tool.
type ToolRecord struct {
	Name         string         // Tool name, e.g. "file_read"
	Description  string         // Human-readable description
	Group        string         // Subsystem group name, e.g. "files", "rag"
	InputSchema  map[string]any // JSON Schema from Go struct reflection
	OutputSchema map[string]any // JSON Schema from Go struct reflection
}

// addToolRecorded registers a tool with the MCP server AND records its metadata.
// This is a generic function that captures the In/Out types for schema extraction.
func addToolRecorded[In, Out any](s *Service, server *mcp.Server, group string, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	mcp.AddTool(server, t, h)
	s.tools = append(s.tools, ToolRecord{
		Name:         t.Name,
		Description:  t.Description,
		Group:        group,
		InputSchema:  structSchema(new(In)),
		OutputSchema: structSchema(new(Out)),
	})
}

// structSchema builds a simple JSON Schema from a struct's json tags via reflection.
// Returns nil for non-struct types or empty structs.
func structSchema(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	if t.NumField() == 0 {
		return nil
	}

	properties := make(map[string]any)
	required := make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		name := f.Name
		isOptional := false
		if jsonTag != "" {
			parts := splitTag(jsonTag)
			name = parts[0]
			for _, p := range parts[1:] {
				if p == "omitempty" {
					isOptional = true
				}
			}
		}

		prop := map[string]any{
			"type": goTypeToJSONType(f.Type),
		}
		properties[name] = prop

		if !isOptional {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// splitTag splits a struct tag value by commas.
func splitTag(tag string) []string {
	var parts []string
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] != ',' {
			i++
		}
		parts = append(parts, tag[:i])
		if i < len(tag) {
			tag = tag[i+1:]
		} else {
			break
		}
	}
	return parts
}

// goTypeToJSONType maps Go types to JSON Schema types.
func goTypeToJSONType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}
