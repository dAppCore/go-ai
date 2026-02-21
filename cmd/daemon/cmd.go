// Package daemon provides the `core daemon` command for running as a background service.
package daemon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"forge.lthn.ai/core/go/pkg/cli"
	"forge.lthn.ai/core/go/pkg/log"
	"forge.lthn.ai/core/go-ai/mcp"
)

func init() {
	cli.RegisterCommands(AddDaemonCommand)
}

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

	daemonCmd := cli.NewGroup(
		"daemon",
		"Manage the core daemon",
		"Manage the core background daemon which provides long-running services.\n\n"+
			"Subcommands:\n"+
			"  start   - Start the daemon in the background\n"+
			"  stop    - Stop the running daemon\n"+
			"  status  - Show daemon status\n"+
			"  run     - Run in foreground (for development/debugging)",
	)

	// Persistent flags inherited by all subcommands
	cli.PersistentStringFlag(daemonCmd, &cfg.MCPTransport, "mcp-transport", "t", cfg.MCPTransport,
		"MCP transport type (stdio, tcp, socket)")
	cli.PersistentStringFlag(daemonCmd, &cfg.MCPAddr, "mcp-addr", "a", cfg.MCPAddr,
		"MCP listen address (e.g., :9100 or /tmp/mcp.sock)")
	cli.PersistentStringFlag(daemonCmd, &cfg.HealthAddr, "health-addr", "", cfg.HealthAddr,
		"Health check endpoint address (empty to disable)")
	cli.PersistentStringFlag(daemonCmd, &cfg.PIDFile, "pid-file", "", cfg.PIDFile,
		"PID file path (empty to disable)")

	// --- Subcommands ---

	startCmd := cli.NewCommand("start", "Start the daemon in the background",
		"Re-executes the core binary as a background daemon process.\n"+
			"The daemon PID is written to the PID file for later management.",
		func(cmd *cli.Command, args []string) error {
			return runStart(cfg)
		},
	)

	stopCmd := cli.NewCommand("stop", "Stop the running daemon",
		"Sends SIGTERM to the daemon process identified by the PID file.\n"+
			"Waits for graceful shutdown before returning.",
		func(cmd *cli.Command, args []string) error {
			return runStop(cfg)
		},
	)

	statusCmd := cli.NewCommand("status", "Show daemon status",
		"Checks if the daemon is running and queries its health endpoint.",
		func(cmd *cli.Command, args []string) error {
			return runStatus(cfg)
		},
	)

	runCmd := cli.NewCommand("run", "Run the daemon in the foreground",
		"Runs the daemon in the current terminal (blocks until SIGINT/SIGTERM).\n"+
			"Useful for development, debugging, or running under a process manager.",
		func(cmd *cli.Command, args []string) error {
			return runForeground(cfg)
		},
	)

	daemonCmd.AddCommand(startCmd, stopCmd, statusCmd, runCmd)
	root.AddCommand(daemonCmd)
}

// runStart re-execs the current binary as a detached daemon process.
func runStart(cfg Config) error {
	// Check if already running
	if pid, running := readPID(cfg.PIDFile); running {
		return fmt.Errorf("daemon already running (PID %d)", pid)
	}

	// Find the current binary
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find executable: %w", err)
	}

	// Build args for the foreground run command
	args := []string{"daemon", "run",
		"--mcp-transport", cfg.MCPTransport,
		"--mcp-addr", cfg.MCPAddr,
		"--health-addr", cfg.HealthAddr,
		"--pid-file", cfg.PIDFile,
	}

	// Launch detached child with CORE_DAEMON=1
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "CORE_DAEMON=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Detach from parent process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	pid := cmd.Process.Pid

	// Release the child process so it runs independently
	_ = cmd.Process.Release()

	// Wait briefly for the health endpoint to come up
	if cfg.HealthAddr != "" {
		ready := waitForHealth(cfg.HealthAddr, 5*time.Second)
		if ready {
			log.Info("Daemon started", "pid", pid, "health", cfg.HealthAddr)
		} else {
			log.Info("Daemon started (health check not yet ready)", "pid", pid)
		}
	} else {
		log.Info("Daemon started", "pid", pid)
	}

	return nil
}

// runStop sends SIGTERM to the daemon process.
func runStop(cfg Config) error {
	pid, running := readPID(cfg.PIDFile)
	if !running {
		log.Info("Daemon is not running")
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	log.Info("Stopping daemon", "pid", pid)
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", pid, err)
	}

	// Wait for the process to exit (poll PID file removal)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, still := readPID(cfg.PIDFile); !still {
			log.Info("Daemon stopped")
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	log.Warn("Daemon did not stop within 30s, sending SIGKILL")
	_ = proc.Signal(syscall.SIGKILL)

	// Clean up stale PID file
	_ = os.Remove(cfg.PIDFile)
	log.Info("Daemon killed")
	return nil
}

// runStatus checks daemon status via PID and health endpoint.
func runStatus(cfg Config) error {
	pid, running := readPID(cfg.PIDFile)
	if !running {
		fmt.Println("Daemon is not running")
		return nil
	}

	fmt.Printf("Daemon is running (PID %d)\n", pid)

	// Query health endpoint if configured
	if cfg.HealthAddr != "" {
		healthURL := fmt.Sprintf("http://%s/health", cfg.HealthAddr)
		resp, err := http.Get(healthURL)
		if err != nil {
			fmt.Printf("Health: unreachable (%v)\n", err)
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Println("Health: ok")
		} else {
			fmt.Printf("Health: unhealthy (HTTP %d)\n", resp.StatusCode)
		}

		// Check readiness
		readyURL := fmt.Sprintf("http://%s/ready", cfg.HealthAddr)
		resp2, err := http.Get(readyURL)
		if err == nil {
			defer resp2.Body.Close()
			if resp2.StatusCode == http.StatusOK {
				fmt.Println("Ready:  yes")
			} else {
				fmt.Println("Ready:  no")
			}
		}
	}

	return nil
}

// runForeground runs the daemon in the current process (blocking).
// This is what `core daemon run` and the detached child process execute.
func runForeground(cfg Config) error {
	os.Setenv("CORE_DAEMON", "1")

	log.Info("Starting daemon",
		"transport", cfg.MCPTransport,
		"addr", cfg.MCPAddr,
		"health", cfg.HealthAddr,
	)

	// Create MCP service
	mcpSvc, err := mcp.New()
	if err != nil {
		return fmt.Errorf("failed to create MCP service: %w", err)
	}

	// Create daemon with health checks
	daemon := cli.NewDaemon(cli.DaemonOptions{
		PIDFile:         cfg.PIDFile,
		HealthAddr:      cfg.HealthAddr,
		ShutdownTimeout: 30,
	})

	// Start daemon (acquires PID, starts health server)
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Mark as ready
	daemon.SetReady(true)

	// Start MCP server in a goroutine
	ctx := cli.Context()
	mcpErr := make(chan error, 1)
	go func() {
		mcpErr <- startMCP(ctx, mcpSvc, cfg)
	}()

	log.Info("Daemon ready",
		"pid", os.Getpid(),
		"health", daemon.HealthAddr(),
		"services", "mcp",
	)

	// Wait for shutdown signal or MCP error
	select {
	case <-ctx.Done():
		log.Info("Shutting down daemon")
	case err := <-mcpErr:
		if err != nil {
			log.Error("MCP server exited", "error", err)
		}
	}

	// Stop the daemon (releases PID, stops health server)
	return daemon.Stop()
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

// --- Helpers ---

// readPID reads the PID file and checks if the process is still running.
func readPID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}

	// Check if process is actually running
	proc, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}

	// Signal 0 tests if the process exists without actually sending a signal
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return pid, false
	}

	return pid, true
}

// waitForHealth polls the health endpoint until it responds or timeout.
func waitForHealth(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://%s/health", addr)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	return false
}
