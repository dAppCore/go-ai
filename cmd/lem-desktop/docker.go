package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// DockerService manages the LEM Docker compose stack.
// Provides start/stop/status for Forgejo, InfluxDB, and inference services.
type DockerService struct {
	composeFile string
	mu          sync.RWMutex
	services    map[string]ContainerStatus
}

// ContainerStatus represents a Docker container's state.
type ContainerStatus struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Health  string `json:"health"`
	Ports   string `json:"ports"`
	Running bool   `json:"running"`
}

// StackStatus represents the overall stack state.
type StackStatus struct {
	Running    bool                       `json:"running"`
	Services   map[string]ContainerStatus `json:"services"`
	ComposeDir string                     `json:"composeDir"`
}

// NewDockerService creates a DockerService.
// composeDir should point to the deploy/ directory containing docker-compose.yml.
func NewDockerService(composeDir string) *DockerService {
	return &DockerService{
		composeFile: filepath.Join(composeDir, "docker-compose.yml"),
		services:    make(map[string]ContainerStatus),
	}
}

// ServiceName returns the Wails service name.
func (d *DockerService) ServiceName() string {
	return "DockerService"
}

// ServiceStartup is called when the Wails app starts.
func (d *DockerService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	log.Println("DockerService started")
	go d.statusLoop(ctx)
	return nil
}

// Start brings up the full Docker compose stack.
func (d *DockerService) Start() error {
	log.Println("Starting LEM stack...")
	return d.compose("up", "-d")
}

// Stop takes down the Docker compose stack.
func (d *DockerService) Stop() error {
	log.Println("Stopping LEM stack...")
	return d.compose("down")
}

// Restart restarts the full stack.
func (d *DockerService) Restart() error {
	if err := d.Stop(); err != nil {
		return err
	}
	return d.Start()
}

// StartService starts a single service.
func (d *DockerService) StartService(name string) error {
	return d.compose("up", "-d", name)
}

// StopService stops a single service.
func (d *DockerService) StopService(name string) error {
	return d.compose("stop", name)
}

// RestartService restarts a single service.
func (d *DockerService) RestartService(name string) error {
	return d.compose("restart", name)
}

// Logs returns recent logs for a service.
func (d *DockerService) Logs(name string, lines int) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	out, err := d.composeOutput("logs", "--tail", fmt.Sprintf("%d", lines), "--no-color", name)
	if err != nil {
		return "", err
	}
	return out, nil
}

// GetStatus returns the current stack status.
func (d *DockerService) GetStatus() StackStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	running := false
	for _, s := range d.services {
		if s.Running {
			running = true
			break
		}
	}

	return StackStatus{
		Running:    running,
		Services:   d.services,
		ComposeDir: filepath.Dir(d.composeFile),
	}
}

// IsRunning returns whether any services are running.
func (d *DockerService) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, s := range d.services {
		if s.Running {
			return true
		}
	}
	return false
}

// Pull pulls latest images for all services.
func (d *DockerService) Pull() error {
	return d.compose("pull")
}

func (d *DockerService) compose(args ...string) error {
	fullArgs := append([]string{"compose", "-f", d.composeFile}, args...)
	cmd := exec.Command("docker", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose %s: %w: %s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

func (d *DockerService) composeOutput(args ...string) (string, error) {
	fullArgs := append([]string{"compose", "-f", d.composeFile}, args...)
	cmd := exec.Command("docker", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker compose %s: %w: %s", strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

func (d *DockerService) refreshStatus() {
	out, err := d.composeOutput("ps", "--format", "json")
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.services = make(map[string]ContainerStatus)

	// docker compose ps --format json outputs one JSON object per line.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var container struct {
			Name    string `json:"Name"`
			Image   string `json:"Image"`
			Service string `json:"Service"`
			Status  string `json:"Status"`
			Health  string `json:"Health"`
			State   string `json:"State"`
			Ports   string `json:"Ports"`
		}
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}

		name := container.Service
		if name == "" {
			name = container.Name
		}

		d.services[name] = ContainerStatus{
			Name:    container.Name,
			Image:   container.Image,
			Status:  container.Status,
			Health:  container.Health,
			Ports:   container.Ports,
			Running: container.State == "running",
		}
	}
}

func (d *DockerService) statusLoop(ctx context.Context) {
	d.refreshStatus()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.refreshStatus()
		}
	}
}
