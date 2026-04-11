package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

type workspaceAgentResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
	Source      string `json:"source"` // "builtin" or "workspace"
}

func (h *Handler) registerWorkspaceAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workspace-agents", h.handleListWorkspaceAgents)
}

// handleListWorkspaceAgents returns all agent definitions from builtins + workspace.
//
//	GET /api/workspace-agents
func (h *Handler) handleListWorkspaceAgents(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, "Failed to load config", http.StatusInternalServerError)
		return
	}

	registry := agents.NewRegistry()
	_ = registry.LoadBuiltins()

	workspace := cfg.Agents.Defaults.Workspace
	if workspace == "" {
		workspace = filepath.Join(filepath.Dir(h.configPath), "workspace")
	}
	registry.Reload(filepath.Join(workspace, "agents"))

	defs := registry.List()
	result := make([]workspaceAgentResponse, 0, len(defs))
	for _, def := range defs {
		result = append(result, workspaceAgentResponse{
			Name:        def.Name,
			Description: def.Description,
			Model:       def.Model,
			Source:      def.Source,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"agents": result,
		"total":  len(result),
	})
}
