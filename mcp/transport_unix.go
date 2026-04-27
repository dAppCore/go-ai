package mcp

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// DefaultUnixSocket is used when ServeUnix is called with an empty path.
const DefaultUnixSocket = "/tmp/core-mcp.sock"

// ServeUnix serves newline-delimited MCP JSON-RPC over a Unix domain socket.
func (s *Service) ServeUnix(ctx context.Context, socketPath string) error {
	if socketPath == "" {
		socketPath = DefaultUnixSocket
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return err
	}
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				fmt.Fprintln(os.Stderr, "MCP Unix accept error:", err)
				continue
			}
		}
		go s.serveConn(ctx, conn)
	}
}
