package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"

	. "dappco.re/go"
)

func TestOllama_NewClient_Good(t *T) {
	base, err := url.Parse("http://127.0.0.1:11434")
	RequireNoError(t, err)
	client := NewClient(base, nil)

	AssertNotNil(t, client)
	AssertEqual(t, base, client.baseURL)
}

func TestOllama_NewClient_Bad(t *T) {
	client := NewClient(nil, nil)
	got := client.baseURL
	httpClient := client.httpClient

	AssertNil(t, got)
	AssertNotNil(t, httpClient)
}

func TestOllama_NewClient_Ugly(t *T) {
	base, err := url.Parse("http://127.0.0.1:11434/base/")
	RequireNoError(t, err)
	client := NewClient(base, http.DefaultClient)

	AssertNotNil(t, client)
	AssertEqual(t, http.DefaultClient, client.httpClient)
}

func TestOllama_Client_Embed_Good(t *T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AssertEqual(t, "/api/embed", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"embedding":[1,2,3]}`))
		AssertNoError(t, err)
	}))
	defer server.Close()

	base, err := url.Parse(server.URL)
	RequireNoError(t, err)
	client := NewClient(base, server.Client())
	response, err := client.Embed(Background(), &EmbedRequest{Model: "m", Input: "hello"})

	AssertNoError(t, err)
	AssertEqual(t, [][]float64{{1, 2, 3}}, response.Embeddings)
}

func TestOllama_Client_Embed_Bad(t *T) {
	client := NewClient(nil, nil)
	response, err := client.Embed(Background(), &EmbedRequest{Model: "m", Input: "hello"})
	got := ErrorMessage(err)

	AssertNil(t, response)
	AssertError(t, err)
	AssertContains(t, got, "base url")
}

func TestOllama_Client_Embed_Ugly(t *T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := w.Write([]byte("model unavailable"))
		AssertNoError(t, err)
	}))
	defer server.Close()

	base, err := url.Parse(server.URL)
	RequireNoError(t, err)
	client := NewClient(base, server.Client())
	response, err := client.Embed(Background(), &EmbedRequest{Model: "m", Input: "hello"})

	AssertNil(t, response)
	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "model unavailable")
}
