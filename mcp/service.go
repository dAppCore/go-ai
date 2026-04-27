package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	serverName        = "core-cli"
	serverVersion     = "0.1.0"
	maxMCPMessageSize = 10 * 1024 * 1024
)

var (
	errInvalidRequest = errors.New("invalid JSON-RPC request")
	errInvalidParams  = errors.New("invalid JSON-RPC params")
)

// Option configures a Service before tools are registered.
type Option func(*Service) error

// Options is accepted by New for compatibility with callers that prefer a struct.
type Options struct {
	WorkspaceRoot  string
	Unrestricted   bool
	ProcessService any
	WSHub          any
	Subsystems     []Subsystem
}

// Subsystem registers additional MCP tools at startup.
type Subsystem interface {
	Name() string
	RegisterTools(*Service)
}

// SubsystemWithShutdown extends Subsystem with graceful cleanup.
type SubsystemWithShutdown interface {
	Subsystem
	Shutdown(context.Context) error
}

// ToolHandler receives the raw JSON arguments from tools/call and returns a
// JSON-serialisable structured response.
type ToolHandler func(context.Context, json.RawMessage) (any, error)

// Tool describes one MCP tool.
type Tool struct {
	Name        string
	Description string
	Group       string
	InputSchema map[string]any
	Handler     ToolHandler
}

// ToolRecord is the public, immutable view of a registered tool.
type ToolRecord struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Group       string         `json:"group,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// Service is the central MCP server state.
type Service struct {
	workspaceRoot string
	tools         map[string]Tool
	toolOrder     []string
	subsystems    []Subsystem

	processMu      sync.Mutex
	processSeq     atomic.Uint64
	processes      map[string]*managedProcess
	wsMu           sync.Mutex
	wsServer       *http.Server
	wsAddr         string
	webviewMu      sync.Mutex
	webviewState   webviewSession
	startedAt      time.Time
	processService any
	wsHub          any
}

// New constructs a Service and registers the built-in 49-tool inventory.
//
// Supported call forms:
//
//	mcp.New(mcp.WithWorkspaceRoot("/repo"))
//	mcp.New(mcp.Options{WorkspaceRoot: "/repo"})
func New(args ...any) (*Service, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("mcp: get working directory: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("mcp: resolve working directory: %w", err)
	}

	s := &Service{
		workspaceRoot: root,
		tools:         make(map[string]Tool),
		processes:     make(map[string]*managedProcess),
		startedAt:     time.Now(),
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case nil:
			continue
		case Option:
			if err := v(s); err != nil {
				return nil, err
			}
		case Options:
			if err := applyOptionsStruct(s, v); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("mcp: unsupported New option %T", arg)
		}
	}

	if err := s.registerBuiltInTools(); err != nil {
		return nil, err
	}
	for _, sub := range s.subsystems {
		if sub != nil {
			sub.RegisterTools(s)
		}
	}

	return s, nil
}

func applyOptionsStruct(s *Service, opts Options) error {
	if opts.Unrestricted {
		if err := WithWorkspaceRoot("")(s); err != nil {
			return err
		}
	} else if opts.WorkspaceRoot != "" {
		if err := WithWorkspaceRoot(opts.WorkspaceRoot)(s); err != nil {
			return err
		}
	}
	if opts.ProcessService != nil {
		s.processService = opts.ProcessService
	}
	if opts.WSHub != nil {
		s.wsHub = opts.WSHub
	}
	for _, sub := range opts.Subsystems {
		if sub != nil {
			s.subsystems = append(s.subsystems, sub)
		}
	}
	return nil
}

// WithWorkspaceRoot restricts file operations to root. Passing an empty string
// disables sandboxing and lets file tools operate on cleaned OS paths.
func WithWorkspaceRoot(root string) Option {
	return func(s *Service) error {
		if root == "" {
			s.workspaceRoot = ""
			return nil
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("mcp: resolve workspace root: %w", err)
		}
		s.workspaceRoot = abs
		return nil
	}
}

// WithProcessService records an externally supplied process service. The
// in-module process tools still provide a local fallback when this is nil.
func WithProcessService(ps any) Option {
	return func(s *Service) error {
		s.processService = ps
		return nil
	}
}

// WithWSHub records an externally supplied WebSocket hub.
func WithWSHub(hub any) Option {
	return func(s *Service) error {
		s.wsHub = hub
		return nil
	}
}

// WithSubsystem appends a subsystem plugin.
func WithSubsystem(sub Subsystem) Option {
	return func(s *Service) error {
		if sub != nil {
			s.subsystems = append(s.subsystems, sub)
		}
		return nil
	}
}

// WorkspaceRoot returns the configured filesystem sandbox root. An empty value
// means unrestricted filesystem access.
func (s *Service) WorkspaceRoot() string {
	return s.workspaceRoot
}

// Tools returns registered tools in registration order.
func (s *Service) Tools() []ToolRecord {
	records := make([]ToolRecord, 0, len(s.toolOrder))
	for _, name := range s.toolOrder {
		tool := s.tools[name]
		records = append(records, ToolRecord{
			Name:        tool.Name,
			Description: tool.Description,
			Group:       tool.Group,
			InputSchema: cloneStringAnyMap(tool.InputSchema),
		})
	}
	return records
}

// ToolNames returns registered tool names in registration order.
func (s *Service) ToolNames() []string {
	return slices.Clone(s.toolOrder)
}

// RegisterTool adds a tool to the service.
func (s *Service) RegisterTool(tool Tool) error {
	tool.Name = strings.TrimSpace(tool.Name)
	if tool.Name == "" {
		return fmt.Errorf("mcp: tool name is required")
	}
	if tool.Handler == nil {
		return fmt.Errorf("mcp: handler is required for tool %q", tool.Name)
	}
	if _, exists := s.tools[tool.Name]; exists {
		return fmt.Errorf("mcp: tool %q already registered", tool.Name)
	}
	if tool.InputSchema == nil {
		tool.InputSchema = objectSchema()
	}
	s.tools[tool.Name] = tool
	s.toolOrder = append(s.toolOrder, tool.Name)
	return nil
}

// RegisterToolFunc adds a tool with a raw JSON argument handler.
func (s *Service) RegisterToolFunc(group, name, description string, handler ToolHandler) error {
	return s.RegisterTool(Tool{
		Name:        name,
		Description: description,
		Group:       group,
		Handler:     handler,
	})
}

// Shutdown gracefully stops subsystems, local WebSocket serving, and managed processes.
func (s *Service) Shutdown(ctx context.Context) error {
	var errs []error
	for _, sub := range s.subsystems {
		if sh, ok := sub.(SubsystemWithShutdown); ok {
			if err := sh.Shutdown(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}

	s.wsMu.Lock()
	wsServer := s.wsServer
	s.wsMu.Unlock()
	if wsServer != nil {
		if err := wsServer.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	s.processMu.Lock()
	processes := make([]*managedProcess, 0, len(s.processes))
	for _, proc := range s.processes {
		processes = append(processes, proc)
	}
	s.processMu.Unlock()
	for _, proc := range processes {
		if proc.isRunning() && proc.cmd.Process != nil {
			if err := proc.cmd.Process.Kill(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

type typedToolFunc[I any, O any] func(context.Context, I) (O, error)

func typedHandler[I any, O any](fn typedToolFunc[I, O]) ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		var input I
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			raw = []byte("{}")
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidParams, err)
		}
		return fn(ctx, input)
	}
}

func objectSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
	}
}

func cloneStringAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func serveReaderWriter(ctx context.Context, r io.Reader, w io.Writer, handle func(context.Context, []byte) ([]byte, error)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxMCPMessageSize)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		response, _ := handle(ctx, scanner.Bytes())
		if len(response) == 0 {
			continue
		}
		if _, err := w.Write(append(response, '\n')); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
