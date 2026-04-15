package security

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withSecurityTempHome(t *testing.T) string {
	t.Helper()

	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", tempHome)
	t.Setenv("HOME", tempHome)
	t.Setenv("DIR_HOME", "")
	return tempHome
}

func withFakeGitHubCLI(t *testing.T) {
	t.Helper()

	withFakeGitHubScript(t, "#!/bin/sh\nexit 0\n")
}

func withFakeGitHubScript(t *testing.T, script string) {
	t.Helper()

	binDir := t.TempDir()
	ghPath := filepath.Join(binDir, "gh")
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}

	path := os.Getenv("PATH")
	if path == "" {
		t.Setenv("PATH", binDir)
		return
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+path)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return buf.String()
}

func stubGitHubAPI(t *testing.T, fn func(endpoint string) ([]byte, error)) {
	t.Helper()

	original := callGitHubAPIRequest
	callGitHubAPIRequest = fn
	t.Cleanup(func() {
		callGitHubAPIRequest = original
	})
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func writeSecurityRegistry(t *testing.T, org string, repoNames ...string) string {
	t.Helper()

	registryDir := t.TempDir()
	registryPath := filepath.Join(registryDir, "repos.yaml")

	var builder strings.Builder
	builder.WriteString("version: 1\n")
	builder.WriteString("org: " + org + "\n")
	builder.WriteString("base_path: " + registryDir + "\n")
	builder.WriteString("repos:\n")
	for _, repoName := range repoNames {
		builder.WriteString("  " + repoName + ":\n")
		builder.WriteString("    type: module\n")
	}

	if err := os.WriteFile(registryPath, []byte(builder.String()), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	return registryPath
}
