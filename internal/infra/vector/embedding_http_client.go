package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type EmbeddingHTTPClient struct {
	baseURL    string
	modelName  string
	httpClient *http.Client
}

func NewEmbeddingHTTPClient(baseURL string, modelName string, httpClient *http.Client) (*EmbeddingHTTPClient, error) {
	if baseURL == "" || modelName == "" || httpClient == nil {
		return nil, errors.New("invalid embedding http client config")
	}
	return &EmbeddingHTTPClient{baseURL: baseURL, modelName: modelName, httpClient: httpClient}, nil
}

func (c *EmbeddingHTTPClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, errors.New("empty text")
	}

	body := map[string]any{
		"model":  c.modelName,
		"prompt": text,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding service status: %d", resp.StatusCode)
	}

	var parsed struct {
		Embedding []float32 `json:"embedding"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return parsed.Embedding, nil
}