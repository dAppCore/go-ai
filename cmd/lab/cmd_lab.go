// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	core "dappco.re/go"
)

func main() {
	if r := runLabCommand(core.Args()[1:], core.Stdout(), core.Stderr()); !r.OK {
		core.Print(core.Stderr(), "%s\n", r.Error())
		core.Exit(2)
	}
}

// LabCommandOptions{Bind: "127.0.0.1:8080", AllowRemote: false} configures `go-ai serve`.
type LabCommandOptions struct {
	Bind        string
	AllowRemote bool
}

const defaultLabBindAddr = "127.0.0.1:8080"

func runLabCommand(args []string, stdout io.Writer, stderr io.Writer) core.Result {
	if len(args) == 0 {
		printLabUsage(stdout)
		return core.Ok(nil)
	}

	switch args[0] {
	case "help", "-h", "--help":
		printLabUsage(stdout)
		return core.Ok(nil)
	case "serve":
		options, err := parseLabServeOptions(args[1:], stderr)
		if err != nil {
			return core.Fail(err)
		}
		return runServe(options)
	default:
		return core.Fail(core.Errorf("unknown go-ai command %q", args[0]))
	}
}

func parseLabServeOptions(args []string, output io.Writer) (LabCommandOptions, error) {
	options := LabCommandOptions{
		Bind: defaultLabBindAddr,
	}
	if output == nil {
		output = io.Discard
	}

	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&options.Bind, "bind", options.Bind, "HTTP listen address")
	flags.BoolVar(&options.AllowRemote, "allow-remote", false, "Allow binding to non-loopback interfaces")

	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if flags.NArg() > 0 {
		return options, core.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return options, nil
}

func printLabUsage(output io.Writer) {
	if output == nil {
		return
	}
	core.WriteString(output, "Usage: go-ai serve [--bind address] [--allow-remote]\n")
}

func runServe(options LabCommandOptions) core.Result {
	if r := validateLabBindAddress(options.Bind, options.AllowRemote); !r.OK {
		return r
	}

	authToken := core.Trim(core.Getenv("CORE_LAB_API_TOKEN"))
	if r := validateLabRemoteAuth(options.AllowRemote, authToken); !r.OK {
		return r
	}

	logger := slog.New(slog.NewJSONHandler(core.Stdout(), nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer cancel()

	srv := &http.Server{
		Addr:         options.Bind,
		Handler:      newLabServeMux(authToken),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		srv.Shutdown(shutCtx)
	}()

	logger.Info("lab dashboard starting", "addr", options.Bind)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return core.Fail(err)
	}
	return core.Ok(nil)
}

func newLabServeMux(authToken string) *http.ServeMux {
	authWrapper := func(handler http.HandlerFunc) http.HandlerFunc {
		return requireLabAuth(handler, authToken)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", authWrapper(labIndex))
	mux.HandleFunc("GET /health", authWrapper(labHealthz))
	mux.HandleFunc("GET /healthz", authWrapper(labHealthz))
	return mux
}

func labIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("go-ai lab\n"))
}

func labHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
}

func validateLabBindAddress(addr string, allowRemote bool) core.Result {
	if allowRemote {
		return core.Ok(nil)
	}

	if isLoopbackBindAddress(addr) {
		return core.Ok(nil)
	}

	return core.Fail(core.Errorf("refusing to bind lab dashboard to non-loopback address %q without --allow-remote", addr))
}

func isLoopbackBindAddress(addr string) bool {
	host, _, err := net.SplitHostPort(core.Trim(addr))
	if err != nil {
		if err.Error() == "missing port in address" {
			return false
		} else {
			return false
		}
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

func requireLabAuth(handler http.HandlerFunc, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			handler(w, r)
			return
		}

		authHeader := core.Trim(r.Header.Get("Authorization"))
		expected := "Bearer " + token
		if len(authHeader) != len(expected) || subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

func validateLabRemoteAuth(allowRemote bool, authToken string) core.Result {
	if !allowRemote {
		return core.Ok(nil)
	}
	if core.Trim(authToken) != "" {
		return core.Ok(nil)
	}
	return core.Fail(core.Errorf("refusing to start lab dashboard with --allow-remote without CORE_LAB_API_TOKEN"))
}
