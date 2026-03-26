package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiEmbeddingProvider_Embed(t *testing.T) {
	// Mock Gemini API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"embedding": map[string]any{
				"values": []float64{0.1, 0.2, 0.3, 0.4},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewGeminiEmbeddingProvider("test-key", "text-embedding-004", 4)
	provider.baseURL = server.URL // override for test

	result, err := provider.Embed(context.Background(), "Hello world")
	require.NoError(t, err)
	assert.Len(t, result, 4)
	assert.InDelta(t, 0.1, result[0], 0.001)
}

func TestOpenAIEmbeddingProvider_Embed(t *testing.T) {
	// Mock OpenAI API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": []float64{0.5, 0.6, 0.7, 0.8}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIEmbeddingProvider("test-key", "text-embedding-3-small", 4)
	provider.baseURL = server.URL // override for test

	result, err := provider.Embed(context.Background(), "Hello world")
	require.NoError(t, err)
	assert.Len(t, result, 4)
	assert.InDelta(t, 0.5, result[0], 0.001)
}

func TestNewEmbeddingProvider_Factory(t *testing.T) {
	p, err := NewEmbeddingProvider(EmbeddingConfig{Provider: "gemini", APIKey: "k", Model: "m", Dimensions: 768})
	require.NoError(t, err)
	assert.Equal(t, 768, p.Dimensions())

	p2, err := NewEmbeddingProvider(EmbeddingConfig{Provider: "openai", APIKey: "k", Model: "m", Dimensions: 1536})
	require.NoError(t, err)
	assert.Equal(t, 1536, p2.Dimensions())

	_, err = NewEmbeddingProvider(EmbeddingConfig{Provider: "unknown", APIKey: "k"})
	assert.Error(t, err)
}
