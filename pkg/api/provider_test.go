// SPDX-License-Identifier: EUPL-1.2

package api

import (
	"net/http"
	"testing"

	coreprovider "dappco.re/go/api/pkg/provider"
	"github.com/gin-gonic/gin"
)

func TestNewProvider_Good(t *testing.T) {
	p := NewProvider()
	if p == nil {
		t.Fatal("expected provider")
	}
	if New() == nil {
		t.Fatal("expected New alias to return provider")
	}

	var provider coreprovider.Provider = p
	if provider.Name() != "ai" {
		t.Fatalf("expected name %q, got %q", "ai", provider.Name())
	}
	if provider.BasePath() != "/v1" {
		t.Fatalf("expected base path %q, got %q", "/v1", provider.BasePath())
	}

	want := map[string]bool{
		http.MethodPost + " /embeddings/text":        false,
		http.MethodPost + " /embeddings/behavioural": false,
		http.MethodPost + " /score/content":          false,
		http.MethodPost + " /score/imprint":          false,
		http.MethodGet + " /score/:id":               false,
		http.MethodGet + " /health":                  false,
	}
	for _, desc := range p.Describe() {
		key := desc.Method + " " + desc.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("expected route description for %s", key)
		}
	}
}

func TestNewProvider_Bad(t *testing.T) {
	p := NewProvider()

	assertDoesNotPanic(t, func() {
		p.RegisterRoutes(nil)
	})
}

func TestNewProvider_Ugly(t *testing.T) {
	var p *AIProvider
	router := gin.New()

	assertDoesNotPanic(t, func() {
		p.RegisterRoutes(router.Group("/v1"))
	})
}

func assertDoesNotPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic, got %v", r)
		}
	}()
	fn()
}
