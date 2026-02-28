package vectordb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"abot/pkg/types"
)

// OpenAIEmbedder calls an OpenAI-compatible embedding endpoint.
type OpenAIEmbedder struct {
	client    *http.Client
	baseURL   string
	apiKey    string
	model     string
	dimension int
}

// OpenAIEmbedderConfig holds configuration for the embedding client.
type OpenAIEmbedderConfig struct {
	BaseURL   string // e.g. "https://api.openai.com/v1"
	APIKey    string
	Model     string // e.g. "text-embedding-3-small"
	Dimension int    // e.g. 1536
}

// NewOpenAIEmbedder creates an embedder using an OpenAI-compatible API.
func NewOpenAIEmbedder(cfg OpenAIEmbedderConfig) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		client:    &http.Client{},
		baseURL:   cfg.BaseURL,
		apiKey:    cfg.APIKey,
		model:     cfg.Model,
		dimension: cfg.Dimension,
	}
}

func (e *OpenAIEmbedder) Dimension() int { return e.dimension }

func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(map[string]any{
		"input": texts,
		"model": e.model,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API %d: %s", resp.StatusCode, raw)
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	out := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
}

var _ Embedder = (*OpenAIEmbedder)(nil)
var _ types.Embedder = (*OpenAIEmbedder)(nil)
