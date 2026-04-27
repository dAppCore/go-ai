package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"dappco.re/go/ai/mcp"
)

type Config struct {
	MCPTransport string
	MCPAddr      string
	MCPTCPAddr   string
	HealthAddr   string
	PIDFile      string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWithContext(ctx, cfg)
}

func runWithContext(ctx context.Context, cfg Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := writePIDFile(cfg.PIDFile); err != nil {
		return err
	}
	defer func() {
		if cfg.PIDFile != "" {
			_ = os.Remove(cfg.PIDFile)
		}
	}()

	svc, err := mcp.New()
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = svc.Shutdown(shutdownCtx)
	}()

	errCh := make(chan error, 4)
	var wg sync.WaitGroup

	start := func(name string, fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("%s: %w", name, err)
			}
		}()
	}

	for _, transport := range configuredTransports(cfg) {
		switch transport {
		case "stdio":
			start("stdio", svc.ServeStdio)
		case "tcp":
			start("tcp", func(ctx context.Context) error { return svc.ServeTCP(ctx, cfg.MCPTCPAddr) })
		case "socket":
			start("socket", func(ctx context.Context) error { return svc.ServeUnix(ctx, cfg.MCPAddr) })
		}
	}

	healthServer, err := startHealth(ctx, cfg.HealthAddr)
	if err != nil {
		cancel()
		return err
	}
	if healthServer != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = healthServer.Shutdown(shutdownCtx)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		cancel()
		<-done
		return err
	case <-ctx.Done():
		<-done
		return nil
	}
}

func loadConfig() Config {
	transportEnv := os.Getenv("CORE_MCP_TRANSPORT")
	transport := defaultString(transportEnv, "tcp")
	socketPath := os.Getenv("CORE_MCP_ADDR")
	tcpAddr := os.Getenv("CORE_MCP_TCP_ADDR")

	if tcpAddr == "" {
		if looksLikeTCPAddr(socketPath) && (transport == "tcp" || transportEnv == "") {
			tcpAddr = socketPath
		} else {
			tcpAddr = mcp.DefaultTCPAddr
		}
	}
	if socketPath == "" || looksLikeTCPAddr(socketPath) {
		socketPath = mcp.DefaultUnixSocket
	}
	if transportEnv == "" && os.Getenv("CORE_MCP_ADDR") != "" && !looksLikeTCPAddr(os.Getenv("CORE_MCP_ADDR")) {
		transport = "all"
	}

	return Config{
		MCPTransport: transport,
		MCPAddr:      socketPath,
		MCPTCPAddr:   tcpAddr,
		HealthAddr:   defaultString(os.Getenv("CORE_HEALTH_ADDR"), "127.0.0.1:9101"),
		PIDFile:      defaultString(os.Getenv("CORE_PID_FILE"), defaultPIDFile()),
	}
}

func configuredTransports(cfg Config) []string {
	switch strings.ToLower(strings.TrimSpace(cfg.MCPTransport)) {
	case "stdio":
		return []string{"stdio"}
	case "socket", "unix":
		return []string{"socket"}
	case "all", "both":
		return []string{"tcp", "socket"}
	case "tcp", "":
		return []string{"tcp"}
	default:
		return []string{"tcp"}
	}
}

func startHealth(ctx context.Context, addr string) (*http.Server, error) {
	if addr == "" {
		return nil, nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "daemon health server:", err)
		}
	}()
	return server, nil
}

func writePIDFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
}

func defaultPIDFile() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".core", "daemon.pid")
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func looksLikeTCPAddr(value string) bool {
	if value == "" {
		return false
	}
	if _, _, err := net.SplitHostPort(value); err == nil {
		return true
	}
	return strings.HasPrefix(value, ":")
}
