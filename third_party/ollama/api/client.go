package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxEmbedResponseBytes = 16 << 20

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func NewClient(baseURL *url.URL, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *Client) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("ollama client is nil")
	}
	if req == nil {
		return nil, fmt.Errorf("embed request is nil")
	}
	if c.baseURL == nil {
		return nil, fmt.Errorf("ollama base url is nil")
	}

	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/api/embed"

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxEmbedResponseBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(body) == 0 {
			return nil, fmt.Errorf("ollama embed request failed: %s", resp.Status)
		}
		return nil, fmt.Errorf("ollama embed request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Embeddings [][]float64 `json:"embeddings"`
		Embedding  []float64   `json:"embedding"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}

	response := &EmbedResponse{}
	switch {
	case len(decoded.Embeddings) > 0:
		response.Embeddings = decoded.Embeddings
	case len(decoded.Embedding) > 0:
		response.Embeddings = [][]float64{decoded.Embedding}
	}

	return response, nil
}
