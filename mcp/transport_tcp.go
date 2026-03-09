package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultTCPAddr is the default address for the MCP TCP server.
const DefaultTCPAddr = "127.0.0.1:9100"

// diagMu protects diagWriter from concurrent access across tests and goroutines.
var diagMu sync.Mutex

// diagWriter is the destination for warning and diagnostic messages.
// Use diagPrintf to write to it safely.
var diagWriter io.Writer = os.Stderr

// diagPrintf writes a formatted message to diagWriter under the mutex.
func diagPrintf(format string, args ...any) {
	diagMu.Lock()
	defer diagMu.Unlock()
	fmt.Fprintf(diagWriter, format, args...)
}

// setDiagWriter swaps the diagnostic writer and returns the previous one.
// Used by tests to capture output without racing.
func setDiagWriter(w io.Writer) io.Writer {
	diagMu.Lock()
	defer diagMu.Unlock()
	old := diagWriter
	diagWriter = w
	return old
}

// maxMCPMessageSize is the maximum size for MCP JSON-RPC messages (10 MB).
const maxMCPMessageSize = 10 * 1024 * 1024

// TCPTransport manages a TCP listener for MCP.
type TCPTransport struct {
	addr     string
	listener net.Listener
}

// NewTCPTransport creates a new TCP transport listener.
// It listens on the provided address (e.g. "localhost:9100").
// Defaults to 127.0.0.1 when the host component is empty (e.g. ":9100").
// Emits a security warning when explicitly binding to 0.0.0.0 (all interfaces).
func NewTCPTransport(addr string) (*TCPTransport, error) {
	host, port, _ := net.SplitHostPort(addr)
	if host == "" {
		addr = net.JoinHostPort("127.0.0.1", port)
	} else if host == "0.0.0.0" {
		diagPrintf("WARNING: MCP TCP server binding to all interfaces (%s). Use 127.0.0.1 for local-only access.\n", addr)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &TCPTransport{addr: addr, listener: listener}, nil
}

// ServeTCP starts a TCP server for the MCP service.
// It accepts connections and spawns a new MCP server session for each connection.
func (s *Service) ServeTCP(ctx context.Context, addr string) error {
	t, err := NewTCPTransport(addr)
	if err != nil {
		return err
	}
	defer func() { _ = t.listener.Close() }()

	// Close listener when context is cancelled to unblock Accept
	go func() {
		<-ctx.Done()
		_ = t.listener.Close()
	}()

	if addr == "" {
		addr = t.listener.Addr().String()
	}
	diagPrintf("MCP TCP server listening on %s\n", addr)

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				diagPrintf("Accept error: %v\n", err)
				continue
			}
		}

		go s.handleConnection(ctx, conn)
	}
}

func (s *Service) handleConnection(ctx context.Context, conn net.Conn) {
	// Note: We don't defer conn.Close() here because it's closed by the Server/Transport

	// Create new server instance for this connection
	impl := &mcp.Implementation{
		Name:    "core-cli",
		Version: "0.1.0",
	}
	server := mcp.NewServer(impl, nil)
	s.registerTools(server)

	// Create transport for this connection
	transport := &connTransport{conn: conn}

	// Run server (blocks until connection closed)
	// Server.Run calls Connect, then Read loop.
	if err := server.Run(ctx, transport); err != nil {
		diagPrintf("Connection error: %v\n", err)
	}
}

// connTransport adapts net.Conn to mcp.Transport
type connTransport struct {
	conn net.Conn
}

func (t *connTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	scanner := bufio.NewScanner(t.conn)
	scanner.Buffer(make([]byte, 64*1024), maxMCPMessageSize)
	return &connConnection{
		conn:    t.conn,
		scanner: scanner,
	}, nil
}

// connConnection implements mcp.Connection
type connConnection struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

func (c *connConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	// Blocks until line is read
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, err
		}
		// EOF - connection closed cleanly
		return nil, io.EOF
	}
	line := c.scanner.Bytes()
	return jsonrpc.DecodeMessage(line)
}

func (c *connConnection) Write(ctx context.Context, msg jsonrpc.Message) error {
	data, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return err
	}
	// Append newline for line-delimited JSON
	data = append(data, '\n')
	_, err = c.conn.Write(data)
	return err
}

func (c *connConnection) Close() error {
	return c.conn.Close()
}

func (c *connConnection) SessionID() string {
	return "tcp-session" // Unique ID might be better, but optional
}
