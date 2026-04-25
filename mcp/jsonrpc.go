package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// HandleFrame handles one newline-delimited JSON-RPC frame.
func (s *Service) HandleFrame(ctx context.Context, frame []byte) ([]byte, error) {
	frame = bytes.TrimSpace(frame)
	if len(frame) == 0 {
		return nil, nil
	}

	var req rpcRequest
	if err := json.Unmarshal(frame, &req); err != nil {
		response := marshalRPCResponse(rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return response, err
	}

	if req.JSONRPC != "2.0" || req.Method == "" {
		response := s.errorResponse(req.ID, -32600, "invalid request")
		return response, errInvalidRequest
	}

	result, err := s.handleMethod(ctx, req)
	if len(req.ID) == 0 {
		return nil, err
	}
	if err != nil {
		response := s.errorResponse(req.ID, rpcCodeForError(err), err.Error())
		return response, err
	}

	return marshalRPCResponse(rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}), nil
}

func (s *Service) handleMethod(ctx context.Context, req rpcRequest) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    serverName,
				"version": serverVersion,
			},
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
		}, nil
	case "notifications/initialized":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.Tools()}, nil
	case "tools/call":
		return s.handleToolCall(ctx, req.Params)
	default:
		return nil, fmt.Errorf("method not found: %s", req.Method)
	}
}

func (s *Service) handleToolCall(ctx context.Context, raw json.RawMessage) (any, error) {
	var params callToolParams
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, fmt.Errorf("%w: missing tools/call params", errInvalidParams)
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidParams, err)
	}
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return nil, fmt.Errorf("%w: tool name is required", errInvalidParams)
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", params.Name)
	}
	if len(bytes.TrimSpace(params.Arguments)) == 0 {
		params.Arguments = []byte("{}")
	}

	output, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		return nil, err
	}

	outputJSON, _ := json.Marshal(output)
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": string(outputJSON),
		}},
		"structuredContent": output,
		"isError":           false,
	}, nil
}

func (s *Service) errorResponse(id json.RawMessage, code int, message string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return marshalRPCResponse(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func rpcCodeForError(err error) int {
	if errors.Is(err, errInvalidRequest) {
		return -32600
	}
	if errors.Is(err, errInvalidParams) {
		return -32602
	}
	if strings.HasPrefix(err.Error(), "method not found:") {
		return -32601
	}
	return -32000
}

func marshalRPCResponse(response rpcResponse) []byte {
	data, err := json.Marshal(response)
	if err != nil {
		fallback, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: -32603, Message: "internal error"},
		})
		return fallback
	}
	return data
}
