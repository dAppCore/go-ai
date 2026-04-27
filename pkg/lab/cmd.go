// SPDX-License-Identifier: EUPL-1.2

// Package lab wires the local lab dashboard command into the core CLI.
package lab

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"os/signal" // Note: retained until lab commands receive a configured core.Signal context.
	"syscall"
	"time"

	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/core"
)

const defaultBindAddr = "127.0.0.1:8080"

// CommandOptions configures `core lab serve`.
type CommandOptions struct {
	Bind        string
	AllowRemote bool
}

func init() {
	cli.RegisterCommands(AddLabCommands)
}

// AddLabCommands registers the top-level lab command group.
func AddLabCommands(root *cli.Command) {
	if commandExists(root, "lab") {
		return
	}

	labCommand := &cli.Command{
		Use:   "lab",
		Short: "Local lab dashboard",
		Long:  "Run local lab dashboard and health endpoints.",
	}

	addServeCommand(labCommand)
	root.AddCommand(labCommand)
}

func addServeCommand(parent *cli.Command) {
	options := &CommandOptions{
		Bind: defaultBindAddr,
	}

	serveCommand := &cli.Command{
		Use:   "serve",
		Short: "Serve the lab dashboard",
		Long:  "Start the local lab dashboard HTTP server.",
		RunE: func(cmd *cli.Command, args []string) error {
			if len(args) > 0 {
				return core.E("lab.serve", core.Sprintf("unexpected argument %q", args[0]), nil)
			}
			return RunServe(*options)
		},
	}

	serveCommand.Flags().StringVar(&options.Bind, "bind", options.Bind, "HTTP listen address")
	serveCommand.Flags().BoolVar(&options.AllowRemote, "allow-remote", false, "Allow binding to non-loopback interfaces")

	parent.AddCommand(serveCommand)
}

// RunServe starts the lab dashboard HTTP server.
func RunServe(options CommandOptions) error {
	if err := ValidateBindAddress(options.Bind, options.AllowRemote); err != nil {
		return err
	}

	authToken := core.Trim(core.Env("CORE_LAB_API_TOKEN"))
	if err := ValidateRemoteAuth(options.AllowRemote, authToken); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:         options.Bind,
		Handler:      newServeMux(authToken),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		core.Info("lab dashboard starting", "addr", options.Bind)
		err := server.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		errc <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errc
	case err := <-errc:
		return err
	}
}

func newServeMux(authToken string) *http.ServeMux {
	authWrapper := func(handler http.HandlerFunc) http.HandlerFunc {
		return requireAuth(handler, authToken)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", authWrapper(index))
	mux.HandleFunc("GET /health", authWrapper(healthz))
	mux.HandleFunc("GET /healthz", authWrapper(healthz))
	return mux
}

func index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("go-ai lab\n"))
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
}

// ValidateBindAddress rejects remote binds unless --allow-remote is set.
func ValidateBindAddress(addr string, allowRemote bool) error {
	if allowRemote || IsLoopbackBindAddress(addr) {
		return nil
	}
	return core.E("lab.serve", core.Sprintf("refusing to bind lab dashboard to non-loopback address %q without --allow-remote", addr), nil)
}

// IsLoopbackBindAddress reports whether addr binds to a loopback host.
func IsLoopbackBindAddress(addr string) bool {
	host, _, err := net.SplitHostPort(core.Trim(addr))
	if err != nil {
		return false
	}

	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func requireAuth(handler http.HandlerFunc, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			handler(w, r)
			return
		}

		authHeader := core.Trim(r.Header.Get("Authorization"))
		expected := core.Concat("Bearer ", token)
		if len(authHeader) != len(expected) || subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

// ValidateRemoteAuth requires CORE_LAB_API_TOKEN before remote access is enabled.
func ValidateRemoteAuth(allowRemote bool, authToken string) error {
	if !allowRemote || core.Trim(authToken) != "" {
		return nil
	}
	return core.E("lab.serve", "refusing to start lab dashboard with --allow-remote without CORE_LAB_API_TOKEN", nil)
}

func commandExists(parent *cli.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}
