package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestBuildBenchmarkModelNames_Good_DefaultsPlusInstalledExtras(t *testing.T) {
	installedModelNames := []string{
		"embeddinggemma:latest",
		"nomic-embed-text:latest",
		"mxbai-embed-large:latest",
		"snowflake-arctic-embed2:335m",
		"mxbai-embed-large:latest",
	}

	got := buildBenchmarkModelNames(installedModelNames)
	want := []string{
		"nomic-embed-text",
		"embeddinggemma",
		"mxbai-embed-large:latest",
		"snowflake-arctic-embed2:335m",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildBenchmarkModelNames() = %v, want %v", got, want)
	}
}

func TestHasInstalledModel_Good_DefaultMatchesTaggedInstall(t *testing.T) {
	installedModelNames := []string{
		"nomic-embed-text:latest",
		"mxbai-embed-large:latest",
	}

	if !hasInstalledModel(installedModelNames, "nomic-embed-text") {
		t.Fatal("expected default model to match installed :latest tag")
	}
	if hasInstalledModel(installedModelNames, "embeddinggemma") {
		t.Fatal("expected missing model to return false")
	}
}

func TestDecodeInstalledModelNames_Good(t *testing.T) {
	got, err := decodeInstalledModelNames([]byte(`{
		"models": [
			{"name": "nomic-embed-text:latest"},
			{"name": "snowflake-arctic-embed2:335m"},
			{"name": ""}
		]
	}`))
	if err != nil {
		t.Fatalf("decodeInstalledModelNames(): %v", err)
	}

	want := []string{
		"nomic-embed-text:latest",
		"snowflake-arctic-embed2:335m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeInstalledModelNames() = %v, want %v", got, want)
	}
}

func TestMain_flattenMemoryGroups_Good_FlattenedOrder(t *testing.T) {
	gotMemories, gotTopics := flattenMemoryGroups([]struct {
		topic    string
		memories []string
	}{{
		topic:    "topic-a",
		memories: []string{"a1", "a2"},
	}, {
		topic:    "topic-b",
		memories: []string{"b1"},
	}})

	wantMemories := []string{"a1", "a2", "b1"}
	wantTopics := []string{"topic-a", "topic-a", "topic-b"}

	if !reflect.DeepEqual(gotMemories, wantMemories) {
		t.Fatalf("flattenMemoryGroups memories = %v, want %v", gotMemories, wantMemories)
	}
	if !reflect.DeepEqual(gotTopics, wantTopics) {
		t.Fatalf("flattenMemoryGroups topics = %v, want %v", gotTopics, wantTopics)
	}
}

func TestMain_buildHTTPClient_Good_DefaultTransportCloned(t *testing.T) {
	client := buildHTTPClient(false)
	if client == nil || client.Transport == nil {
		t.Fatal("expected a client with transport configured")
	}
	if got, ok := client.Transport.(*http.Transport); !ok {
		t.Fatalf("expected transport type *http.Transport, got %T", client.Transport)
	} else if got.TLSClientConfig != nil && got.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("insecure skip verify should be false by default")
	}
}

func TestMain_buildHTTPClient_Good_EnablesInsecureSkipVerify(t *testing.T) {
	client := buildHTTPClient(true)
	transport := client.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to be enabled when allowInsecureTLS is true")
	}
}

func TestMain_buildHTTPClient_Ugly_UsesConfiguredTimeout(t *testing.T) {
	client := buildHTTPClient(false)
	if client.Timeout != ollamaHTTPTimeout {
		t.Fatalf("buildHTTPClient() timeout = %v, want %v", client.Timeout, ollamaHTTPTimeout)
	}
}

func TestMain_qualityLabel_Good_Excellent(t *testing.T) {
	if got := qualityLabel(0.16); got != "(excellent)" {
		t.Fatalf("qualityLabel(0.16) = %q, want (excellent)", got)
	}
}

func TestMain_qualityLabel_Bad_Fair(t *testing.T) {
	if got := qualityLabel(0.06); got != "(fair)" {
		t.Fatalf("qualityLabel(0.06) = %q, want (fair)", got)
	}
}

func TestMain_qualityLabel_Ugly_PoorOnBoundaryAndNaN(t *testing.T) {
	if got := qualityLabel(0.10); got != "(good)" {
		t.Fatalf("qualityLabel(0.10) = %q, want (good)", got)
	}
	if got := qualityLabel(math.NaN()); got != "(poor)" {
		t.Fatalf("qualityLabel(NaN) = %q, want (poor)", got)
	}
}

func TestMain_listInstalledModelNames_Good_UsesOllamaAPI(t *testing.T) {
	previousURL := *ollamaURL
	previousClient := httpClient

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"models":[{"name":"nomic-embed-text:latest"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	*ollamaURL = srv.URL
	httpClient = buildHTTPClient(false)

	t.Cleanup(func() {
		*ollamaURL = previousURL
		httpClient = previousClient
	})

	got, err := listInstalledModelNames()
	if err != nil {
		t.Fatalf("listInstalledModelNames: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"nomic-embed-text:latest"}) {
		t.Fatalf("listInstalledModelNames() = %v, want %v", got, []string{"nomic-embed-text:latest"})
	}
}

func TestMain_listInstalledModelNames_Bad_HTTPError(t *testing.T) {
	previousURL := *ollamaURL
	previousClient := httpClient

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		http.Error(w, "server fail", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	*ollamaURL = srv.URL
	httpClient = buildHTTPClient(false)

	t.Cleanup(func() {
		*ollamaURL = previousURL
		httpClient = previousClient
	})

	if _, err := listInstalledModelNames(); err == nil {
		t.Fatal("expected listInstalledModelNames to fail on non-200 status")
	}
}

func TestMain_listInstalledModelNames_Ugly_EmptyResponse(t *testing.T) {
	previousURL := *ollamaURL
	previousClient := httpClient

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`{"models":[]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	*ollamaURL = srv.URL
	httpClient = buildHTTPClient(false)

	t.Cleanup(func() {
		*ollamaURL = previousURL
		httpClient = previousClient
	})

	got, err := listInstalledModelNames()
	if err != nil {
		t.Fatalf("listInstalledModelNames: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no models, got %v", got)
	}
}

func TestMain_embed_Good_ParsesEmbeddingVector(t *testing.T) {
	previousURL := *ollamaURL
	previousClient := httpClient

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if _, err := w.Write([]byte(`{"embedding":[0.1,0.2,0.3]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	*ollamaURL = srv.URL
	httpClient = buildHTTPClient(false)

	t.Cleanup(func() {
		*ollamaURL = previousURL
		httpClient = previousClient
	})

	got, err := embed("nomic-embed-text", "hello")
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	want := []float64{0.1, 0.2, 0.3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("embed() = %v, want %v", got, want)
	}
}

func TestMain_embed_Bad_HTTPError(t *testing.T) {
	previousURL := *ollamaURL
	previousClient := httpClient

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failure", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)
	*ollamaURL = srv.URL
	httpClient = buildHTTPClient(false)

	t.Cleanup(func() {
		*ollamaURL = previousURL
		httpClient = previousClient
	})

	if _, err := embed("nomic-embed-text", "hello"); err == nil {
		t.Fatal("expected embed to fail on non-200 status")
	}
}

func TestMain_embed_Ugly_EmptyEmbeddingErrors(t *testing.T) {
	previousURL := *ollamaURL
	previousClient := httpClient

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`{"embedding":[]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	*ollamaURL = srv.URL
	httpClient = buildHTTPClient(false)

	t.Cleanup(func() {
		*ollamaURL = previousURL
		httpClient = previousClient
	})

	if _, err := embed("nomic-embed-text", "hello"); err == nil {
		t.Fatal("expected empty embeddings to error")
	}
}
