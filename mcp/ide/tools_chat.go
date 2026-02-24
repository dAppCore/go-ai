package ide

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Chat tool input/output types.

// ChatSendInput is the input for ide_chat_send.
type ChatSendInput struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

// ChatSendOutput is the output for ide_chat_send.
type ChatSendOutput struct {
	Sent      bool      `json:"sent"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatHistoryInput is the input for ide_chat_history.
type ChatHistoryInput struct {
	SessionID string `json:"sessionId"`
	Limit     int    `json:"limit,omitempty"`
}

// ChatMessage represents a single message in history.
type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatHistoryOutput is the output for ide_chat_history.
type ChatHistoryOutput struct {
	SessionID string        `json:"sessionId"`
	Messages  []ChatMessage `json:"messages"`
}

// SessionListInput is the input for ide_session_list.
type SessionListInput struct{}

// Session represents an agent session.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// SessionListOutput is the output for ide_session_list.
type SessionListOutput struct {
	Sessions []Session `json:"sessions"`
}

// SessionCreateInput is the input for ide_session_create.
type SessionCreateInput struct {
	Name string `json:"name"`
}

// SessionCreateOutput is the output for ide_session_create.
type SessionCreateOutput struct {
	Session Session `json:"session"`
}

// PlanStatusInput is the input for ide_plan_status.
type PlanStatusInput struct {
	SessionID string `json:"sessionId"`
}

// PlanStep is a single step in an agent plan.
type PlanStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// PlanStatusOutput is the output for ide_plan_status.
type PlanStatusOutput struct {
	SessionID string     `json:"sessionId"`
	Status    string     `json:"status"`
	Steps     []PlanStep `json:"steps"`
}

func (s *Subsystem) registerChatTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ide_chat_send",
		Description: "Send a message to an agent chat session",
	}, s.chatSend)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ide_chat_history",
		Description: "Retrieve message history for a chat session",
	}, s.chatHistory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ide_session_list",
		Description: "List active agent sessions",
	}, s.sessionList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ide_session_create",
		Description: "Create a new agent session",
	}, s.sessionCreate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ide_plan_status",
		Description: "Get the current plan status for a session",
	}, s.planStatus)
}

// chatSend forwards a chat message to the Laravel backend via bridge.
// Stub implementation: delegates to bridge, real response arrives via WebSocket subscription.
func (s *Subsystem) chatSend(_ context.Context, _ *mcp.CallToolRequest, input ChatSendInput) (*mcp.CallToolResult, ChatSendOutput, error) {
	if s.bridge == nil {
		return nil, ChatSendOutput{}, errBridgeNotAvailable
	}
	err := s.bridge.Send(BridgeMessage{
		Type:      "chat_send",
		Channel:   "chat:" + input.SessionID,
		SessionID: input.SessionID,
		Data:      input.Message,
	})
	if err != nil {
		return nil, ChatSendOutput{}, fmt.Errorf("failed to send message: %w", err)
	}
	return nil, ChatSendOutput{
		Sent:      true,
		SessionID: input.SessionID,
		Timestamp: time.Now(),
	}, nil
}

// chatHistory requests message history from the Laravel backend.
// Stub implementation: sends request via bridge, returns empty messages. Real data arrives via WebSocket.
func (s *Subsystem) chatHistory(_ context.Context, _ *mcp.CallToolRequest, input ChatHistoryInput) (*mcp.CallToolResult, ChatHistoryOutput, error) {
	if s.bridge == nil {
		return nil, ChatHistoryOutput{}, errBridgeNotAvailable
	}
	// Request history via bridge; for now return placeholder indicating the
	// request was forwarded. Real data arrives via WebSocket subscription.
	_ = s.bridge.Send(BridgeMessage{
		Type:      "chat_history",
		SessionID: input.SessionID,
		Data:      map[string]any{"limit": input.Limit},
	})
	return nil, ChatHistoryOutput{
		SessionID: input.SessionID,
		Messages:  []ChatMessage{},
	}, nil
}

// sessionList requests the session list from the Laravel backend.
// Stub implementation: sends request via bridge, returns empty sessions. Awaiting Laravel backend.
func (s *Subsystem) sessionList(_ context.Context, _ *mcp.CallToolRequest, _ SessionListInput) (*mcp.CallToolResult, SessionListOutput, error) {
	if s.bridge == nil {
		return nil, SessionListOutput{}, errBridgeNotAvailable
	}
	_ = s.bridge.Send(BridgeMessage{Type: "session_list"})
	return nil, SessionListOutput{Sessions: []Session{}}, nil
}

// sessionCreate requests a new session from the Laravel backend.
// Stub implementation: sends request via bridge, returns placeholder session. Awaiting Laravel backend.
func (s *Subsystem) sessionCreate(_ context.Context, _ *mcp.CallToolRequest, input SessionCreateInput) (*mcp.CallToolResult, SessionCreateOutput, error) {
	if s.bridge == nil {
		return nil, SessionCreateOutput{}, errBridgeNotAvailable
	}
	_ = s.bridge.Send(BridgeMessage{
		Type: "session_create",
		Data: map[string]any{"name": input.Name},
	})
	return nil, SessionCreateOutput{
		Session: Session{
			Name:      input.Name,
			Status:    "creating",
			CreatedAt: time.Now(),
		},
	}, nil
}

// planStatus requests plan status from the Laravel backend.
// Stub implementation: sends request via bridge, returns "unknown" status. Awaiting Laravel backend.
func (s *Subsystem) planStatus(_ context.Context, _ *mcp.CallToolRequest, input PlanStatusInput) (*mcp.CallToolResult, PlanStatusOutput, error) {
	if s.bridge == nil {
		return nil, PlanStatusOutput{}, errBridgeNotAvailable
	}
	_ = s.bridge.Send(BridgeMessage{
		Type:      "plan_status",
		SessionID: input.SessionID,
	})
	return nil, PlanStatusOutput{
		SessionID: input.SessionID,
		Status:    "unknown",
		Steps:     []PlanStep{},
	}, nil
}
