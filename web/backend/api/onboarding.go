package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/discovery"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/workspace"
)

// embeddingInfo maps embedding choice keys to provider configuration.
type embeddingInfo struct {
	provider string
	model    string
	baseURL  string
}

var embeddingDefaults = map[string]embeddingInfo{
	"gemini": {
		provider: "gemini",
		model:    "text-embedding-004",
		baseURL:  "https://generativelanguage.googleapis.com/v1beta",
	},
	"openai": {
		provider: "openai",
		model:    "text-embedding-3-small",
		baseURL:  "https://api.openai.com/v1",
	},
}

// onboardingCompleteRequest is the payload for POST /api/onboarding/complete.
type onboardingCompleteRequest struct {
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	APIKey            string `json:"api_key"`
	APIBase           string `json:"api_base,omitempty"`
	UserName          string `json:"user_name"`
	ApprovalMode      string `json:"approval_mode"`
	TelegramEnabled   bool   `json:"telegram_enabled"`
	TelegramToken     string `json:"telegram_token"`
	EmbeddingProvider string `json:"embedding_provider"`
	EmbeddingAPIKey   string `json:"embedding_api_key"`
}

// discoverModelsRequest is the payload for POST /api/onboarding/discover-models.
type discoverModelsRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	APIBase  string `json:"api_base,omitempty"`
}

// registerOnboardingRoutes binds onboarding wizard endpoints to the ServeMux.
func (h *Handler) registerOnboardingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/onboarding/status", h.handleOnboardingStatus)
	mux.HandleFunc("GET /api/onboarding/providers", h.handleProviderList)
	mux.HandleFunc("POST /api/onboarding/discover-models", h.handleDiscoverModels)
	mux.HandleFunc("POST /api/onboarding/validate-key", h.handleValidateKey)
	mux.HandleFunc("POST /api/onboarding/complete", h.handleOnboardingComplete)
}

// handleOnboardingStatus returns whether the initial setup is complete.
func (h *Handler) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	completed := false
	if _, err := os.Stat(h.configPath); err == nil {
		completed = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"completed": completed})
}

// handleProviderList returns the full catalog of supported providers.
//
//	GET /api/onboarding/providers
func (h *Handler) handleProviderList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(discovery.Providers)
}

// handleDiscoverModels queries a provider's /models endpoint and returns available models.
//
//	POST /api/onboarding/discover-models
func (h *Handler) handleDiscoverModels(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req discoverModelsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Resolve API base from provider catalog if not explicitly provided.
	apiBase := req.APIBase
	if apiBase == "" {
		if prov := discovery.FindProvider(req.Provider); prov != nil {
			apiBase = prov.APIBase
		}
	}
	if apiBase == "" {
		http.Error(w, "api_base is required for this provider", http.StatusBadRequest)
		return
	}

	models, err := discovery.DiscoverModels(req.Provider, apiBase, req.APIKey)

	resp := map[string]any{"models": models}
	if err != nil {
		resp["error"] = err.Error()
		resp["models"] = []discovery.Model{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleValidateKey tests an API key by making a lightweight request to the provider.
//
//	POST /api/onboarding/validate-key
func (h *Handler) handleValidateKey(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		APIBase  string `json:"api_base,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.APIKey == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": "api_key is required",
		})
		return
	}

	// Resolve API base
	apiBase := req.APIBase
	if apiBase == "" {
		if prov := discovery.FindProvider(req.Provider); prov != nil {
			apiBase = prov.APIBase
		}
	}
	if apiBase == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": "cannot determine API base for provider",
		})
		return
	}

	// Use discovery to validate — if it returns models, the key works.
	models, discErr := discovery.DiscoverModels(req.Provider, apiBase, req.APIKey)
	valid := len(models) > 0
	resp := map[string]any{"valid": valid}
	if discErr != nil {
		resp["error"] = discErr.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleOnboardingComplete applies the onboarding configuration.
//
//	POST /api/onboarding/complete
func (h *Handler) handleOnboardingComplete(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req onboardingCompleteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.Provider == "" || req.Model == "" {
		http.Error(w, "provider and model are required", http.StatusBadRequest)
		return
	}

	// Resolve API base
	apiBase := req.APIBase
	if apiBase == "" {
		if prov := discovery.FindProvider(req.Provider); prov != nil {
			apiBase = prov.APIBase
		}
	}

	// Build the model string with protocol prefix
	protocol := req.Provider
	fullModel := protocol + "/" + req.Model
	// Avoid double-prefixing if user already included the protocol
	if strings.HasPrefix(req.Model, protocol+"/") {
		fullModel = req.Model
	}

	cfg := config.DefaultConfig()

	if req.ApprovalMode != "" {
		cfg.Agents.Defaults.ApprovalMode = req.ApprovalMode
	}

	newModel := &config.ModelConfig{
		ModelName: req.Model,
		Model:     fullModel,
		APIBase:   apiBase,
	}
	newModel.SetAPIKey(req.APIKey)

	cfg.ModelList = append([]*config.ModelConfig{newModel}, cfg.ModelList...)
	cfg.Agents.Defaults.Provider = req.Model

	if req.TelegramEnabled && req.TelegramToken != "" {
		cfg.Channels.Telegram.Enabled = true
		cfg.Channels.Telegram.SetToken(req.TelegramToken)
	}

	configureEmbeddings(cfg, req.EmbeddingProvider, req.EmbeddingAPIKey, req.APIKey, req.Provider, apiBase)

	configDir := filepath.Dir(h.configPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create config directory: %v", err), http.StatusInternalServerError)
		return
	}

	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	ws := cfg.WorkspacePath()
	if err := workspace.Deploy(ws, workspace.TemplateData{UserName: req.UserName}); err != nil {
		logger.Warn(fmt.Sprintf("failed to create workspace templates: %v", err))
	}

	logger.Infof("onboarding completed successfully for user %q with provider %q model %q", req.UserName, req.Provider, req.Model)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// configureEmbeddings sets up the embeddings section of the config based on the
// user's choice during onboarding.
func configureEmbeddings(cfg *config.Config, embChoice, embAPIKey, chatAPIKey, chatProvider, chatAPIBase string) {
	if embChoice == "same" || embChoice == "" {
		switch chatProvider {
		case "openai":
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			cfg.Embeddings.BaseURL = "https://api.openai.com/v1"
			cfg.Embeddings.APIKey = chatAPIKey
		case "anthropic", "openrouter":
			cfg.Embeddings.Provider = "gemini"
			cfg.Embeddings.Model = "text-embedding-004"
			cfg.Embeddings.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
			if embAPIKey != "" {
				cfg.Embeddings.APIKey = embAPIKey
			}
		default:
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			cfg.Embeddings.BaseURL = chatAPIBase
			cfg.Embeddings.APIKey = chatAPIKey
		}
		return
	}

	ei, ok := embeddingDefaults[embChoice]
	if !ok {
		return
	}
	cfg.Embeddings.Provider = ei.provider
	cfg.Embeddings.Model = ei.model
	cfg.Embeddings.BaseURL = ei.baseURL
	if embAPIKey != "" {
		cfg.Embeddings.APIKey = embAPIKey
	}
}
