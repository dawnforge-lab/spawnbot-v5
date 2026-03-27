package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

// setupOnboardingTestEnv creates a temporary directory with SPAWNBOT_HOME set
// and returns a config path inside it (config.json does NOT exist yet).
func setupOnboardingTestEnv(t *testing.T) (configPath string, cleanup func()) {
	t.Helper()

	tmp := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldSpawnbotHome := os.Getenv("SPAWNBOT_HOME")

	if err := os.Setenv("HOME", tmp); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	if err := os.Setenv("SPAWNBOT_HOME", filepath.Join(tmp, ".spawnbot")); err != nil {
		t.Fatalf("set SPAWNBOT_HOME: %v", err)
	}

	configPath = filepath.Join(tmp, ".spawnbot", "config.json")

	cleanup = func() {
		_ = os.Setenv("HOME", oldHome)
		if oldSpawnbotHome == "" {
			_ = os.Unsetenv("SPAWNBOT_HOME")
		} else {
			_ = os.Setenv("SPAWNBOT_HOME", oldSpawnbotHome)
		}
	}
	return configPath, cleanup
}

func TestOnboardingStatus_NotCompleted(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["completed"] {
		t.Fatal("expected completed=false when config.json does not exist")
	}
}

func TestOnboardingStatus_Completed(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	// Create the config file so status reports completed
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.ModelList = []*config.ModelConfig{{
		ModelName: "test-model",
		Model:     "openai/gpt-4o",
	}}
	cfg.WithSecurity(&config.SecurityConfig{
		ModelList: map[string]config.ModelSecurityEntry{
			"test-model": {APIKeys: []string{"sk-test"}},
		},
	})
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp["completed"] {
		t.Fatal("expected completed=true when config.json exists")
	}
}

func TestOnboardingComplete_WritesConfig(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	payload := `{
		"provider": "openrouter",
		"model": "anthropic/claude-sonnet-4",
		"api_key": "sk-test-key-12345",
		"user_name": "TestUser",
		"approval_mode": "yolo",
		"telegram_enabled": false,
		"telegram_token": "",
		"embedding_provider": "same",
		"embedding_api_key": ""
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/complete", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp["success"] {
		t.Fatal("expected success=true")
	}

	// Verify config file was written
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.json was not created")
	}

	// Verify config contents
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Agents.Defaults.ApprovalMode != "yolo" {
		t.Fatalf("approval_mode = %q, want %q", cfg.Agents.Defaults.ApprovalMode, "yolo")
	}

	if cfg.Agents.Defaults.Provider != "anthropic/claude-sonnet-4" {
		t.Fatalf("provider = %q, want %q", cfg.Agents.Defaults.Provider, "anthropic/claude-sonnet-4")
	}

	// First model should be the user-selected one
	if len(cfg.ModelList) == 0 {
		t.Fatal("model_list is empty")
	}
	if cfg.ModelList[0].ModelName != "anthropic/claude-sonnet-4" {
		t.Fatalf("model_list[0].model_name = %q, want %q", cfg.ModelList[0].ModelName, "anthropic/claude-sonnet-4")
	}

	// Security file should exist
	secPath := filepath.Join(filepath.Dir(configPath), ".security.yml")
	if _, err := os.Stat(secPath); os.IsNotExist(err) {
		t.Fatal(".security.yml was not created")
	}
}

func TestOnboardingComplete_WithTelegram(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	payload := `{
		"provider": "openai",
		"model": "gpt-4o",
		"api_key": "sk-openai-key",
		"user_name": "Alice",
		"approval_mode": "approval",
		"telegram_enabled": true,
		"telegram_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		"embedding_provider": "same",
		"embedding_api_key": ""
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/complete", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if !cfg.Channels.Telegram.Enabled {
		t.Fatal("telegram should be enabled")
	}

	if cfg.Channels.Telegram.Token() == "" {
		t.Fatal("telegram token should be set")
	}
}

func TestOnboardingComplete_RejectsInvalidJSON(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/complete", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestOnboardingComplete_RejectsMissingProvider(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Missing both provider and model
	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/complete", bytes.NewBufferString(`{"api_key":"sk-test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestOnboardingComplete_WorkspaceCreated(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	payload := `{
		"provider": "openrouter",
		"model": "anthropic/claude-sonnet-4",
		"api_key": "sk-test-key",
		"user_name": "Bob",
		"approval_mode": "yolo",
		"telegram_enabled": false,
		"telegram_token": "",
		"embedding_provider": "same",
		"embedding_api_key": ""
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/complete", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	workspace := cfg.WorkspacePath()

	// Check workspace directory was created
	if _, err := os.Stat(workspace); os.IsNotExist(err) {
		t.Fatal("workspace directory was not created")
	}

	// Check SOUL.md was created
	soulPath := filepath.Join(workspace, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		t.Fatal("SOUL.md was not created in workspace")
	}

	// Check USER.md contains the user name
	userPath := filepath.Join(workspace, "USER.md")
	data, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatalf("read USER.md: %v", err)
	}
	if !bytes.Contains(data, []byte("Bob")) {
		t.Fatalf("USER.md does not contain user name 'Bob', got: %s", string(data))
	}
}

func TestOnboardingValidateKey_MissingFields(t *testing.T) {
	configPath, cleanup := setupOnboardingTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/validate-key", bytes.NewBufferString(`{"provider":"","api_key":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["valid"] != false {
		t.Fatal("expected valid=false for missing fields")
	}
}
