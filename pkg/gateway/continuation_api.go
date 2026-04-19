package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agent"
)

type pendingContinuationResponse struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agent_id"`
	SessionKey string     `json:"session_key"`
	Kind       string     `json:"kind"`
	Intent     string     `json:"intent"`
	CreatedAt  time.Time  `json:"created_at"`
	FiresAt    *time.Time `json:"fires_at,omitempty"`
	EventName  string     `json:"event_name,omitempty"`
}

func makeContinuationsHandler(al *agent.AgentLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		agentID := r.PathValue("id")
		pending := al.GetPendingContinuations(agentID)
		resp := make([]pendingContinuationResponse, 0, len(pending))
		for _, pc := range pending {
			resp = append(resp, pendingContinuationResponse{
				ID:         pc.ID,
				AgentID:    pc.AgentID,
				SessionKey: pc.SessionKey,
				Kind:       string(pc.Kind),
				Intent:     pc.Intent,
				CreatedAt:  pc.CreatedAt,
				FiresAt:    pc.FiresAt,
				EventName:  pc.EventName,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
