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

// Provider describes a supported LLM provider for onboarding selection
// and runtime routing. This is the single source of truth — the factory
// and getDefaultAPIBase both read from this catalog.
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
	// OpenAICompat marks providers that use the standard OpenAI-compatible
	// HTTP protocol. The factory creates them automatically — no switch case needed.
	OpenAICompat bool `json:"openai_compat,omitempty"`
}

// Providers is the full catalog of supported providers for onboarding.
// Ordered by popularity / ease of setup.
//
// To add a new OpenAI-compatible provider: add one entry here with
// OpenAICompat: true. That's it — factory, API base lookup, and
// onboarding all pick it up automatically.
var Providers = []Provider{
	{Key: "openrouter", Name: "OpenRouter (200+ models)", APIBase: "https://openrouter.ai/api/v1", KeyHint: "https://openrouter.ai/keys", OpenAICompat: true},
	{Key: "anthropic", Name: "Anthropic (Claude)", APIBase: "https://api.anthropic.com", KeyHint: "https://console.anthropic.com/settings/keys"},
	{Key: "openai", Name: "OpenAI", APIBase: "https://api.openai.com/v1", KeyHint: "https://platform.openai.com/api-keys"},
	{Key: "gemini", Name: "Google Gemini", APIBase: "https://generativelanguage.googleapis.com/v1beta", KeyHint: "https://aistudio.google.com/apikey"},
	{Key: "deepseek", Name: "DeepSeek", APIBase: "https://api.deepseek.com/v1", KeyHint: "https://platform.deepseek.com/api_keys", OpenAICompat: true},
	{Key: "groq", Name: "Groq", APIBase: "https://api.groq.com/openai/v1", KeyHint: "https://console.groq.com/keys", OpenAICompat: true},
	{Key: "xai", Name: "xAI (Grok)", APIBase: "https://api.x.ai/v1", KeyHint: "https://console.x.ai/", OpenAICompat: true},
	{Key: "mistral", Name: "Mistral", APIBase: "https://api.mistral.ai/v1", KeyHint: "https://console.mistral.ai/api-keys", OpenAICompat: true},
	{Key: "cerebras", Name: "Cerebras", APIBase: "https://api.cerebras.ai/v1", KeyHint: "https://cloud.cerebras.ai/", OpenAICompat: true},
	{Key: "nvidia", Name: "NVIDIA NIM", APIBase: "https://integrate.api.nvidia.com/v1", KeyHint: "https://build.nvidia.com/", OpenAICompat: true},
	{Key: "ollama", Name: "Ollama (local/network)", APIBase: "http://localhost:11434/v1", Local: true, OpenAICompat: true},
	{Key: "ollama-cloud", Name: "Ollama Cloud", APIBase: "https://api.ollama.com/v1", KeyHint: "https://ollama.com/settings/keys", OpenAICompat: true},
	{Key: "llamacpp", Name: "llama.cpp server (local/network)", APIBase: "http://localhost:8080/v1", Local: true, OpenAICompat: true},
	{Key: "vllm", Name: "vLLM (local)", APIBase: "http://localhost:8000/v1", Local: true, OpenAICompat: true},
	{Key: "litellm", Name: "LiteLLM (proxy)", APIBase: "http://localhost:4000/v1", Local: true, OpenAICompat: true},
	{Key: "azure", Name: "Azure OpenAI", APIBase: "", KeyHint: "https://portal.azure.com/"},
	{Key: "bedrock", Name: "AWS Bedrock", APIBase: "", KeyHint: "Uses AWS credentials (env/profile)"},
	{Key: "novita", Name: "Novita AI", APIBase: "https://api.novita.ai/openai", KeyHint: "https://novita.ai/", OpenAICompat: true},
	{Key: "moonshot", Name: "Moonshot (Kimi)", APIBase: "https://api.moonshot.cn/v1", OpenAICompat: true},
	{Key: "kimi-coding", Name: "Kimi Coding (subscription)", APIBase: "https://api.kimi.com/coding/v1", KeyHint: "https://www.kimi.com/code/en", OpenAICompat: true},
	{Key: "minimax", Name: "MiniMax", APIBase: "https://api.minimaxi.com/v1"},
	{Key: "minimax-coding", Name: "MiniMax Coding (subscription)", APIBase: "https://api.minimaxi.com/v1", KeyHint: "https://platform.minimax.io/"},
	{Key: "volcengine", Name: "VolcEngine (ByteDance)", APIBase: "https://ark.cn-beijing.volces.com/api/v3", OpenAICompat: true},
	{Key: "qwen", Name: "Qwen (Alibaba)", APIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{Key: "zhipu", Name: "Zhipu AI (GLM)", APIBase: "https://open.bigmodel.cn/api/paas/v4", OpenAICompat: true},
	{Key: "zhipu-coding", Name: "Zhipu AI Coding (subscription)", APIBase: "https://open.bigmodel.cn/api/coding/paas/v4", KeyHint: "https://open.bigmodel.cn/", OpenAICompat: true},
	{Key: "qwen-intl", Name: "Qwen International (Alibaba)", APIBase: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"},
	{Key: "vivgrid", Name: "VivGrid", APIBase: "https://api.vivgrid.com/v1", OpenAICompat: true},
	{Key: "modelscope", Name: "ModelScope", APIBase: "https://api-inference.modelscope.cn/v1", OpenAICompat: true},
	{Key: "mimo", Name: "Xiaomi MiMo", APIBase: "https://api.xiaomimimo.com/v1", OpenAICompat: true},
	{Key: "avian", Name: "Avian", APIBase: "https://api.avian.io/v1", OpenAICompat: true},
	{Key: "longcat", Name: "LongCat", APIBase: "https://api.longcat.chat/openai", OpenAICompat: true},
	{Key: "shengsuanyun", Name: "ShengSuanYun", APIBase: "https://router.shengsuanyun.com/api/v1", OpenAICompat: true},
}

// providerIndex is a lookup map built once from Providers.
var providerIndex map[string]*Provider

func init() {
	providerIndex = make(map[string]*Provider, len(Providers))
	for i := range Providers {
		providerIndex[Providers[i].Key] = &Providers[i]
	}
}

// IsOpenAICompat returns true if the protocol key maps to a registered
// OpenAI-compatible provider. Used by the factory to avoid hardcoded case lists.
func IsOpenAICompat(key string) bool {
	if p, ok := providerIndex[key]; ok {
		return p.OpenAICompat
	}
	return false
}

// DefaultAPIBase returns the default API base for a protocol key, or "".
func DefaultAPIBase(key string) string {
	if p, ok := providerIndex[key]; ok {
		return p.APIBase
	}
	return ""
}

// Model represents a discovered model from a provider API.
type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// DiscoverModels queries a provider's models endpoint and returns available models.
// For providers without a /models endpoint, uses a hardcoded catalog.
// Falls back to the LiteLLM community model catalog as a last resort.
func DiscoverModels(providerKey, apiBase, apiKey string) ([]Model, error) {
	// Check for a hardcoded catalog first (providers without /models endpoint)
	if staticModels := staticCatalog(providerKey); staticModels != nil {
		return staticModels, nil
	}

	// Try the provider's own /models endpoint
	models, err := discoverFromProvider(providerKey, apiBase, apiKey)
	if err != nil {
		// Auth errors are real — don't fall back, let the user fix the key
		return nil, err
	}
	if len(models) > 0 {
		return models, nil
	}

	// Provider doesn't support listing — fall back to LiteLLM catalog
	catalogModels, catalogErr := discoverFromCatalog(providerKey)
	if catalogErr != nil {
		return nil, fmt.Errorf("provider API returned no models; catalog fallback failed: %w", catalogErr)
	}
	if len(catalogModels) == 0 {
		return nil, nil
	}
	return catalogModels, nil
}

// staticCatalog returns a hardcoded model list for providers that don't expose
// a /models API endpoint. Returns nil if the provider supports dynamic discovery.
func staticCatalog(providerKey string) []Model {
	switch providerKey {
	case "minimax", "minimax-coding":
		return []Model{
			{ID: "MiniMax-M2.7", OwnedBy: "minimax"},
			{ID: "MiniMax-M2.7-highspeed", OwnedBy: "minimax"},
			{ID: "MiniMax-M2.5", OwnedBy: "minimax"},
			{ID: "MiniMax-M2.5-highspeed", OwnedBy: "minimax"},
			{ID: "MiniMax-M2.1", OwnedBy: "minimax"},
			{ID: "MiniMax-M2.1-highspeed", OwnedBy: "minimax"},
			{ID: "MiniMax-M2", OwnedBy: "minimax"},
			{ID: "MiniMax-M2-highspeed", OwnedBy: "minimax"},
		}
	default:
		return nil
	}
}

// discoverFromProvider queries the provider's own /models endpoint.
func discoverFromProvider(providerKey, apiBase, apiKey string) ([]Model, error) {
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
		return nil, nil // network error — not an auth issue, try fallback
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, nil
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("invalid API key (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	return parseModelsResponse(body)
}

// litellmCatalogURL is the LiteLLM community model catalog.
// This is a static JSON data file — no code is fetched or executed.
const litellmCatalogURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// discoverFromCatalog fetches the LiteLLM model catalog and filters by provider.
func discoverFromCatalog(providerKey string) ([]Model, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(litellmCatalogURL)
	if err != nil {
		return nil, fmt.Errorf("fetching catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog returned HTTP %d", resp.StatusCode)
	}

	// Limit to 10MB — the file is ~2MB currently
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("reading catalog: %w", err)
	}

	var catalog map[string]struct {
		Provider string `json:"litellm_provider"`
		Mode     string `json:"mode"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("parsing catalog: %w", err)
	}

	var models []Model
	seen := make(map[string]bool)
	for key, entry := range catalog {
		if entry.Provider != providerKey || entry.Mode != "chat" {
			continue
		}
		// Model keys are like "groq/llama-3.3-70b" — strip provider prefix
		modelID := key
		if idx := strings.Index(key, "/"); idx >= 0 {
			modelID = key[idx+1:]
		}
		if seen[modelID] {
			continue
		}
		seen[modelID] = true
		models = append(models, Model{ID: modelID, OwnedBy: entry.Provider})
	}

	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
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

	case "kimi-coding":
		// Kimi Coding uses the same Bearer auth as moonshot but different base URL
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		return base + "/models", headers

	case "zhipu", "zhipu-coding":
		// Zhipu uses Bearer auth with the API key directly
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
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
	return providerIndex[key]
}
