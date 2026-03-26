package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// EmbeddingConfig holds configuration for the embedding provider.
type EmbeddingConfig struct {
	Provider   string
	Model      string
	APIKey     string
	Dimensions int
	BaseURL    string // optional override for custom endpoints
}

// EmbeddingProvider generates vector embeddings from text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimensions() int
}

// NewEmbeddingProvider creates a provider from config.
func NewEmbeddingProvider(cfg EmbeddingConfig) (EmbeddingProvider, error) {
	switch cfg.Provider {
	case "gemini":
		p := NewGeminiEmbeddingProvider(cfg.APIKey, cfg.Model, cfg.Dimensions)
		if cfg.BaseURL != "" {
			p.baseURL = cfg.BaseURL
		}
		return p, nil
	case "openai":
		p := NewOpenAIEmbeddingProvider(cfg.APIKey, cfg.Model, cfg.Dimensions)
		if cfg.BaseURL != "" {
			p.baseURL = cfg.BaseURL
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s (supported: gemini, openai)", cfg.Provider)
	}
}

// GeminiEmbeddingProvider calls the Gemini embedContent API.
type GeminiEmbeddingProvider struct {
	apiKey     string
	model      string
	dimensions int
	baseURL    string
	client     *http.Client
}

// NewGeminiEmbeddingProvider creates a new Gemini embedding provider.
func NewGeminiEmbeddingProvider(apiKey, model string, dimensions int) *GeminiEmbeddingProvider {
	return &GeminiEmbeddingProvider{
		apiKey:     apiKey,
		model:      model,
		dimensions: dimensions,
		baseURL:    "https://generativelanguage.googleapis.com",
		client:     &http.Client{},
	}
}

// Dimensions returns the number of dimensions in the embeddings.
func (p *GeminiEmbeddingProvider) Dimensions() int {
	return p.dimensions
}

// Embed generates an embedding for the given text.
func (p *GeminiEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model": "models/" + p.model,
		"content": map[string]any{
			"parts": []map[string]any{
				{"text": text},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:embedContent?key=%s", p.baseURL, p.model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini embed: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embedding struct {
			Values []float64 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gemini embed: decode response: %w", err)
	}

	out := make([]float32, len(result.Embedding.Values))
	for i, v := range result.Embedding.Values {
		out[i] = float32(v)
	}
	return out, nil
}

// OpenAIEmbeddingProvider calls the OpenAI embeddings API.
type OpenAIEmbeddingProvider struct {
	apiKey     string
	model      string
	dimensions int
	baseURL    string
	client     *http.Client
}

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider.
func NewOpenAIEmbeddingProvider(apiKey, model string, dimensions int) *OpenAIEmbeddingProvider {
	return &OpenAIEmbeddingProvider{
		apiKey:     apiKey,
		model:      model,
		dimensions: dimensions,
		baseURL:    "https://api.openai.com",
		client:     &http.Client{},
	}
}

// Dimensions returns the number of dimensions in the embeddings.
func (p *OpenAIEmbeddingProvider) Dimensions() int {
	return p.dimensions
}

// Embed generates an embedding for the given text.
func (p *OpenAIEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model": p.model,
		"input": text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai embed: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai embed: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embed: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai embed: decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("openai embed: empty data in response")
	}

	out := make([]float32, len(result.Data[0].Embedding))
	for i, v := range result.Data[0].Embedding {
		out[i] = float32(v)
	}
	return out, nil
}
