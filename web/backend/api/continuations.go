package api

import (
	"encoding/json"
	"net/http"
	"time"
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
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}
	defer resp.Body.Close()

	var payload json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
