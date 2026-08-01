package mcp

import (
	"context"

	core "dappco.re/go"
)

type rpcRequest struct {
	JSONRPC string
	ID      RawMessage
	HasID   bool
	Method  string
	Params  RawMessage
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type callToolParams struct {
	Name      string
	Arguments RawMessage
}

// HandleFrame handles one newline-delimited JSON-RPC frame.
func (s *Service) HandleFrame(ctx context.Context, frame []byte) core.Result {
	frame = []byte(core.Trim(string(frame)))
	if len(frame) == 0 {
		return core.Ok([]byte(nil))
	}

	reqResult := decodeRPCRequest(frame)
	if !reqResult.OK {
		response := marshalRPCResponse(rpcResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return core.Ok(response)
	}
	req := reqResult.Value.(rpcRequest)

	if req.JSONRPC != "2.0" || req.Method == "" {
		response := s.errorResponse(req.ID, -32600, "invalid request")
		return core.Ok(response)
	}

	result := s.handleMethod(ctx, req)
	if !req.HasID {
		if !result.OK {
			return result
		}
		return core.Ok([]byte(nil))
	}
	if !result.OK {
		err, _ := resultError(result).(error)
		response := s.errorResponse(req.ID, rpcCodeForError(err), err.Error())
		return core.Ok(response)
	}

	return core.Ok(marshalRPCResponse(rpcResponse{
		JSONRPC: "2.0",
		ID:      rawMessageValue(req.ID),
		Result:  result.Value,
	}))
}

func (s *Service) handleMethod(ctx context.Context, req rpcRequest) core.Result {
	switch req.Method {
	case "initialize":
		return core.Ok(map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    serverName,
				"version": serverVersion,
			},
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
		})
	case "notifications/initialized":
		return core.Ok(nil)
	case "ping":
		return core.Ok(map[string]any{})
	case "tools/list":
		return core.Ok(map[string]any{"tools": s.Tools()})
	case "tools/call":
		return s.handleToolCall(ctx, req.Params)
	default:
		return core.Fail(core.Errorf("method not found: %s", req.Method))
	}
}

func (s *Service) handleToolCall(ctx context.Context, raw RawMessage) core.Result {
	raw = RawMessage(core.Trim(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return core.Fail(core.Errorf("%w: missing tools/call params", errInvalidParams))
	}
	paramsResult := decodeCallToolParams(raw)
	if !paramsResult.OK {
		return paramsResult
	}
	params := paramsResult.Value.(callToolParams)
	params.Name = core.Trim(params.Name)
	if params.Name == "" {
		return core.Fail(core.Errorf("%w: tool name is required", errInvalidParams))
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		return core.Fail(core.Errorf("tool not found: %s", params.Name))
	}
	if len(core.Trim(string(params.Arguments))) == 0 {
		params.Arguments = RawMessage("{}")
	}

	outputResult := tool.Handler(ctx, params.Arguments)
	if !outputResult.OK {
		return outputResult
	}

	outputJSON := core.JSONMarshalString(outputResult.Value)
	return core.Ok(map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": string(outputJSON),
		}},
		"structuredContent": outputResult.Value,
		"isError":           false,
	})
}

func (s *Service) errorResponse(id RawMessage, code int, message string) []byte {
	if len(id) == 0 {
		id = RawMessage("null")
	}
	return marshalRPCResponse(rpcResponse{
		JSONRPC: "2.0",
		ID:      rawMessageValue(id),
		Error:   &rpcError{Code: code, Message: message},
	})
}

func decodeRPCRequest(frame []byte) core.Result {
	var fields map[string]any
	if r := core.JSONUnmarshal(frame, &fields); !r.OK {
		return r
	}
	req := rpcRequest{}
	if value, ok := fields["jsonrpc"].(string); ok {
		req.JSONRPC = value
	}
	if value, ok := fields["method"].(string); ok {
		req.Method = value
	}
	if value, ok := fields["id"]; ok {
		req.HasID = true
		if raw := core.JSONMarshal(value); raw.OK {
			req.ID = RawMessage(raw.Value.([]byte))
		} else {
			return raw
		}
	}
	if value, ok := fields["params"]; ok {
		if raw := core.JSONMarshal(value); raw.OK {
			req.Params = RawMessage(raw.Value.([]byte))
		} else {
			return raw
		}
	}
	return core.Ok(req)
}

func decodeCallToolParams(raw RawMessage) core.Result {
	var fields map[string]any
	if r := core.JSONUnmarshal([]byte(raw), &fields); !r.OK {
		return core.Fail(core.Errorf("%w: %s", errInvalidParams, r.Error()))
	}
	params := callToolParams{}
	if value, ok := fields["name"].(string); ok {
		params.Name = value
	}
	if value, ok := fields["arguments"]; ok {
		rawArgs := core.JSONMarshal(value)
		if !rawArgs.OK {
			return rawArgs
		}
		params.Arguments = RawMessage(rawArgs.Value.([]byte))
	}
	return core.Ok(params)
}

func rawMessageValue(raw RawMessage) any {
	raw = RawMessage(core.Trim(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	if r := core.JSONUnmarshal([]byte(raw), &value); r.OK {
		return value
	}
	return nil
}

func resultError(r core.Result) any {
	if err, ok := r.Value.(error); ok {
		return err
	}
	return core.E("mcp.result", r.Error(), nil)
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
			ID:      nil,
			Error:   &rpcError{Code: -32603, Message: "internal error"},
		})
		if !fallback.OK {
			return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`)
		}
		return fallback.Value.([]byte)
	}
	return data.Value.([]byte)
}
