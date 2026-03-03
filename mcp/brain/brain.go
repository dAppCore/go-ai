// SPDX-License-Identifier: EUPL-1.2

// Package brain provides an MCP subsystem that proxies OpenBrain knowledge
// store operations to the Laravel php-agentic backend via the IDE bridge.
package brain

import (
	"context"
	"errors"

	"forge.lthn.ai/core/go-ai/mcp/ide"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// errBridgeNotAvailable is returned when a tool requires the Laravel bridge
// but it has not been initialised (headless mode).
var errBridgeNotAvailable = errors.New("brain: bridge not available")

// Subsystem implements mcp.Subsystem for OpenBrain knowledge store operations.
// It proxies brain_* tool calls to the Laravel backend via the shared IDE bridge.
type Subsystem struct {
	bridge *ide.Bridge
}

// New creates a brain subsystem that uses the given IDE bridge for Laravel communication.
// Pass nil if headless (tools will return errBridgeNotAvailable).
func New(bridge *ide.Bridge) *Subsystem {
	return &Subsystem{bridge: bridge}
}

// Name implements mcp.Subsystem.
func (s *Subsystem) Name() string { return "brain" }

// RegisterTools implements mcp.Subsystem.
func (s *Subsystem) RegisterTools(server *mcp.Server) {
	s.registerBrainTools(server)
}

// Shutdown implements mcp.SubsystemWithShutdown.
func (s *Subsystem) Shutdown(_ context.Context) error {
	return nil
}
