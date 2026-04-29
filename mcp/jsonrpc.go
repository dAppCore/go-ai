package mcp

import (
	"context"

	core "dappco.re/go"
)

type rpcRequest struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      RawMessage `json:"id,omitempty"`
	Method  string     `json:"method"`
	Params  RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      RawMessage `json:"id"`
	Result  any        `json:"result,omitempty"`
	Error   *rpcError  `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type callToolParams struct {
	Name      string     `json:"name"`
	Arguments RawMessage `json:"arguments,omitempty"`
}

// HandleFrame handles one newline-delimited JSON-RPC frame.
func (s *Service) HandleFrame(ctx context.Context, frame []byte) ([]byte, error) {
	frame = []byte(core.Trim(string(frame)))
	if len(frame) == 0 {
		return nil, nil
	}

	var req rpcRequest
	if r := core.JSONUnmarshal(frame, &req); !r.OK {
		response := marshalRPCResponse(rpcResponse{
			JSONRPC: "2.0",
			ID:      RawMessage("null"),
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return response, core.NewError(r.Error())
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
		return nil, core.Errorf("method not found: %s", req.Method)
	}
}

func (s *Service) handleToolCall(ctx context.Context, raw RawMessage) (any, error) {
	var params callToolParams
	raw = RawMessage(core.Trim(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, core.Errorf("%w: missing tools/call params", errInvalidParams)
	}
	if r := core.JSONUnmarshal([]byte(raw), &params); !r.OK {
		return nil, core.Errorf("%w: %s", errInvalidParams, r.Error())
	}
	params.Name = core.Trim(params.Name)
	if params.Name == "" {
		return nil, core.Errorf("%w: tool name is required", errInvalidParams)
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		return nil, core.Errorf("tool not found: %s", params.Name)
	}
	if len(core.Trim(string(params.Arguments))) == 0 {
		params.Arguments = RawMessage("{}")
	}

	output, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		return nil, err
	}

	outputJSON := core.JSONMarshalString(output)
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": string(outputJSON),
		}},
		"structuredContent": output,
		"isError":           false,
	}, nil
}

func (s *Service) errorResponse(id RawMessage, code int, message string) []byte {
	if len(id) == 0 {
		id = RawMessage("null")
	}
	return marshalRPCResponse(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func rpcCodeForError(err error) int {
	if core.Is(err, errInvalidRequest) {
		return -32600
	}
	if core.Is(err, errInvalidParams) {
		return -32602
	}
	if core.HasPrefix(err.Error(), "method not found:") {
		return -32601
	}
	return -32000
}

func marshalRPCResponse(response rpcResponse) []byte {
	data := core.JSONMarshal(response)
	if !data.OK {
		fallback := core.JSONMarshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      RawMessage("null"),
			Error:   &rpcError{Code: -32603, Message: "internal error"},
		})
		if !fallback.OK {
			return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`)
		}
		return fallback.Value.([]byte)
	}
	return data.Value.([]byte)
}
