//go:build ignore
// +build ignore

package lab

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dappco.re/go/cli/pkg/cli"
)

func TestCmdLab_HasCommand_Good(t *testing.T) {
	root := &cli.Command{Use: "core"}
	root.AddCommand(&cli.Command{Use: "lab"})

	if !hasCommand(root, "lab") {
		t.Fatal("expected hasCommand to detect existing lab command")
	}
	if hasCommand(root, "missing") {
		t.Fatal("expected hasCommand to ignore missing command")
	}
}

func TestCmdLab_AddLabCommands_Good(t *testing.T) {
	root := &cli.Command{Use: "core"}

	AddLabCommands(root)
	AddLabCommands(root)

	commands := root.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected one lab command, got %d", len(commands))
	}
	if commands[0].Name() != "lab" {
		t.Fatalf("expected top-level command lab, got %s", commands[0].Name())
	}

	cmd, _, err := root.Find([]string{"lab", "serve"})
	if err != nil {
		t.Fatalf("find lab serve command: %v", err)
	}
	if cmd.Name() != "serve" {
		t.Fatalf("expected serve subcommand, got %s", cmd.Name())
	}
}

func TestCmdLab_Serve_Bad_NonLoopbackWithoutFlag(t *testing.T) {
	t.Setenv("CORE_LAB_API_TOKEN", "expected-token")

	err := runServe(LabCommandOptions{Bind: "0.0.0.0:8080"})
	if err == nil {
		t.Fatal("expected non-loopback bind to be rejected without --allow-remote")
	}
	if got := err.Error(); !strings.Contains(got, "non-loopback") || !strings.Contains(got, "--allow-remote") {
		t.Fatalf("expected clear non-loopback --allow-remote error, got %q", got)
	}
}

func TestCmdLab_Serve_Bad_AllowRemoteWithoutToken(t *testing.T) {
	t.Setenv("CORE_LAB_API_TOKEN", "")

	err := runServe(LabCommandOptions{Bind: defaultLabBindAddr, AllowRemote: true})
	if err == nil {
		t.Fatal("expected --allow-remote to require CORE_LAB_API_TOKEN")
	}
	if got := err.Error(); !strings.Contains(got, "--allow-remote") || !strings.Contains(got, "CORE_LAB_API_TOKEN") {
		t.Fatalf("expected clear --allow-remote CORE_LAB_API_TOKEN error, got %q", got)
	}
}

func TestCmdLab_validateLabBindAddress_Good_LoopbackAllowed(t *testing.T) {
	tests := []string{
		"127.0.0.1:8080",
		"localhost:8080",
		"[::1]:8080",
	}

	for _, addr := range tests {
		if err := validateLabBindAddress(addr, false); err != nil {
			t.Fatalf("validateLabBindAddress(%q, false) = %v", addr, err)
		}
	}
}

func TestCmdLab_validateLabBindAddress_Good_AllowRemoteBypassesAddressChecks(t *testing.T) {
	if err := validateLabBindAddress("0.0.0.0:8080", true); err != nil {
		t.Fatalf("validateLabBindAddress should allow remote when flag enabled: %v", err)
	}
}

func TestCmdLab_validateLabBindAddress_Bad_RejectsRemoteWithoutFlag(t *testing.T) {
	if err := validateLabBindAddress("0.0.0.0:8080", false); err == nil {
		t.Fatal("expected remote address to be rejected without --allow-remote")
	}
	if err := validateLabBindAddress(":8080", false); err == nil {
		t.Fatal("expected wildcard bind to be rejected without --allow-remote")
	}
}

func TestCmdLab_validateLabRemoteAuth_Bad_RejectsAllowRemoteWithoutToken(t *testing.T) {
	if err := validateLabRemoteAuth(true, ""); err == nil {
		t.Fatal("expected --allow-remote to require CORE_LAB_API_TOKEN")
	}
}

func TestCmdLab_validateLabRemoteAuth_Good_AllowsLocalOnlyWithoutToken(t *testing.T) {
	if err := validateLabRemoteAuth(false, ""); err != nil {
		t.Fatalf("validateLabRemoteAuth(local-only, empty token) = %v", err)
	}
}

func TestCmdLab_validateLabRemoteAuth_Good_AllowsRemoteWithToken(t *testing.T) {
	if err := validateLabRemoteAuth(true, "expected-token"); err != nil {
		t.Fatalf("validateLabRemoteAuth(remote, token) = %v", err)
	}
}

func TestCmdLab_isLoopbackBindAddress_Good(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{name: "localhost", addr: "localhost:8080", want: true},
		{name: "ipv4 loopback", addr: "127.0.0.1:8080", want: true},
		{name: "ipv6 loopback", addr: "[::1]:8080", want: true},
		{name: "wildcard bind", addr: ":8080", want: false},
	}

	for _, tc := range tests {
		if got := isLoopbackBindAddress(tc.addr); got != tc.want {
			t.Fatalf("isLoopbackBindAddress(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestCmdLab_isLoopbackBindAddress_Ugly_InvalidInputsReturnFalse(t *testing.T) {
	tests := []string{
		"",
		"::notanaddr:8080",
		"0.0.0.0:8080",
		"example.com:8080",
	}

	for _, addr := range tests {
		if got := isLoopbackBindAddress(addr); got {
			t.Fatalf("isLoopbackBindAddress(%q) = true, want false", addr)
		}
	}
}

func TestCmdLab_requireLabAuth_Good_AllowWithoutToken(t *testing.T) {
	var called bool
	handler := requireLabAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if !called {
		t.Fatal("wrapped handler was not executed")
	}
	if got := rr.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", got)
	}
}

func TestCmdLab_requireLabAuth_Bad_MissingTokenIsRejected(t *testing.T) {
	var called bool
	handler := requireLabAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, "expected-token")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if called {
		t.Fatal("wrapped handler should not run when authorization is missing")
	}
	if got := rr.Result().StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("expected 401 status, got %d", got)
	}
}

func TestCmdLab_requireLabAuth_Bad_InvalidTokenIsRejected(t *testing.T) {
	var called bool
	handler := requireLabAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, "expected-token")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if called {
		t.Fatal("wrapped handler should not run when authorization is invalid")
	}
	if got := rr.Result().StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("expected 401 status, got %d", got)
	}
}

func TestCmdLab_requireLabAuth_Good_AllowsWhenTokenMatches(t *testing.T) {
	var called bool
	handler := requireLabAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, "expected-token")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer expected-token")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if !called {
		t.Fatal("wrapped handler should run when token is correct")
	}
	if got := rr.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", got)
	}
}
