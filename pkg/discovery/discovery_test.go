package discovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseModelsResponse_OpenAIFormat(t *testing.T) {
	body := `{"data":[{"id":"gpt-4o","owned_by":"openai"},{"id":"gpt-4o-mini","owned_by":"openai"}]}`
	models, err := parseModelsResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", models[0].ID)
	}
}

func TestParseModelsResponse_GeminiFormat(t *testing.T) {
	body := `{"models":[{"name":"models/gemini-2.0-flash","displayName":"Gemini 2.0 Flash"},{"name":"models/gemini-pro","displayName":"Gemini Pro"}]}`
	models, err := parseModelsResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	// Should strip "models/" prefix
	for _, m := range models {
		if m.ID == "" || m.ID[:1] == "m" {
			t.Errorf("expected stripped ID, got %q", m.ID)
		}
	}
}

func TestParseModelsResponse_ArrayFormat(t *testing.T) {
	body := `[{"id":"llama-3-70b","owned_by":"meta"},{"id":"mixtral-8x7b","owned_by":"mistral"}]`
	models, err := parseModelsResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestParseModelsResponse_Empty(t *testing.T) {
	models, err := parseModelsResponse([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if models != nil {
		t.Errorf("expected nil for empty response, got %v", models)
	}
}

func TestDiscoverModels_Integration(t *testing.T) {
	// Mock server returning OpenAI-format models
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "model-a", "owned_by": "test"},
				{"id": "model-b", "owned_by": "test"},
			},
		})
	}))
	defer srv.Close()

	models, err := DiscoverModels(srv.URL, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestDiscoverModels_InvalidKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := DiscoverModels(srv.URL, "bad-key")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestDiscoverModels_NotSupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	models, err := DiscoverModels(srv.URL, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if models != nil {
		t.Errorf("expected nil for 404, got %v", models)
	}
}

func TestFindProvider(t *testing.T) {
	p := FindProvider("openai")
	if p == nil {
		t.Fatal("expected to find openai provider")
	}
	if p.Name != "OpenAI" {
		t.Errorf("expected OpenAI, got %s", p.Name)
	}

	if FindProvider("nonexistent") != nil {
		t.Error("expected nil for nonexistent provider")
	}
}

func TestProvidersCatalogNoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range Providers {
		if seen[p.Key] {
			t.Errorf("duplicate provider key: %s", p.Key)
		}
		seen[p.Key] = true
	}
}
