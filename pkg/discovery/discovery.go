// Package discovery provides dynamic model discovery for LLM providers.
// It queries provider APIs to list available models, removing the need
// for hardcoded model defaults during onboarding.
package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Provider describes a supported LLM provider for onboarding selection.
type Provider struct {
	// Key is the internal identifier (matches factory protocol prefix).
	Key string `json:"key"`
	// Name is the human-readable display name.
	Name string `json:"name"`
	// APIBase is the default API endpoint.
	APIBase string `json:"api_base"`
	// KeyHint tells the user where to get an API key.
	KeyHint string `json:"key_hint,omitempty"`
	// Local indicates the provider runs locally (no API key needed).
	Local bool `json:"local,omitempty"`
}

// Providers is the full catalog of supported providers for onboarding.
// Ordered by popularity / ease of setup.
var Providers = []Provider{
	{Key: "openrouter", Name: "OpenRouter (200+ models)", APIBase: "https://openrouter.ai/api/v1", KeyHint: "https://openrouter.ai/keys"},
	{Key: "anthropic", Name: "Anthropic (Claude)", APIBase: "https://api.anthropic.com", KeyHint: "https://console.anthropic.com/settings/keys"},
	{Key: "openai", Name: "OpenAI", APIBase: "https://api.openai.com/v1", KeyHint: "https://platform.openai.com/api-keys"},
	{Key: "gemini", Name: "Google Gemini", APIBase: "https://generativelanguage.googleapis.com/v1beta", KeyHint: "https://aistudio.google.com/apikey"},
	{Key: "deepseek", Name: "DeepSeek", APIBase: "https://api.deepseek.com/v1", KeyHint: "https://platform.deepseek.com/api_keys"},
	{Key: "groq", Name: "Groq", APIBase: "https://api.groq.com/openai/v1", KeyHint: "https://console.groq.com/keys"},
	{Key: "xai", Name: "xAI (Grok)", APIBase: "https://api.x.ai/v1", KeyHint: "https://console.x.ai/"},
	{Key: "mistral", Name: "Mistral", APIBase: "https://api.mistral.ai/v1", KeyHint: "https://console.mistral.ai/api-keys"},
	{Key: "cerebras", Name: "Cerebras", APIBase: "https://api.cerebras.ai/v1", KeyHint: "https://cloud.cerebras.ai/"},
	{Key: "nvidia", Name: "NVIDIA NIM", APIBase: "https://integrate.api.nvidia.com/v1", KeyHint: "https://build.nvidia.com/"},
	{Key: "ollama", Name: "Ollama (local)", APIBase: "http://localhost:11434/v1", Local: true},
	{Key: "vllm", Name: "vLLM (local)", APIBase: "http://localhost:8000/v1", Local: true},
	{Key: "litellm", Name: "LiteLLM (proxy)", APIBase: "http://localhost:4000/v1", Local: true},
	{Key: "azure", Name: "Azure OpenAI", APIBase: "", KeyHint: "https://portal.azure.com/"},
	{Key: "bedrock", Name: "AWS Bedrock", APIBase: "", KeyHint: "Uses AWS credentials (env/profile)"},
	{Key: "novita", Name: "Novita AI", APIBase: "https://api.novita.ai/openai", KeyHint: "https://novita.ai/"},
	{Key: "moonshot", Name: "Moonshot (Kimi)", APIBase: "https://api.moonshot.cn/v1"},
	{Key: "minimax", Name: "MiniMax", APIBase: "https://api.minimaxi.com/v1"},
	{Key: "volcengine", Name: "VolcEngine (ByteDance)", APIBase: "https://ark.cn-beijing.volces.com/api/v3"},
	{Key: "qwen", Name: "Qwen (Alibaba)", APIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{Key: "zhipu", Name: "Zhipu AI (GLM)", APIBase: "https://open.bigmodel.cn/api/paas/v4"},
	{Key: "qwen-intl", Name: "Qwen International (Alibaba)", APIBase: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"},
	{Key: "vivgrid", Name: "VivGrid", APIBase: "https://api.vivgrid.com/v1"},
	{Key: "modelscope", Name: "ModelScope", APIBase: "https://api-inference.modelscope.cn/v1"},
	{Key: "mimo", Name: "Xiaomi MiMo", APIBase: "https://api.xiaomimimo.com/v1"},
}

// Model represents a discovered model from a provider API.
type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// DiscoverModels queries a provider's models endpoint and returns available models.
// The providerKey is used to determine auth headers and URL format.
// For providers that don't support model listing, returns an empty list and nil error.
func DiscoverModels(providerKey, apiBase, apiKey string) ([]Model, error) {
	url, headers := buildDiscoveryRequest(providerKey, apiBase, apiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("invalid API key (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	return parseModelsResponse(body)
}

// buildDiscoveryRequest returns the URL and headers for model discovery
// based on the provider. Each provider has its own auth scheme.
func buildDiscoveryRequest(providerKey, apiBase, apiKey string) (string, map[string]string) {
	base := strings.TrimRight(apiBase, "/")
	headers := make(map[string]string)

	switch providerKey {
	case "anthropic":
		// Anthropic uses x-api-key header and has its own models endpoint
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
		return base + "/v1/models", headers

	case "gemini":
		// Gemini uses API key as query param or x-goog-api-key header
		headers["x-goog-api-key"] = apiKey
		return base + "/models", headers

	default:
		// OpenAI-compatible: Bearer auth + /models
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		return base + "/models", headers
	}
}

// parseModelsResponse handles multiple response formats from different providers.
func parseModelsResponse(body []byte) ([]Model, error) {
	// Try OpenAI format: { "data": [ { "id": "..." } ] }
	var openaiResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openaiResp); err == nil && len(openaiResp.Data) > 0 {
		models := make([]Model, 0, len(openaiResp.Data))
		for _, m := range openaiResp.Data {
			models = append(models, Model{ID: m.ID, OwnedBy: m.OwnedBy})
		}
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		return models, nil
	}

	// Try Gemini format: { "models": [ { "name": "models/gemini-pro" } ] }
	var geminiResp struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &geminiResp); err == nil && len(geminiResp.Models) > 0 {
		models := make([]Model, 0, len(geminiResp.Models))
		for _, m := range geminiResp.Models {
			id := strings.TrimPrefix(m.Name, "models/")
			models = append(models, Model{ID: id, OwnedBy: "google"})
		}
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		return models, nil
	}

	// Try bare array: [ { "id": "..." } ]
	var arrayResp []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	}
	if err := json.Unmarshal(body, &arrayResp); err == nil && len(arrayResp) > 0 {
		models := make([]Model, 0, len(arrayResp))
		for _, m := range arrayResp {
			models = append(models, Model{ID: m.ID, OwnedBy: m.OwnedBy})
		}
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		return models, nil
	}

	return nil, nil
}

// FindProvider returns the Provider entry for a given key, or nil if not found.
func FindProvider(key string) *Provider {
	for i := range Providers {
		if Providers[i].Key == key {
			return &Providers[i]
		}
	}
	return nil
}
