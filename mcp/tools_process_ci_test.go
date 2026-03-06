package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"forge.lthn.ai/core/go/pkg/core"
	"forge.lthn.ai/core/go-process"
)

// newTestProcessService creates a real process.Service backed by a core.Core for CI tests.
func newTestProcessService(t *testing.T) *process.Service {
	t.Helper()
	c, err := core.New(
		core.WithName("process", process.NewService(process.Options{})),
	)
	if err != nil {
		t.Fatalf("Failed to create framework core: %v", err)
	}
	svc, err := core.ServiceFor[*process.Service](c, "process")
	if err != nil {
		t.Fatalf("Failed to get process service: %v", err)
	}
	// Start services (calls OnStartup)
	if err := c.ServiceStartup(context.Background(), nil); err != nil {
		t.Fatalf("Failed to start core: %v", err)
	}
	t.Cleanup(func() {
		_ = c.ServiceShutdown(context.Background())
	})
	return svc
}

// newTestMCPWithProcess creates an MCP Service wired to a real process.Service.
func newTestMCPWithProcess(t *testing.T) (*Service, *process.Service) {
	t.Helper()
	ps := newTestProcessService(t)
	s, err := New(WithProcessService(ps))
	if err != nil {
		t.Fatalf("Failed to create MCP service: %v", err)
	}
	return s, ps
}

// --- CI-safe handler tests ---

// TestProcessStart_Good_Echo starts "echo hello" and verifies the output.
func TestProcessStart_Good_Echo(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, out, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "echo",
		Args:    []string{"hello"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}
	if out.ID == "" {
		t.Error("Expected non-empty process ID")
	}
	if out.Command != "echo" {
		t.Errorf("Expected command 'echo', got %q", out.Command)
	}
	if out.PID <= 0 {
		t.Errorf("Expected positive PID, got %d", out.PID)
	}
	if out.StartedAt.IsZero() {
		t.Error("Expected non-zero StartedAt")
	}
}

// TestProcessStart_Bad_EmptyCommand verifies empty command returns an error.
func TestProcessStart_Bad_EmptyCommand(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processStart(ctx, nil, ProcessStartInput{})
	if err == nil {
		t.Fatal("Expected error for empty command")
	}
	if !strings.Contains(err.Error(), "command cannot be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestProcessStart_Bad_NonexistentCommand verifies an invalid command returns an error.
func TestProcessStart_Bad_NonexistentCommand(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "/nonexistent/binary/that/does/not/exist",
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent command")
	}
}

// TestProcessList_Good_Empty verifies list is empty initially.
func TestProcessList_Good_Empty(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, out, err := s.processList(ctx, nil, ProcessListInput{})
	if err != nil {
		t.Fatalf("processList failed: %v", err)
	}
	if out.Total != 0 {
		t.Errorf("Expected 0 processes, got %d", out.Total)
	}
}

// TestProcessList_Good_AfterStart verifies a started process appears in list.
func TestProcessList_Good_AfterStart(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	// Start a short-lived process
	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "echo",
		Args:    []string{"listing"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	// Give it a moment to register
	time.Sleep(50 * time.Millisecond)

	// List all processes (including exited)
	_, listOut, err := s.processList(ctx, nil, ProcessListInput{})
	if err != nil {
		t.Fatalf("processList failed: %v", err)
	}
	if listOut.Total < 1 {
		t.Fatalf("Expected at least 1 process, got %d", listOut.Total)
	}

	found := false
	for _, p := range listOut.Processes {
		if p.ID == startOut.ID {
			found = true
			if p.Command != "echo" {
				t.Errorf("Expected command 'echo', got %q", p.Command)
			}
		}
	}
	if !found {
		t.Errorf("Process %s not found in list", startOut.ID)
	}
}

// TestProcessList_Good_RunningOnly verifies filtering for running-only processes.
func TestProcessList_Good_RunningOnly(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	// Start a process that exits quickly
	_, _, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "echo",
		Args:    []string{"done"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	// Wait for it to exit
	time.Sleep(100 * time.Millisecond)

	// Running-only should be empty now
	_, listOut, err := s.processList(ctx, nil, ProcessListInput{RunningOnly: true})
	if err != nil {
		t.Fatalf("processList failed: %v", err)
	}
	if listOut.Total != 0 {
		t.Errorf("Expected 0 running processes after echo exits, got %d", listOut.Total)
	}
}

// TestProcessOutput_Good_Echo verifies output capture from echo.
func TestProcessOutput_Good_Echo(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "echo",
		Args:    []string{"output_test"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	// Wait for process to complete and output to be captured
	time.Sleep(200 * time.Millisecond)

	_, outputOut, err := s.processOutput(ctx, nil, ProcessOutputInput{ID: startOut.ID})
	if err != nil {
		t.Fatalf("processOutput failed: %v", err)
	}
	if !strings.Contains(outputOut.Output, "output_test") {
		t.Errorf("Expected output to contain 'output_test', got %q", outputOut.Output)
	}
}

// TestProcessOutput_Bad_EmptyID verifies empty ID returns error.
func TestProcessOutput_Bad_EmptyID(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processOutput(ctx, nil, ProcessOutputInput{})
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "id cannot be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestProcessOutput_Bad_NotFound verifies nonexistent ID returns error.
func TestProcessOutput_Bad_NotFound(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processOutput(ctx, nil, ProcessOutputInput{ID: "nonexistent-id"})
	if err == nil {
		t.Fatal("Expected error for nonexistent ID")
	}
}

// TestProcessStop_Good_LongRunning starts a sleep, stops it, and verifies.
func TestProcessStop_Good_LongRunning(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	// Start a process that sleeps for a while
	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "sleep",
		Args:    []string{"10"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	// Verify it's running
	time.Sleep(50 * time.Millisecond)
	_, listOut, _ := s.processList(ctx, nil, ProcessListInput{RunningOnly: true})
	if listOut.Total < 1 {
		t.Fatal("Expected at least 1 running process")
	}

	// Stop it
	_, stopOut, err := s.processStop(ctx, nil, ProcessStopInput{ID: startOut.ID})
	if err != nil {
		t.Fatalf("processStop failed: %v", err)
	}
	if !stopOut.Success {
		t.Error("Expected stop to succeed")
	}
	if stopOut.ID != startOut.ID {
		t.Errorf("Expected ID %q, got %q", startOut.ID, stopOut.ID)
	}
}

// TestProcessStop_Bad_EmptyID verifies empty ID returns error.
func TestProcessStop_Bad_EmptyID(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processStop(ctx, nil, ProcessStopInput{})
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
}

// TestProcessStop_Bad_NotFound verifies nonexistent ID returns error.
func TestProcessStop_Bad_NotFound(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processStop(ctx, nil, ProcessStopInput{ID: "nonexistent-id"})
	if err == nil {
		t.Fatal("Expected error for nonexistent ID")
	}
}

// TestProcessKill_Good_LongRunning starts a sleep, kills it, and verifies.
func TestProcessKill_Good_LongRunning(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "sleep",
		Args:    []string{"10"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, killOut, err := s.processKill(ctx, nil, ProcessKillInput{ID: startOut.ID})
	if err != nil {
		t.Fatalf("processKill failed: %v", err)
	}
	if !killOut.Success {
		t.Error("Expected kill to succeed")
	}
	if killOut.Message != "Process killed" {
		t.Errorf("Expected message 'Process killed', got %q", killOut.Message)
	}
}

// TestProcessKill_Bad_EmptyID verifies empty ID returns error.
func TestProcessKill_Bad_EmptyID(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processKill(ctx, nil, ProcessKillInput{})
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
}

// TestProcessKill_Bad_NotFound verifies nonexistent ID returns error.
func TestProcessKill_Bad_NotFound(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processKill(ctx, nil, ProcessKillInput{ID: "nonexistent-id"})
	if err == nil {
		t.Fatal("Expected error for nonexistent ID")
	}
}

// TestProcessInput_Bad_EmptyID verifies empty ID returns error.
func TestProcessInput_Bad_EmptyID(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processInput(ctx, nil, ProcessInputInput{})
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
}

// TestProcessInput_Bad_EmptyInput verifies empty input string returns error.
func TestProcessInput_Bad_EmptyInput(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processInput(ctx, nil, ProcessInputInput{ID: "some-id"})
	if err == nil {
		t.Fatal("Expected error for empty input")
	}
}

// TestProcessInput_Bad_NotFound verifies nonexistent process ID returns error.
func TestProcessInput_Bad_NotFound(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, _, err := s.processInput(ctx, nil, ProcessInputInput{
		ID:    "nonexistent-id",
		Input: "hello\n",
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent ID")
	}
}

// TestProcessInput_Good_Cat sends input to cat and reads it back.
func TestProcessInput_Good_Cat(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	// Start cat which reads stdin and echoes to stdout
	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "cat",
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send input
	_, inputOut, err := s.processInput(ctx, nil, ProcessInputInput{
		ID:    startOut.ID,
		Input: "stdin_test\n",
	})
	if err != nil {
		t.Fatalf("processInput failed: %v", err)
	}
	if !inputOut.Success {
		t.Error("Expected input to succeed")
	}

	// Wait for output capture
	time.Sleep(100 * time.Millisecond)

	// Read output
	_, outputOut, err := s.processOutput(ctx, nil, ProcessOutputInput{ID: startOut.ID})
	if err != nil {
		t.Fatalf("processOutput failed: %v", err)
	}
	if !strings.Contains(outputOut.Output, "stdin_test") {
		t.Errorf("Expected output to contain 'stdin_test', got %q", outputOut.Output)
	}

	// Kill the cat process (it's still running)
	_, _, _ = s.processKill(ctx, nil, ProcessKillInput{ID: startOut.ID})
}

// TestProcessStart_Good_WithDir verifies working directory is respected.
func TestProcessStart_Good_WithDir(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()
	dir := t.TempDir()

	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "pwd",
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	_, outputOut, err := s.processOutput(ctx, nil, ProcessOutputInput{ID: startOut.ID})
	if err != nil {
		t.Fatalf("processOutput failed: %v", err)
	}
	if !strings.Contains(outputOut.Output, dir) {
		t.Errorf("Expected output to contain dir %q, got %q", dir, outputOut.Output)
	}
}

// TestProcessStart_Good_WithEnv verifies environment variables are passed.
func TestProcessStart_Good_WithEnv(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "env",
		Env:     []string{"TEST_MCP_VAR=hello_from_test"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	_, outputOut, err := s.processOutput(ctx, nil, ProcessOutputInput{ID: startOut.ID})
	if err != nil {
		t.Fatalf("processOutput failed: %v", err)
	}
	if !strings.Contains(outputOut.Output, "TEST_MCP_VAR=hello_from_test") {
		t.Errorf("Expected output to contain env var, got %q", outputOut.Output)
	}
}

// TestProcessToolsRegistered_Good_WithService verifies tools are registered when service is provided.
func TestProcessToolsRegistered_Good_WithService(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	if s.processService == nil {
		t.Error("Expected process service to be set")
	}
}

// TestProcessFullLifecycle_Good tests the start → list → output → kill → list cycle.
func TestProcessFullLifecycle_Good(t *testing.T) {
	s, _ := newTestMCPWithProcess(t)
	ctx := context.Background()

	// 1. Start
	_, startOut, err := s.processStart(ctx, nil, ProcessStartInput{
		Command: "sleep",
		Args:    []string{"10"},
	})
	if err != nil {
		t.Fatalf("processStart failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 2. List (should be running)
	_, listOut, _ := s.processList(ctx, nil, ProcessListInput{RunningOnly: true})
	if listOut.Total < 1 {
		t.Fatal("Expected at least 1 running process")
	}

	// 3. Kill
	_, killOut, err := s.processKill(ctx, nil, ProcessKillInput{ID: startOut.ID})
	if err != nil {
		t.Fatalf("processKill failed: %v", err)
	}
	if !killOut.Success {
		t.Error("Expected kill to succeed")
	}

	// 4. Wait for exit
	time.Sleep(100 * time.Millisecond)

	// 5. Should not be running anymore
	_, listOut, _ = s.processList(ctx, nil, ProcessListInput{RunningOnly: true})
	for _, p := range listOut.Processes {
		if p.ID == startOut.ID {
			t.Errorf("Process %s should not be running after kill", startOut.ID)
		}
	}
}
