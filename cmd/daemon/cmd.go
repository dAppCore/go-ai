// Package daemon provides the `core daemon` command for running as a background service.
package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"forge.lthn.ai/core/cli/pkg/cli"
	"forge.lthn.ai/core/go-ai/mcp"
	"forge.lthn.ai/core/go-log"
	"forge.lthn.ai/core/go-process"
)

// Transport types for MCP server.
const (
	TransportStdio  = "stdio"
	TransportTCP    = "tcp"
	TransportSocket = "socket"
)

// Config holds daemon configuration.
type Config struct {
	// MCPTransport is the MCP server transport type (stdio, tcp, socket).
	MCPTransport string
	// MCPAddr is the address/path for tcp or socket transports.
	MCPAddr string
	// HealthAddr is the address for health check endpoints.
	HealthAddr string
	// PIDFile is the path for the PID file.
	PIDFile string
}

// DefaultConfig returns the default daemon configuration.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		MCPTransport: TransportTCP,
		MCPAddr:      mcp.DefaultTCPAddr,
		HealthAddr:   "127.0.0.1:9101",
		PIDFile:      filepath.Join(home, ".core", "daemon.pid"),
	}
}

// ConfigFromEnv loads configuration from environment variables.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("CORE_MCP_TRANSPORT"); v != "" {
		cfg.MCPTransport = v
	}
	if v := os.Getenv("CORE_MCP_ADDR"); v != "" {
		cfg.MCPAddr = v
	}
	if v := os.Getenv("CORE_HEALTH_ADDR"); v != "" {
		cfg.HealthAddr = v
	}
	if v := os.Getenv("CORE_PID_FILE"); v != "" {
		cfg.PIDFile = v
	}

	return cfg
}

// AddDaemonCommand adds the 'daemon' command group to the root.
func AddDaemonCommand(root *cli.Command) {
	cfg := ConfigFromEnv()

	cli.AddDaemonCommand(root, cli.DaemonCommandConfig{
		Name:        "daemon",
		Description: "Manage the core daemon",
		PIDFile:     cfg.PIDFile,
		HealthAddr:  cfg.HealthAddr,
		RunForeground: func(ctx context.Context, _ *process.Daemon) error {
			log.Info("Starting MCP service",
				"transport", cfg.MCPTransport,
				"addr", cfg.MCPAddr,
			)
			mcpSvc, err := mcp.New()
			if err != nil {
				return fmt.Errorf("failed to create MCP service: %w", err)
			}
			return startMCP(ctx, mcpSvc, cfg)
		},
		ExtraStartArgs: func() []string {
			return []string{
				"--mcp-transport", cfg.MCPTransport,
				"--mcp-addr", cfg.MCPAddr,
			}
		},
		Flags: func(cmd *cli.Command) {
			cli.PersistentStringFlag(cmd, &cfg.MCPTransport, "mcp-transport", "t", cfg.MCPTransport,
				"MCP transport type (stdio, tcp, socket)")
			cli.PersistentStringFlag(cmd, &cfg.MCPAddr, "mcp-addr", "a", cfg.MCPAddr,
				"MCP listen address (e.g., :9100 or /tmp/mcp.sock)")
		},
	})
}

// startMCP starts the MCP server with the configured transport.
func startMCP(ctx context.Context, svc *mcp.Service, cfg Config) error {
	switch cfg.MCPTransport {
	case TransportStdio:
		log.Info("Starting MCP server", "transport", "stdio")
		return svc.ServeStdio(ctx)

	case TransportTCP:
		log.Info("Starting MCP server", "transport", "tcp", "addr", cfg.MCPAddr)
		return svc.ServeTCP(ctx, cfg.MCPAddr)

	case TransportSocket:
		log.Info("Starting MCP server", "transport", "unix", "path", cfg.MCPAddr)
		return svc.ServeUnix(ctx, cfg.MCPAddr)

	default:
		return fmt.Errorf("unknown MCP transport: %s (valid: stdio, tcp, socket)", cfg.MCPTransport)
	}
}
