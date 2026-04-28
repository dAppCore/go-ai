package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
)

// DefaultTCPAddr is the default TCP MCP listen address.
const DefaultTCPAddr = "127.0.0.1:9100"

// ServeTCP serves newline-delimited MCP JSON-RPC over TCP.
func (s *Service) ServeTCP(ctx context.Context, addr string) error {
	addr = normalizeTCPAddr(addr)
	host, port, err := net.SplitHostPort(addr)
	if err == nil && host == "" {
		addr = net.JoinHostPort("127.0.0.1", port)
	}
	if err == nil && host == "0.0.0.0" {
		fmt.Fprintf(os.Stderr, "WARNING: MCP TCP server binding to all interfaces (%s). Use 127.0.0.1 for local-only access.\n", addr)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			fmt.Fprintln(os.Stderr, "MCP TCP listener close error:", err)
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				fmt.Fprintln(os.Stderr, "MCP TCP accept error:", err)
				continue
			}
		}
		go s.serveConn(ctx, conn)
	}
}

func normalizeTCPAddr(addr string) string {
	if addr == "" {
		return DefaultTCPAddr
	}
	host, port, err := net.SplitHostPort(addr)
	if err == nil && host == "" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}

func (s *Service) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			fmt.Fprintln(os.Stderr, "MCP TCP connection close error:", err)
		}
	}()
	if err := serveReaderWriter(ctx, conn, conn, s.HandleFrame); err != nil && !errors.Is(err, net.ErrClosed) {
		fmt.Fprintln(os.Stderr, "MCP TCP connection error:", err)
	}
}
