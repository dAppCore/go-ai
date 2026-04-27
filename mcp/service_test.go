package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestService_RegisterTool_Good(t *testing.T) {
	s := &Service{tools: map[string]Tool{}}

	err := s.RegisterTool(Tool{
		Name:        "custom_tool",
		Description: "Custom tool",
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			return map[string]bool{"ok": true}, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}
	if got := s.ToolNames(); len(got) != 1 || got[0] != "custom_tool" {
		t.Fatalf("ToolNames() = %v, want [custom_tool]", got)
	}
}

func TestService_RegisterTool_Bad(t *testing.T) {
	s := &Service{tools: map[string]Tool{}}
	if err := s.RegisterTool(Tool{Name: "", Handler: func(context.Context, json.RawMessage) (any, error) { return nil, nil }}); err == nil {
		t.Fatal("expected missing name to fail")
	}
	if err := s.RegisterTool(Tool{Name: "missing_handler"}); err == nil {
		t.Fatal("expected missing handler to fail")
	}
	if err := s.RegisterTool(Tool{Name: "dup", Handler: func(context.Context, json.RawMessage) (any, error) { return nil, nil }}); err != nil {
		t.Fatalf("first duplicate setup failed: %v", err)
	}
	if err := s.RegisterTool(Tool{Name: "dup", Handler: func(context.Context, json.RawMessage) (any, error) { return nil, nil }}); err == nil {
		t.Fatal("expected duplicate registration to fail")
	}
}

func TestService_HandleFrame_Good(t *testing.T) {
	s, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	frame := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lang_detect","arguments":{"path":"main.go"}}}`)
	response, err := s.HandleFrame(context.Background(), frame)
	if err != nil {
		t.Fatalf("HandleFrame failed: %v", err)
	}
	var decoded struct {
		Result struct {
			StructuredContent DetectLanguageOutput `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response, &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded.Result.StructuredContent.Language != "go" {
		t.Fatalf("language = %q, want go", decoded.Result.StructuredContent.Language)
	}
}

func TestService_HandleFrame_Bad(t *testing.T) {
	s, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := s.HandleFrame(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"missing"}`))
	if err == nil {
		t.Fatal("expected missing method to return error")
	}
	var decoded struct {
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal(response, &decoded); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if decoded.Error == nil || decoded.Error.Code != -32601 {
		t.Fatalf("error = %+v, want method-not-found", decoded.Error)
	}
}

func TestServeStdio_Good(t *testing.T) {
	s, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	oldReader, oldWriter := stdioReader, stdioWriter
	defer func() {
		stdioReader, stdioWriter = oldReader, oldWriter
	}()

	var out bytes.Buffer
	stdioReader = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	stdioWriter = &out

	if err := s.ServeStdio(context.Background()); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}
	if !strings.Contains(out.String(), `"tools"`) {
		t.Fatalf("stdio output %q missing tools list", out.String())
	}
}

func TestServeTCP_Good(t *testing.T) {
	s, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	addr := reserveTCPAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ServeTCP(ctx, addr)
	}()
	waitForTCP(t, addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lang_detect","arguments":{"path":"x.py"}}}` + "\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(line, `"language":"python"`) {
		t.Fatalf("response %q missing python language", line)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("ServeTCP returned %v", err)
	}
}

func TestServeUnix_Good(t *testing.T) {
	s, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "mcp.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ServeUnix(ctx, socketPath)
	}()
	waitForUnix(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial unix: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(line, `"file_read"`) {
		t.Fatalf("response %q missing file_read", line)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("ServeUnix returned %v", err)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestService_ToolInventory_Good(t *testing.T) {
	s, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, want := len(s.Tools()), 49; got != want {
		t.Fatalf("tool count = %d, want %d", got, want)
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}
	return addr
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for tcp %s", addr)
}

func waitForUnix(t *testing.T, socketPath string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for unix socket %s", socketPath)
}
