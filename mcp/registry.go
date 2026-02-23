// SPDX-License-Identifier: EUPL-1.2

package mcp

import (
	"context"
	"encoding/json"
	"iter"
	"reflect"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RESTHandler handles a tool call from a REST endpoint.
// It receives raw JSON input and returns the typed output or an error.
type RESTHandler func(ctx context.Context, body []byte) (any, error)

// ToolRecord captures metadata about a registered MCP tool.
type ToolRecord struct {
	Name         string         // Tool name, e.g. "file_read"
	Description  string         // Human-readable description
	Group        string         // Subsystem group name, e.g. "files", "rag"
	InputSchema  map[string]any // JSON Schema from Go struct reflection
	OutputSchema map[string]any // JSON Schema from Go struct reflection
	RESTHandler  RESTHandler    // REST-callable handler created at registration time
}

// addToolRecorded registers a tool with the MCP server AND records its metadata.
// This is a generic function that captures the In/Out types for schema extraction.
// It also creates a RESTHandler closure that can unmarshal JSON to the correct
// input type and call the handler directly, enabling the MCP-to-REST bridge.
func addToolRecorded[In, Out any](s *Service, server *mcp.Server, group string, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	mcp.AddTool(server, t, h)

	restHandler := func(ctx context.Context, body []byte) (any, error) {
		var input In
		if len(body) > 0 {
			if err := json.Unmarshal(body, &input); err != nil {
				return nil, err
			}
		}
		// nil: REST callers have no MCP request context.
		// Tool handlers called via REST must not dereference CallToolRequest.
		_, output, err := h(ctx, nil, input)
		return output, err
	}

	s.tools = append(s.tools, ToolRecord{
		Name:         t.Name,
		Description:  t.Description,
		Group:        group,
		InputSchema:  structSchema(new(In)),
		OutputSchema: structSchema(new(Out)),
		RESTHandler:  restHandler,
	})
}

// structSchema builds a simple JSON Schema from a struct's json tags via reflection.
// Returns nil for non-struct types or empty structs.
func structSchema(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	if t.NumField() == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}

	properties := make(map[string]any)
	required := make([]string, 0)

	for f := range t.Fields() {
		f := f
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
	return strings.Split(tag, ",")
}

// splitTagSeq returns an iterator over the tag parts.
func splitTagSeq(tag string) iter.Seq[string] {
	return strings.SplitSeq(tag, ",")
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
