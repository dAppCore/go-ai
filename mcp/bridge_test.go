// SPDX-License-Identifier: EUPL-1.2

package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	api "forge.lthn.ai/core/go-api"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestBridgeToAPI_Good_AllTools(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	bridge := api.NewToolBridge("/tools")
	BridgeToAPI(svc, bridge)

	svcCount := len(svc.Tools())
	bridgeCount := len(bridge.Tools())

	if svcCount == 0 {
		t.Fatal("expected non-zero tool count from service")
	}
	if bridgeCount != svcCount {
		t.Fatalf("bridge tool count %d != service tool count %d", bridgeCount, svcCount)
	}

	// Verify names match.
	svcNames := make(map[string]bool)
	for _, tr := range svc.Tools() {
		svcNames[tr.Name] = true
	}
	for _, td := range bridge.Tools() {
		if !svcNames[td.Name] {
			t.Errorf("bridge has tool %q not found in service", td.Name)
		}
	}
}

func TestBridgeToAPI_Good_DescribableGroup(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	bridge := api.NewToolBridge("/tools")
	BridgeToAPI(svc, bridge)

	// ToolBridge implements DescribableGroup.
	var dg api.DescribableGroup = bridge
	descs := dg.Describe()

	if len(descs) != len(svc.Tools()) {
		t.Fatalf("expected %d descriptions, got %d", len(svc.Tools()), len(descs))
	}

	for _, d := range descs {
		if d.Method != "POST" {
			t.Errorf("expected Method=POST for %s, got %q", d.Path, d.Method)
		}
		if d.Summary == "" {
			t.Errorf("expected non-empty Summary for %s", d.Path)
		}
		if len(d.Tags) == 0 {
			t.Errorf("expected non-empty Tags for %s", d.Path)
		}
	}
}

func TestBridgeToAPI_Good_FileRead(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file in the workspace.
	testContent := "hello from bridge test"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	svc, err := New(WithWorkspaceRoot(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	bridge := api.NewToolBridge("/tools")
	BridgeToAPI(svc, bridge)

	// Register with a Gin engine and make a request.
	engine := gin.New()
	rg := engine.Group(bridge.BasePath())
	bridge.RegisterRoutes(rg)

	body := `{"path":"test.txt"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/tools/file_read", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse the response envelope.
	var resp api.Response[ReadFileOutput]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got error: %+v", resp.Error)
	}
	if resp.Data.Content != testContent {
		t.Fatalf("expected content %q, got %q", testContent, resp.Data.Content)
	}
	if resp.Data.Path != "test.txt" {
		t.Fatalf("expected path %q, got %q", "test.txt", resp.Data.Path)
	}
}

func TestBridgeToAPI_Bad_InvalidJSON(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	bridge := api.NewToolBridge("/tools")
	BridgeToAPI(svc, bridge)

	engine := gin.New()
	rg := engine.Group(bridge.BasePath())
	bridge.RegisterRoutes(rg)

	// Send malformed JSON.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/tools/file_read", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		// The handler unmarshals via RESTHandler which returns an error,
		// but since it's a JSON parse error it ends up as tool_error.
		// Check we get a non-200 with an error envelope.
		if w.Code == http.StatusOK {
			t.Fatalf("expected non-200 for invalid JSON, got 200")
		}
	}

	var resp api.Response[any]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Success {
		t.Fatal("expected Success=false for invalid JSON")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
}

func TestBridgeToAPI_Good_EndToEnd(t *testing.T) {
	svc, err := New(WithWorkspaceRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	bridge := api.NewToolBridge("/tools")
	BridgeToAPI(svc, bridge)

	// Create an api.Engine with the bridge registered and Swagger enabled.
	e, err := api.New(
		api.WithSwagger("MCP Bridge Test", "Testing MCP-to-REST bridge", "0.1.0"),
	)
	if err != nil {
		t.Fatal(err)
	}
	e.Register(bridge)

	// Use a real test server because gin-swagger reads RequestURI
	// which is not populated by httptest.NewRecorder.
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	// Verify the health endpoint still works.
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", resp.StatusCode)
	}

	// Verify a tool endpoint is reachable through the engine.
	resp2, err := http.Post(srv.URL+"/tools/lang_list", "application/json", nil)
	if err != nil {
		t.Fatalf("lang_list request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /tools/lang_list, got %d", resp2.StatusCode)
	}

	var langResp api.Response[GetSupportedLanguagesOutput]
	if err := json.NewDecoder(resp2.Body).Decode(&langResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !langResp.Success {
		t.Fatalf("expected Success=true, got error: %+v", langResp.Error)
	}
	if len(langResp.Data.Languages) == 0 {
		t.Fatal("expected non-empty languages list")
	}

	// Verify Swagger endpoint contains tool paths.
	resp3, err := http.Get(srv.URL + "/swagger/doc.json")
	if err != nil {
		t.Fatalf("swagger request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /swagger/doc.json, got %d", resp3.StatusCode)
	}

	var specDoc map[string]any
	if err := json.NewDecoder(resp3.Body).Decode(&specDoc); err != nil {
		t.Fatalf("swagger unmarshal error: %v", err)
	}
	paths, ok := specDoc["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected 'paths' in swagger spec")
	}
	if _, ok := paths["/tools/file_read"]; !ok {
		t.Error("expected /tools/file_read in swagger paths")
	}
	if _, ok := paths["/tools/lang_list"]; !ok {
		t.Error("expected /tools/lang_list in swagger paths")
	}
}
