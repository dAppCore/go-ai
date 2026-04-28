package mcp

import (
	"context"
	"errors"
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
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			fmt.Fprintln(os.Stderr, "MCP Unix listener close error:", err)
		}
		if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "MCP Unix socket cleanup error:", err)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			fmt.Fprintln(os.Stderr, "MCP Unix listener close error:", err)
		}
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
