// Package main provides the LEM Desktop application.
// A system tray app inspired by BugSETI that bundles:
// - Local Forgejo for agentic git workflows
// - InfluxDB for metrics and coordination
// - Inference proxy to M3 MLX or local vLLM
// - Scoring agent for automated checkpoint evaluation
// - Lab dashboard for training and generation monitoring
//
// Built on Wails v3 — ships as a signed native binary on macOS (Lethean CIC),
// Linux AppImage, and Windows installer.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dappco.re/lthn/lem/cmd/lem-desktop/icons"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend
var assets embed.FS

// Tray icon data — placeholders until real icons are generated.
var (
	trayIconTemplate = icons.Placeholder()
	trayIconLight    = icons.Placeholder()
	trayIconDark     = icons.Placeholder()
)

func main() {
	// Strip embed prefix so files serve from root.
	staticAssets, err := fs.Sub(assets, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	// ── Configuration ──
	influxURL := envOr("INFLUX_URL", "http://localhost:8181")
	influxDB := envOr("INFLUX_DB", "training")
	apiURL := envOr("LEM_API_URL", "http://localhost:8080")
	m3Host := envOr("M3_HOST", "10.69.69.108")
	baseModel := envOr("BASE_MODEL", "deepseek-ai/DeepSeek-R1-Distill-Qwen-7B")
	dbPath := envOr("LEM_DB", "")
	workDir := envOr("WORK_DIR", filepath.Join(os.TempDir(), "scoring-agent"))
	deployDir := envOr("LEM_DEPLOY_DIR", findDeployDir())

	// ── Services ──
	dashboardService := NewDashboardService(influxURL, influxDB, dbPath)
	dockerService := NewDockerService(deployDir)
	agentRunner := NewAgentRunner(apiURL, influxURL, influxDB, m3Host, baseModel, workDir)
	trayService := NewTrayService(nil)

	services := []application.Service{
		application.NewService(dashboardService),
		application.NewService(dockerService),
		application.NewService(agentRunner),
		application.NewService(trayService),
	}

	// ── Application ──
	app := application.New(application.Options{
		Name:        "LEM",
		Description: "Lethean Ethics Model — Training, Scoring & Inference",
		Services:    services,
		Assets: application.AssetOptions{
			Handler: spaHandler(staticAssets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	// Wire up references.
	trayService.app = app
	trayService.SetServices(dashboardService, dockerService, agentRunner)

	// Set up system tray.
	setupSystemTray(app, trayService, dashboardService, dockerService)

	// Show dashboard on first launch.
	app.Event.RegisterApplicationEventHook(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		if w, ok := app.Window.Get("dashboard"); ok {
			w.Show()
			w.Focus()
		}
	})

	log.Println("Starting LEM Desktop...")
	log.Println("  - System tray active")
	log.Println("  - Dashboard ready")
	log.Printf("  - InfluxDB: %s/%s", influxURL, influxDB)
	log.Printf("  - Inference: %s", apiURL)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// spaHandler serves static files with SPA fallback for client-side routing.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(fsys, path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// findDeployDir locates the deploy/ directory relative to the binary.
func findDeployDir() string {
	// Check relative to executable.
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(exe), "deploy")
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
	}
	// Check relative to working directory.
	if cwd, err := os.Getwd(); err == nil {
		dir := filepath.Join(cwd, "deploy")
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
	}
	return "deploy"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
