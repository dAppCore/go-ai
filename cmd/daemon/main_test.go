package main

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDaemon_RunTCP_Good(t *testing.T) {
	addr := reserveDaemonTCPAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithContext(ctx, Config{
			MCPTransport: "tcp",
			MCPTCPAddr:   addr,
			HealthAddr:   "",
			PIDFile:      "",
		})
	}()
	waitForDaemonTCP(t, addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lang_detect","arguments":{"path":"main.go"}}}` + "\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(line, `"language":"go"`) {
		t.Fatalf("response %q missing go language", line)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("daemon returned %v", err)
	}
}

func TestLoadConfig_COREMCPAddrUnixDefaultsAll(t *testing.T) {
	t.Setenv("CORE_MCP_TRANSPORT", "")
	t.Setenv("CORE_MCP_ADDR", "/tmp/ofm-violet-test.sock")
	t.Setenv("CORE_MCP_TCP_ADDR", "127.0.0.1:0")
	t.Setenv("CORE_HEALTH_ADDR", "")
	t.Setenv("CORE_PID_FILE", "")

	cfg := loadConfig()
	if cfg.MCPTransport != "all" {
		t.Fatalf("MCPTransport = %q, want all", cfg.MCPTransport)
	}
	transports := configuredTransports(cfg)
	if strings.Join(transports, ",") != "tcp,socket" {
		t.Fatalf("configuredTransports = %v, want [tcp socket]", transports)
	}
}

func reserveDaemonTCPAddr(t *testing.T) string {
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

func waitForDaemonTCP(t *testing.T, addr string) {
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
