package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

// providerInfo holds the configuration details for a chosen LLM provider.
type providerInfo struct {
	modelName string
	model     string
	apiBase   string
}

// providerDefaults maps provider selection keys to their default model config.
var providerDefaults = map[string]providerInfo{
	"openrouter": {
		modelName: "anthropic/claude-sonnet-4",
		model:     "openrouter/anthropic/claude-sonnet-4",
		apiBase:   "https://openrouter.ai/api/v1",
	},
	"anthropic": {
		modelName: "claude-sonnet-4-20250514",
		model:     "anthropic/claude-sonnet-4-20250514",
		apiBase:   "https://api.anthropic.com/v1",
	},
	"openai": {
		modelName: "gpt-4o",
		model:     "openai/gpt-4o",
		apiBase:   "https://api.openai.com/v1",
	},
	"custom": {
		modelName: "custom-model",
		model:     "openai/custom-model",
		apiBase:   "http://localhost:8080/v1",
	},
}

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
	APIKey            string `json:"api_key"`
	UserName          string `json:"user_name"`
	ApprovalMode      string `json:"approval_mode"`
	TelegramEnabled   bool   `json:"telegram_enabled"`
	TelegramToken     string `json:"telegram_token"`
	EmbeddingProvider string `json:"embedding_provider"`
	EmbeddingAPIKey   string `json:"embedding_api_key"`
	CustomBaseURL     string `json:"custom_base_url,omitempty"`
}

// validateKeyRequest is the payload for POST /api/onboarding/validate-key.
type validateKeyRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
}

// registerOnboardingRoutes binds onboarding wizard endpoints to the ServeMux.
func (h *Handler) registerOnboardingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/onboarding/status", h.handleOnboardingStatus)
	mux.HandleFunc("POST /api/onboarding/validate-key", h.handleValidateKey)
	mux.HandleFunc("POST /api/onboarding/complete", h.handleOnboardingComplete)
}

// handleOnboardingStatus returns whether the initial setup is complete.
//
//	GET /api/onboarding/status
func (h *Handler) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	completed := false
	if _, err := os.Stat(h.configPath); err == nil {
		completed = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"completed": completed})
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

	var req validateKeyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.Provider == "" || req.APIKey == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": "provider and api_key are required",
		})
		return
	}

	pi, ok := providerDefaults[req.Provider]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": fmt.Sprintf("unknown provider: %s", req.Provider),
		})
		return
	}

	valid, validationErr := validateAPIKey(pi.apiBase, req.APIKey)

	resp := map[string]any{"valid": valid}
	if validationErr != "" {
		resp["error"] = validationErr
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// validateAPIKey makes a lightweight GET /models request to check if an API key
// is accepted by the provider. Returns (valid, errorMessage).
func validateAPIKey(apiBase, apiKey string) (bool, string) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", apiBase+"/models", nil)
	if err != nil {
		return false, fmt.Sprintf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	// Drain body so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == http.StatusOK {
		return true, ""
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, "invalid API key"
	}

	// Some providers return 404 for /models but the key may still be valid.
	// Accept any non-auth error as "probably valid".
	if resp.StatusCode == http.StatusNotFound {
		return true, ""
	}

	return false, fmt.Sprintf("unexpected status: %d", resp.StatusCode)
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

	if req.Provider == "" || req.APIKey == "" {
		http.Error(w, "provider and api_key are required", http.StatusBadRequest)
		return
	}

	pi, ok := providerDefaults[req.Provider]
	if !ok && req.Provider != "custom" {
		http.Error(w, fmt.Sprintf("unknown provider: %s", req.Provider), http.StatusBadRequest)
		return
	}
	if req.Provider == "custom" {
		pi = providerDefaults["custom"]
		if req.CustomBaseURL != "" {
			pi.apiBase = req.CustomBaseURL
		}
	}

	// Build the configuration
	cfg := config.DefaultConfig()

	// Set approval mode
	if req.ApprovalMode != "" {
		cfg.Agents.Defaults.ApprovalMode = req.ApprovalMode
	}

	// Set up the selected provider model entry
	newModel := &config.ModelConfig{
		ModelName: pi.modelName,
		Model:     pi.model,
		APIBase:   pi.apiBase,
	}
	newModel.SetAPIKey(req.APIKey)

	// Prepend the user's chosen model so it becomes the default (first entry)
	cfg.ModelList = append([]*config.ModelConfig{newModel}, cfg.ModelList...)

	// Set the default agent to use this model
	cfg.Agents.Defaults.Provider = pi.modelName

	// Configure Telegram if requested
	if req.TelegramEnabled && req.TelegramToken != "" {
		cfg.Channels.Telegram.Enabled = true
		cfg.Channels.Telegram.SetToken(req.TelegramToken)
	}

	// Configure embeddings
	configureEmbeddings(cfg, req.EmbeddingProvider, req.EmbeddingAPIKey, req.APIKey, req.Provider)

	// Ensure config directory exists
	configDir := filepath.Dir(h.configPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create config directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Save config (writes both config.json and .security.yml)
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	// Create workspace templates
	workspace := cfg.WorkspacePath()
	if err := createWorkspaceDir(workspace, req.UserName); err != nil {
		logger.Warn(fmt.Sprintf("failed to create workspace templates: %v", err))
		// Non-fatal — config was saved successfully
	}

	logger.Infof("onboarding completed successfully for user %q with provider %q", req.UserName, req.Provider)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// configureEmbeddings sets up the embeddings section of the config based on the
// user's choice during onboarding. Matches the CLI onboard logic.
func configureEmbeddings(cfg *config.Config, embChoice, embAPIKey, chatAPIKey, chatProvider string) {
	if embChoice == "same" || embChoice == "" {
		switch chatProvider {
		case "openai":
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			cfg.Embeddings.BaseURL = "https://api.openai.com/v1"
		case "anthropic", "openrouter":
			// Neither provides embeddings; fall back to Gemini defaults.
			cfg.Embeddings.Provider = "gemini"
			cfg.Embeddings.Model = "text-embedding-004"
			cfg.Embeddings.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
			return // no API key available
		default:
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			if pi, ok := providerDefaults[chatProvider]; ok {
				cfg.Embeddings.BaseURL = pi.apiBase
			}
		}
		cfg.Embeddings.APIKey = chatAPIKey
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

// createWorkspaceDir creates the workspace directory and writes the default
// template files (SOUL.md, USER.md, etc.) with the user's name substituted.
func createWorkspaceDir(workspace, userName string) error {
	if userName == "" {
		userName = "friend"
	}

	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create subdirectories
	for _, sub := range []string{"memory", "skills"} {
		if err := os.MkdirAll(filepath.Join(workspace, sub), 0o755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", sub, err)
		}
	}

	// Write SOUL.md if it doesn't exist
	soulPath := filepath.Join(workspace, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		content := fmt.Sprintf("# Spawnbot\n\nYou are Spawnbot, a personal AI assistant for %s.\n\nBe helpful, concise, and proactive.\n", userName)
		if err := os.WriteFile(soulPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write SOUL.md: %w", err)
		}
	}

	// Write USER.md if it doesn't exist
	userPath := filepath.Join(workspace, "USER.md")
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		content := fmt.Sprintf("# User Profile\n\nName: %s\n", userName)
		if err := os.WriteFile(userPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write USER.md: %w", err)
		}
	}

	// Write GOALS.md if it doesn't exist
	goalsPath := filepath.Join(workspace, "GOALS.md")
	if _, err := os.Stat(goalsPath); os.IsNotExist(err) {
		content := "# Goals\n\n## Active\n<!-- Spawnbot will track your objectives here -->\n\n## Completed\n<!-- Finished goals move here -->\n"
		if err := os.WriteFile(goalsPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write GOALS.md: %w", err)
		}
	}

	// Write PLAYBOOK.md if it doesn't exist
	playbookPath := filepath.Join(workspace, "PLAYBOOK.md")
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		content := "# Playbook\n\n## Communication Style\n- Be direct and concise\n- Lead with the answer, not the reasoning\n- Ask for clarification when instructions are ambiguous\n\n## Tool Usage\n- Always use tools when action is needed — never pretend to do something\n- Use memory_store when learning something worth remembering\n- Use memory_search before answering questions that might be in memory\n\n## Autonomy\n- Check GOALS.md when idle to find proactive work\n- Notify the user of important feed updates\n- Store interesting observations in memory for future reference\n"
		if err := os.WriteFile(playbookPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write PLAYBOOK.md: %w", err)
		}
	}

	// Write HEARTBEAT.md if it doesn't exist
	heartbeatPath := filepath.Join(workspace, "HEARTBEAT.md")
	if _, err := os.Stat(heartbeatPath); os.IsNotExist(err) {
		content := "# Heartbeat\n\n## Idle Triggers\nWhen idle for extended periods, check:\n- GOALS.md for pending objectives\n- Recent memory for follow-ups\n- Feed updates that need attention\n\n## Proactive Behaviors\n- Summarize important feed items for the user\n- Flag upcoming deadlines from GOALS.md\n- Offer help when context suggests the user might need it\n"
		if err := os.WriteFile(heartbeatPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write HEARTBEAT.md: %w", err)
		}
	}

	return nil
}
