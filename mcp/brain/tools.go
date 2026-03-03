// SPDX-License-Identifier: EUPL-1.2

package brain

import (
	"context"
	"fmt"
	"time"

	"forge.lthn.ai/core/go-ai/mcp/ide"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// -- Input/Output types -------------------------------------------------------

// RememberInput is the input for brain_remember.
type RememberInput struct {
	Content    string   `json:"content"`
	Type       string   `json:"type"`
	Tags       []string `json:"tags,omitempty"`
	Project    string   `json:"project,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	Supersedes string   `json:"supersedes,omitempty"`
	ExpiresIn  int      `json:"expires_in,omitempty"`
}

// RememberOutput is the output for brain_remember.
type RememberOutput struct {
	Success   bool      `json:"success"`
	MemoryID  string    `json:"memoryId,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// RecallInput is the input for brain_recall.
type RecallInput struct {
	Query string       `json:"query"`
	TopK  int          `json:"top_k,omitempty"`
	Filter RecallFilter `json:"filter,omitempty"`
}

// RecallFilter holds optional filter criteria for brain_recall.
type RecallFilter struct {
	Project       string   `json:"project,omitempty"`
	Type          any      `json:"type,omitempty"`
	AgentID       string   `json:"agent_id,omitempty"`
	MinConfidence float64  `json:"min_confidence,omitempty"`
}

// RecallOutput is the output for brain_recall.
type RecallOutput struct {
	Success  bool     `json:"success"`
	Count    int      `json:"count"`
	Memories []Memory `json:"memories"`
}

// Memory is a single memory entry returned by recall or list.
type Memory struct {
	ID           string   `json:"id"`
	AgentID      string   `json:"agent_id"`
	Type         string   `json:"type"`
	Content      string   `json:"content"`
	Tags         []string `json:"tags,omitempty"`
	Project      string   `json:"project,omitempty"`
	Confidence   float64  `json:"confidence"`
	SupersedesID string   `json:"supersedes_id,omitempty"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// ForgetInput is the input for brain_forget.
type ForgetInput struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// ForgetOutput is the output for brain_forget.
type ForgetOutput struct {
	Success   bool      `json:"success"`
	Forgotten string    `json:"forgotten"`
	Timestamp time.Time `json:"timestamp"`
}

// ListInput is the input for brain_list.
type ListInput struct {
	Project string `json:"project,omitempty"`
	Type    string `json:"type,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// ListOutput is the output for brain_list.
type ListOutput struct {
	Success  bool     `json:"success"`
	Count    int      `json:"count"`
	Memories []Memory `json:"memories"`
}

// -- Tool registration --------------------------------------------------------

func (s *Subsystem) registerBrainTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "brain_remember",
		Description: "Store a memory in the shared OpenBrain knowledge store. Persists decisions, observations, conventions, research, plans, bugs, or architecture knowledge for other agents.",
	}, s.brainRemember)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "brain_recall",
		Description: "Semantic search across the shared OpenBrain knowledge store. Returns memories ranked by similarity to your query, with optional filtering.",
	}, s.brainRecall)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "brain_forget",
		Description: "Remove a memory from the shared OpenBrain knowledge store. Permanently deletes from both database and vector index.",
	}, s.brainForget)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "brain_list",
		Description: "List memories in the shared OpenBrain knowledge store. Supports filtering by project, type, and agent. No vector search -- use brain_recall for semantic queries.",
	}, s.brainList)
}

// -- Tool handlers ------------------------------------------------------------

func (s *Subsystem) brainRemember(_ context.Context, _ *mcp.CallToolRequest, input RememberInput) (*mcp.CallToolResult, RememberOutput, error) {
	if s.bridge == nil {
		return nil, RememberOutput{}, errBridgeNotAvailable
	}

	err := s.bridge.Send(ide.BridgeMessage{
		Type: "brain_remember",
		Data: map[string]any{
			"content":    input.Content,
			"type":       input.Type,
			"tags":       input.Tags,
			"project":    input.Project,
			"confidence": input.Confidence,
			"supersedes": input.Supersedes,
			"expires_in": input.ExpiresIn,
		},
	})
	if err != nil {
		return nil, RememberOutput{}, fmt.Errorf("failed to send brain_remember: %w", err)
	}

	return nil, RememberOutput{
		Success:   true,
		Timestamp: time.Now(),
	}, nil
}

func (s *Subsystem) brainRecall(_ context.Context, _ *mcp.CallToolRequest, input RecallInput) (*mcp.CallToolResult, RecallOutput, error) {
	if s.bridge == nil {
		return nil, RecallOutput{}, errBridgeNotAvailable
	}

	err := s.bridge.Send(ide.BridgeMessage{
		Type: "brain_recall",
		Data: map[string]any{
			"query":  input.Query,
			"top_k":  input.TopK,
			"filter": input.Filter,
		},
	})
	if err != nil {
		return nil, RecallOutput{}, fmt.Errorf("failed to send brain_recall: %w", err)
	}

	return nil, RecallOutput{
		Success:  true,
		Memories: []Memory{},
	}, nil
}

func (s *Subsystem) brainForget(_ context.Context, _ *mcp.CallToolRequest, input ForgetInput) (*mcp.CallToolResult, ForgetOutput, error) {
	if s.bridge == nil {
		return nil, ForgetOutput{}, errBridgeNotAvailable
	}

	err := s.bridge.Send(ide.BridgeMessage{
		Type: "brain_forget",
		Data: map[string]any{
			"id":     input.ID,
			"reason": input.Reason,
		},
	})
	if err != nil {
		return nil, ForgetOutput{}, fmt.Errorf("failed to send brain_forget: %w", err)
	}

	return nil, ForgetOutput{
		Success:   true,
		Forgotten: input.ID,
		Timestamp: time.Now(),
	}, nil
}

func (s *Subsystem) brainList(_ context.Context, _ *mcp.CallToolRequest, input ListInput) (*mcp.CallToolResult, ListOutput, error) {
	if s.bridge == nil {
		return nil, ListOutput{}, errBridgeNotAvailable
	}

	err := s.bridge.Send(ide.BridgeMessage{
		Type: "brain_list",
		Data: map[string]any{
			"project":  input.Project,
			"type":     input.Type,
			"agent_id": input.AgentID,
			"limit":    input.Limit,
		},
	})
	if err != nil {
		return nil, ListOutput{}, fmt.Errorf("failed to send brain_list: %w", err)
	}

	return nil, ListOutput{
		Success:  true,
		Memories: []Memory{},
	}, nil
}
