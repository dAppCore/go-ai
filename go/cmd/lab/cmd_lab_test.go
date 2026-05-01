// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	core "dappco.re/go"
	execabs "golang.org/x/sys/execabs"
)

func TestCmdLab_RunCommand_Good_Help(t *testing.T) {
	stdout := core.NewBuffer()

	if r := runLabCommand([]string{"--help"}, stdout, ioDiscard{}); !r.OK {
		t.Fatalf("runLabCommand(help): %s", r.Error())
	}
	if got := stdout.String(); !core.Contains(got, "go-ai serve") {
		t.Fatalf("expected usage to mention go-ai serve, got %q", got)
	}
}

func TestCmdLab_RunCommand_Bad_UnknownCommand(t *testing.T) {
	r := runLabCommand([]string{"missing"}, ioDiscard{}, ioDiscard{})
	if r.OK {
		t.Fatal("expected unknown command to be rejected")
	}
	if got := r.Error(); !core.Contains(got, "unknown go-ai command") {
		t.Fatalf("expected unknown command error, got %q", got)
	}
}

func TestCmdLab_parseLabServeOptions_Good_Defaults(t *testing.T) {
	result := parseLabServeOptions(nil, ioDiscard{})
	if !result.OK {
		t.Fatalf("parseLabServeOptions(defaults): %s", result.Error())
	}
	options := result.Value.(LabCommandOptions)

	if options.Bind != defaultLabBindAddr {
		t.Fatalf("expected default bind %q, got %q", defaultLabBindAddr, options.Bind)
	}
	if options.AllowRemote {
		t.Fatal("expected allow-remote to default false")
	}
}

func TestCmdLab_parseLabServeOptions_Good_CustomFlags(t *testing.T) {
	result := parseLabServeOptions([]string{"--bind", "127.0.0.1:9090", "--allow-remote"}, ioDiscard{})
	if !result.OK {
		t.Fatalf("parseLabServeOptions(custom): %s", result.Error())
	}
	options := result.Value.(LabCommandOptions)

	if options.Bind != "127.0.0.1:9090" {
		t.Fatalf("expected custom bind, got %q", options.Bind)
	}
	if !options.AllowRemote {
		t.Fatal("expected allow-remote true")
	}
}

func TestCmdLab_Build_Good_ProducesExecutable(t *testing.T) {
	tempDir := t.TempDir()
	exePath := core.PathJoin(tempDir, "lab-build-check")

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate current test file")
	}
	repoRoot := cleanLabTestPath(core.PathJoin(labTestPathDir(currentFile), "..", ".."))

	cmd := execabs.Command("go", "build", "-o", exePath, "./cmd/lab")
	cmd.Dir = repoRoot
	cmd.Env = append(core.Environ(), "GOCACHE="+core.PathJoin(tempDir, "gocache"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/lab: %v\n%s", err, output)
	}

	infoResult := core.Stat(exePath)
	if !infoResult.OK {
		t.Fatalf("stat build output: %v", infoResult.Error())
	}
	info := infoResult.Value.(core.FsFileInfo)
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected build output to be executable, mode %s", info.Mode())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

func labTestPathDir(path string) string {
	sep := byte(core.PathSeparator)
	trimmed := path
	for len(trimmed) > 1 && trimmed[len(trimmed)-1] == sep {
		trimmed = trimmed[:len(trimmed)-1]
	}
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == sep {
			if i == 0 {
				return string(sep)
			}
			return trimmed[:i]
		}
	}
	return "."
}

func cleanLabTestPath(path string) string {
	return core.CleanPath(path, string(core.PathSeparator))
}

func TestCmdLab_Serve_Bad_NonLoopbackWithoutFlag(t *testing.T) {
	t.Setenv("CORE_LAB_API_TOKEN", "expected-token")

	r := runServe(LabCommandOptions{Bind: "0.0.0.0:8080"})
	if r.OK {
		t.Fatal("expected non-loopback bind to be rejected without --allow-remote")
	}
	if got := r.Error(); !core.Contains(got, "non-loopback") || !core.Contains(got, "--allow-remote") {
		t.Fatalf("expected clear non-loopback --allow-remote error, got %q", got)
	}
}

func TestCmdLab_Serve_Bad_AllowRemoteWithoutToken(t *testing.T) {
	t.Setenv("CORE_LAB_API_TOKEN", "")

	r := runServe(LabCommandOptions{Bind: defaultLabBindAddr, AllowRemote: true})
	if r.OK {
		t.Fatal("expected --allow-remote to require CORE_LAB_API_TOKEN")
	}
	if got := r.Error(); !core.Contains(got, "--allow-remote") || !core.Contains(got, "CORE_LAB_API_TOKEN") {
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
		if r := validateLabBindAddress(addr, false); !r.OK {
			t.Fatalf("validateLabBindAddress(%q, false) = %s", addr, r.Error())
		}
	}
}

func TestCmdLab_validateLabBindAddress_Good_AllowRemoteBypassesAddressChecks(t *testing.T) {
	if r := validateLabBindAddress("0.0.0.0:8080", true); !r.OK {
		t.Fatalf("validateLabBindAddress should allow remote when flag enabled: %s", r.Error())
	}
}

func TestCmdLab_validateLabBindAddress_Bad_RejectsRemoteWithoutFlag(t *testing.T) {
	if r := validateLabBindAddress("0.0.0.0:8080", false); r.OK {
		t.Fatal("expected remote address to be rejected without --allow-remote")
	}
	if r := validateLabBindAddress(":8080", false); r.OK {
		t.Fatal("expected wildcard bind to be rejected without --allow-remote")
	}
}

func TestCmdLab_validateLabRemoteAuth_Bad_RejectsAllowRemoteWithoutToken(t *testing.T) {
	if r := validateLabRemoteAuth(true, ""); r.OK {
		t.Fatal("expected --allow-remote to require CORE_LAB_API_TOKEN")
	}
}

func TestCmdLab_validateLabRemoteAuth_Good_AllowsLocalOnlyWithoutToken(t *testing.T) {
	if r := validateLabRemoteAuth(false, ""); !r.OK {
		t.Fatalf("validateLabRemoteAuth(local-only, empty token) = %s", r.Error())
	}
}

func TestCmdLab_validateLabRemoteAuth_Good_AllowsRemoteWithToken(t *testing.T) {
	if r := validateLabRemoteAuth(true, "expected-token"); !r.OK {
		t.Fatalf("validateLabRemoteAuth(remote, token) = %s", r.Error())
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

func TestCmdLab_newLabServeMux_Good_Healthz(t *testing.T) {
	mux := newLabServeMux("")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if got := rr.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", got)
	}
	if got := core.Trim(rr.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("expected healthz JSON, got %q", got)
	}
}

func TestCmdLab_newLabServeMux_Good_HealthAlias(t *testing.T) {
	mux := newLabServeMux("")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

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
