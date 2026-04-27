package mcp

import (
	"context"
	"io"
	"os"
)

var (
	stdioReader io.Reader = os.Stdin
	stdioWriter io.Writer = os.Stdout
)

// ServeStdio serves newline-delimited MCP JSON-RPC over stdin/stdout.
func (s *Service) ServeStdio(ctx context.Context) error {
	return serveReaderWriter(ctx, stdioReader, stdioWriter, s.HandleFrame)
}

// Run starts the transport selected by MCP_UNIX_SOCKET or MCP_ADDR. With no
// environment configured it serves stdio.
func (s *Service) Run(ctx context.Context) error {
	if socketPath := os.Getenv("MCP_UNIX_SOCKET"); socketPath != "" {
		return s.ServeUnix(ctx, socketPath)
	}
	if addr := os.Getenv("MCP_ADDR"); addr != "" {
		return s.ServeTCP(ctx, addr)
	}
	return s.ServeStdio(ctx)
}
