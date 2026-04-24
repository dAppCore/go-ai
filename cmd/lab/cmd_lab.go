//go:build ignore
// +build ignore

package lab

import (
	"context"
	"crypto/subtle"
	"fmt" // Note: intrinsic — this file is //go:build ignore (demo scaffold); stdlib use is intentional for easy copy-paste reference.
	"log/slog"
	"net"
	"net/http"
	"os"        // Note: intrinsic — this file is //go:build ignore (demo scaffold); stdlib use is intentional for easy copy-paste reference.
	"os/signal" // Note: intrinsic — this file is //go:build ignore (demo scaffold); stdlib use is intentional for easy copy-paste reference.
	"strings"   // Note: intrinsic — this file is //go:build ignore (demo scaffold); stdlib use is intentional for easy copy-paste reference.
	"time"

	"forge.lthn.ai/core/cli/pkg/cli"
	"forge.lthn.ai/lthn/lem/pkg/lab"
	"forge.lthn.ai/lthn/lem/pkg/lab/collector"
	"forge.lthn.ai/lthn/lem/pkg/lab/handler"
)

func init() {
	cli.RegisterCommands(AddLabCommands)
}

// LabCommandOptions{Bind: "127.0.0.1:8080", AllowRemote: false} configures `core lab serve`.
type LabCommandOptions struct {
	Bind        string
	AllowRemote bool
}

const defaultLabBindAddr = "127.0.0.1:8080"

// core lab serve --bind :8080
func AddLabCommands(root *cli.Command) {
	if hasCommand(root, "lab") {
		return
	}

	root.AddCommand(newLabCommand())
}

func newLabCommand() *cli.Command {
	options := &LabCommandOptions{
		Bind: defaultLabBindAddr,
	}

	labCmd := &cli.Command{
		Use:   "lab",
		Short: "Homelab monitoring dashboard",
		Long:  "Lab dashboard with real-time monitoring of machines, training runs, models, and services.",
	}

	serveCmd := &cli.Command{
		Use:   "serve",
		Short: "Start the lab dashboard web server",
		Long:  "Starts the lab dashboard HTTP server with live-updating collectors for system stats, Docker, Forgejo, HuggingFace, InfluxDB, and more.",
		RunE: func(cmd *cli.Command, args []string) error {
			return runServe(*options)
		},
	}
	serveCmd.Flags().StringVar(&options.Bind, "bind", options.Bind, "HTTP listen address")
	serveCmd.Flags().BoolVar(&options.AllowRemote, "allow-remote", false, "Allow binding to non-loopback interfaces")

	labCmd.AddCommand(serveCmd)
	return labCmd
}

func hasCommand(parent *cli.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}

func runServe(options LabCommandOptions) error {
	if err := validateLabBindAddress(options.Bind, options.AllowRemote); err != nil {
		return err
	}

	authToken := strings.TrimSpace(os.Getenv("CORE_LAB_API_TOKEN"))
	if err := validateLabRemoteAuth(options.Bind, authToken); err != nil {
		return err
	}

	cfg := lab.LoadConfig()
	cfg.Addr = options.Bind

	store := lab.NewStore()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	reg := collector.NewRegistry(logger)
	reg.Register(collector.NewSystem(cfg, store), 60*time.Second)
	reg.Register(collector.NewPrometheus(cfg.PrometheusURL, store),
		time.Duration(cfg.PrometheusInterval)*time.Second)
	reg.Register(collector.NewHuggingFace(cfg.HFAuthor, store),
		time.Duration(cfg.HFInterval)*time.Second)
	reg.Register(collector.NewDocker(store),
		time.Duration(cfg.DockerInterval)*time.Second)

	if cfg.ForgeToken != "" {
		reg.Register(collector.NewForgejo(cfg.ForgeURL, cfg.ForgeToken, store),
			time.Duration(cfg.ForgeInterval)*time.Second)
	}

	reg.Register(collector.NewTraining(cfg, store),
		time.Duration(cfg.TrainingInterval)*time.Second)
	reg.Register(collector.NewServices(store), 60*time.Second)

	if cfg.InfluxToken != "" {
		reg.Register(collector.NewInfluxDB(cfg, store),
			time.Duration(cfg.InfluxInterval)*time.Second)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	reg.Start(ctx)
	defer reg.Stop()

	web := handler.NewWebHandler(store)
	api := handler.NewAPIHandler(store)
	authWrapper := func(handler http.HandlerFunc) http.HandlerFunc {
		return requireLabAuth(handler, authToken)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", authWrapper(web.Dashboard))
	mux.HandleFunc("GET /models", authWrapper(web.Models))
	mux.HandleFunc("GET /training", authWrapper(web.Training))
	mux.HandleFunc("GET /dataset", authWrapper(web.Dataset))
	mux.HandleFunc("GET /golden-set", authWrapper(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dataset", http.StatusMovedPermanently)
	}))
	mux.HandleFunc("GET /runs", authWrapper(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/training", http.StatusMovedPermanently)
	}))
	mux.HandleFunc("GET /agents", authWrapper(web.Agents))
	mux.HandleFunc("GET /services", authWrapper(web.Services))

	mux.HandleFunc("GET /events", authWrapper(web.Events))

	mux.HandleFunc("GET /api/status", authWrapper(api.Status))
	mux.HandleFunc("GET /api/models", authWrapper(api.Models))
	mux.HandleFunc("GET /api/training", authWrapper(api.Training))
	mux.HandleFunc("GET /api/dataset", authWrapper(api.GoldenSet))
	mux.HandleFunc("GET /api/golden-set", authWrapper(api.GoldenSet))
	mux.HandleFunc("GET /api/runs", authWrapper(api.Runs))
	mux.HandleFunc("GET /api/agents", authWrapper(api.Agents))
	mux.HandleFunc("GET /api/services", authWrapper(api.Services))
	mux.HandleFunc("GET /health", authWrapper(api.Health))

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
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

	logger.Info("lab dashboard starting", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func validateLabBindAddress(addr string, allowRemote bool) error {
	if allowRemote {
		return nil
	}

	if isLoopbackBindAddress(addr) {
		return nil
	}

	return fmt.Errorf("refusing to bind lab dashboard to non-loopback address %q without --allow-remote", addr)
}

func isLoopbackBindAddress(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
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

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		expected := "Bearer " + token
		if len(authHeader) != len(expected) || subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

func validateLabRemoteAuth(bindAddr, authToken string) error {
	if isLoopbackBindAddress(bindAddr) {
		return nil
	}
	if strings.TrimSpace(authToken) != "" {
		return nil
	}
	return fmt.Errorf("refusing to expose lab dashboard on %q without CORE_LAB_API_TOKEN", bindAddr)
}
