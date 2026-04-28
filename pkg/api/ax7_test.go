package api

import (
	"net/http"
	"net/http/httptest"

	core "dappco.re/go"
	"github.com/gin-gonic/gin"
)

func TestAPI_New_Good(t *core.T) {
	provider := New()
	name := provider.Name()
	basePath := provider.BasePath()

	core.AssertNotNil(t, provider)
	core.AssertEqual(t, "ai", name)
	core.AssertEqual(t, "/v1", basePath)
}

func TestAPI_New_Bad(t *core.T) {
	first := New()
	second := New()
	same := first == second

	core.AssertNotNil(t, first)
	core.AssertNotNil(t, second)
	core.AssertFalse(t, same)
}

func TestAPI_New_Ugly(t *core.T) {
	provider := New()
	descriptions := provider.Describe()
	got := len(descriptions)

	core.AssertTrue(t, got > 0)
	core.AssertEqual(t, "ai", provider.Name())
}

func TestAPI_NewProvider_Good(t *core.T) {
	provider := NewProvider()
	name := provider.Name()
	basePath := provider.BasePath()

	core.AssertNotNil(t, provider)
	core.AssertEqual(t, "ai", name)
	core.AssertEqual(t, "/v1", basePath)
}

func TestAPI_NewProvider_Bad(t *core.T) {
	first := NewProvider()
	second := NewProvider()
	same := first == second

	core.AssertNotNil(t, first)
	core.AssertNotNil(t, second)
	core.AssertFalse(t, same)
}

func TestAPI_NewProvider_Ugly(t *core.T) {
	provider := NewProvider()
	descriptions := provider.Describe()
	got := len(descriptions)

	core.AssertEqual(t, 6, got)
	core.AssertEqual(t, "ai", provider.Name())
}

func TestAPI_AIProvider_Name_Good(t *core.T) {
	provider := &AIProvider{}
	got := provider.Name()
	want := "ai"

	core.AssertEqual(t, want, got)
	core.AssertNotEqual(t, "", got)
}

func TestAPI_AIProvider_Name_Bad(t *core.T) {
	var provider *AIProvider
	got := provider.Name()
	want := "ai"

	core.AssertEqual(t, want, got)
	core.AssertNotEqual(t, "", got)
}

func TestAPI_AIProvider_Name_Ugly(t *core.T) {
	provider := NewProvider()
	got := provider.Name()
	again := provider.Name()

	core.AssertEqual(t, got, again)
	core.AssertEqual(t, "ai", got)
}

func TestAPI_AIProvider_BasePath_Good(t *core.T) {
	provider := &AIProvider{}
	got := provider.BasePath()
	want := "/v1"

	core.AssertEqual(t, want, got)
	core.AssertTrue(t, core.HasPrefix(got, "/"))
}

func TestAPI_AIProvider_BasePath_Bad(t *core.T) {
	var provider *AIProvider
	got := provider.BasePath()
	want := "/v1"

	core.AssertEqual(t, want, got)
	core.AssertNotEqual(t, "", got)
}

func TestAPI_AIProvider_BasePath_Ugly(t *core.T) {
	provider := NewProvider()
	got := provider.BasePath()
	again := provider.BasePath()

	core.AssertEqual(t, got, again)
	core.AssertEqual(t, "/v1", got)
}

func TestAPI_AIProvider_RegisterRoutes_Good(t *core.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewProvider().RegisterRoutes(router.Group("/v1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	router.ServeHTTP(rec, req)
	core.AssertEqual(t, http.StatusOK, rec.Code)
}

func TestAPI_AIProvider_RegisterRoutes_Bad(t *core.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var provider *AIProvider

	provider.RegisterRoutes(router.Group("/v1"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	router.ServeHTTP(rec, req)
	core.AssertEqual(t, http.StatusNotFound, rec.Code)
}

func TestAPI_AIProvider_RegisterRoutes_Ugly(t *core.T) {
	provider := NewProvider()
	core.AssertNotPanics(t, func() {
		provider.RegisterRoutes(nil)
	})
	core.AssertEqual(t, "ai", provider.Name())
}

func TestAPI_AIProvider_Describe_Good(t *core.T) {
	provider := NewProvider()
	descriptions := provider.Describe()
	first := descriptions[0]

	core.AssertLen(t, descriptions, 6)
	core.AssertEqual(t, http.MethodPost, first.Method)
}

func TestAPI_AIProvider_Describe_Bad(t *core.T) {
	var provider *AIProvider
	descriptions := provider.Describe()
	got := len(descriptions)

	core.AssertEqual(t, 6, got)
	core.AssertEqual(t, "/health", descriptions[5].Path)
}

func TestAPI_AIProvider_Describe_Ugly(t *core.T) {
	provider := NewProvider()
	descriptions := provider.Describe()
	health := descriptions[5]

	core.AssertEqual(t, http.MethodGet, health.Method)
	core.AssertEqual(t, "/health", health.Path)
}
