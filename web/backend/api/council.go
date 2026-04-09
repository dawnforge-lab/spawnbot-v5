package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/council"
)

// registerCouncilRoutes binds council list, detail, and delete endpoints to the ServeMux.
func (h *Handler) registerCouncilRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/councils", h.handleListCouncils)
	mux.HandleFunc("GET /api/councils/{id}", h.handleGetCouncil)
	mux.HandleFunc("DELETE /api/councils/{id}", h.handleDeleteCouncil)
}

// getCouncilStore returns the council store, initializing it lazily if needed.
func (h *Handler) getCouncilStore() (*council.Store, error) {
	if h.councilStore != nil {
		return h.councilStore, nil
	}

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(cfg.WorkspacePath(), "councils")
	h.councilStore = council.NewStore(dir)
	return h.councilStore, nil
}

// handleListCouncils returns a list of all council sessions.
//
//	GET /api/councils
func (h *Handler) handleListCouncils(w http.ResponseWriter, r *http.Request) {
	store, err := h.getCouncilStore()
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}

	metas, err := store.List()
	if err != nil {
		http.Error(w, "failed to list councils", http.StatusInternalServerError)
		return
	}

	if metas == nil {
		metas = []*council.CouncilMeta{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metas)
}

// handleGetCouncil returns the meta and transcript for a specific council session.
//
//	GET /api/councils/{id}
func (h *Handler) handleGetCouncil(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing council id", http.StatusBadRequest)
		return
	}

	store, err := h.getCouncilStore()
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}

	meta, err := store.Load(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "council not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to load council", http.StatusInternalServerError)
		}
		return
	}

	transcript, err := store.GetTranscript(id)
	if err != nil {
		http.Error(w, "failed to load transcript", http.StatusInternalServerError)
		return
	}

	if transcript == nil {
		transcript = []council.TranscriptEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":          meta.ID,
		"title":       meta.Title,
		"description": meta.Description,
		"roster":      meta.Roster,
		"status":      meta.Status,
		"rounds":      meta.Rounds,
		"created_at":  meta.CreatedAt,
		"updated_at":  meta.UpdatedAt,
		"transcript":  transcript,
	})
}

// handleDeleteCouncil deletes a specific council session.
//
//	DELETE /api/councils/{id}
func (h *Handler) handleDeleteCouncil(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing council id", http.StatusBadRequest)
		return
	}

	store, err := h.getCouncilStore()
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}

	if err := store.Delete(id); err != nil {
		http.Error(w, "failed to delete council", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
