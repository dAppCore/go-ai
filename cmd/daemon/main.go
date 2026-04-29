package main

import (
	"context"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	core "dappco.re/go"
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
	if r := run(); !r.OK {
		core.Print(core.Stderr(), "%s\n", r.Error())
		core.Exit(1)
	}
}

func run() core.Result {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return runWithContext(ctx, cfg)
}

func runWithContext(ctx context.Context, cfg Config) core.Result {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if r := writePIDFile(cfg.PIDFile); !r.OK {
		return r
	}
	defer func() {
		if cfg.PIDFile != "" {
			if r := core.Remove(cfg.PIDFile); !r.OK && !core.IsNotExist(daemonResultError(r)) {
				core.Print(core.Stderr(), "daemon pid cleanup: %s\n", r.Error())
			}
		}
	}()

	svc, err := mcp.New()
	if err != nil {
		return core.Fail(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if r := svc.Shutdown(shutdownCtx); !r.OK {
			core.Print(core.Stderr(), "daemon mcp shutdown: %s\n", r.Error())
		}
	}()

	errCh := make(chan error, 4)
	var wg sync.WaitGroup

	start := func(name string, fn func(context.Context) core.Result) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := fn(ctx); !r.OK && !core.Is(daemonResultError(r), context.Canceled) {
				err := daemonResultError(r)
				errCh <- core.Errorf("%s: %w", name, err)
			}
		}()
	}

	for _, transport := range configuredTransports(cfg) {
		switch transport {
		case "stdio":
			start("stdio", svc.ServeStdio)
		case "tcp":
			start("tcp", func(ctx context.Context) core.Result { return svc.ServeTCP(ctx, cfg.MCPTCPAddr) })
		case "socket":
			start("socket", func(ctx context.Context) core.Result { return svc.ServeUnix(ctx, cfg.MCPAddr) })
		}
	}

	healthServer, err := startHealth(ctx, cfg.HealthAddr)
	if err != nil {
		cancel()
		return core.Fail(err)
	}
	if healthServer != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := healthServer.Shutdown(shutdownCtx); err != nil && !core.Is(err, http.ErrServerClosed) {
				core.Print(core.Stderr(), "daemon health shutdown: %v\n", err)
			}
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
		return core.Fail(err)
	case <-ctx.Done():
		<-done
		return core.Ok(nil)
	}
}

func loadConfig() Config {
	transportEnv := core.Getenv("CORE_MCP_TRANSPORT")
	transport := defaultString(transportEnv, "tcp")
	socketPath := core.Getenv("CORE_MCP_ADDR")
	tcpAddr := core.Getenv("CORE_MCP_TCP_ADDR")

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
	mcpAddr := core.Getenv("CORE_MCP_ADDR")
	if transportEnv == "" && mcpAddr != "" && !looksLikeTCPAddr(mcpAddr) {
		transport = "all"
	}

	return Config{
		MCPTransport: transport,
		MCPAddr:      socketPath,
		MCPTCPAddr:   tcpAddr,
		HealthAddr:   defaultString(core.Getenv("CORE_HEALTH_ADDR"), "127.0.0.1:9101"),
		PIDFile:      defaultString(core.Getenv("CORE_PID_FILE"), defaultPIDFile()),
	}
}

func configuredTransports(cfg Config) []string {
	switch core.Lower(core.Trim(cfg.MCPTransport)) {
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
		core.WriteString(w, `{"ok":true}`)
	})
	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.Background()); err != nil && !core.Is(err, http.ErrServerClosed) {
			core.Print(core.Stderr(), "daemon health shutdown: %v\n", err)
		}
	}()
	go func() {
		if err := server.Serve(listener); err != nil && !core.Is(err, http.ErrServerClosed) {
			core.Print(core.Stderr(), "daemon health server: %v\n", err)
		}
	}()
	return server, nil
}

func writePIDFile(path string) core.Result {
	if path == "" {
		return core.Ok(nil)
	}
	if r := core.MkdirAll(daemonPathDir(path), 0o755); !r.OK {
		return r
	}
	if r := core.WriteFile(path, []byte(core.Sprintf("%d\n", core.Getpid())), 0o644); !r.OK {
		return r
	}
	return core.Ok(nil)
}

func defaultPIDFile() string {
	home := core.UserHomeDir()
	if !home.OK || home.Value.(string) == "" {
		return ""
	}
	return core.PathJoin(home.Value.(string), ".core", "daemon.pid")
}

func defaultString(value, fallback string) string {
	if core.Trim(value) == "" {
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
	return core.HasPrefix(value, ":")
}

func daemonResultError(r core.Result) (err error) {
	if err, ok := r.Value.(error); ok {
		return err
	}
	return core.NewError(r.Error())
}

func daemonPathDir(path string) string {
	sep := byte(core.PathSeparator)
	trimmed := path
	for len(trimmed) > 1 && trimmed[len(trimmed)-1] == sep {
		trimmed = trimmed[:len(trimmed)-1]
	}
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == sep {
			if i == 0 {
				return string(sep)
			}
			return trimmed[:i]
		}
	}
	return "."
}
