package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

func (h *Handler) registerContinuationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agents/{id}/continuations", h.handleGetContinuations)
}

func (h *Handler) handleGetContinuations(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	base := h.gatewayProxyURL()
	gatewayURL := base.String() + "/agents/" + agentID + "/continuations"

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(gatewayURL)
	if err != nil {
		logger.WarnCF("api", "Gateway unreachable fetching continuations",
			map[string]any{"agent_id": agentID, "error": err.Error()})
		http.Error(w, "gateway unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.WarnCF("api", "Gateway returned non-2xx fetching continuations",
			map[string]any{"agent_id": agentID, "status": resp.StatusCode})
		http.Error(w, fmt.Sprintf("gateway error: %s", resp.Status), http.StatusBadGateway)
		return
	}

	var payload json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		logger.WarnCF("api", "Failed to decode continuations response",
			map[string]any{"agent_id": agentID, "error": err.Error()})
		http.Error(w, "invalid response from gateway", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
