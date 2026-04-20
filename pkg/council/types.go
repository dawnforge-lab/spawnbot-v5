package council

import "time"

// Status constants for council sessions.
const (
	StatusActive = "active"
	StatusPaused = "paused"
	StatusClosed = "closed"
)

// Role constants for transcript entries.
const (
	RoleSystem    = "system"
	RoleAgent     = "agent"
	RoleModerator = "moderator"
	RoleUser      = "user"
	RoleSynthesis = "synthesis"
)

// CouncilMeta holds the metadata for a council session.
type CouncilMeta struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	AgentContext string    `json:"agent_context,omitempty"` // Brief intro from the invoking agent
	Roster       []string  `json:"roster"`
	Status       string    `json:"status"`
	Rounds       int       `json:"rounds"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TranscriptEntry represents a single message in the council transcript.
type TranscriptEntry struct {
	Role      string    `json:"role"`
	AgentID   string    `json:"agent_id,omitempty"`
	AgentType string    `json:"agent_type,omitempty"`
	Content   string    `json:"content"`
	Round     int       `json:"round"`
	Timestamp time.Time `json:"timestamp"`
}

// CouncilConfig holds the configuration for creating a new council.
type CouncilConfig struct {
	ID           string   `json:"id,omitempty"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	Roster       []string `json:"roster"`
	Model        string   `json:"model,omitempty"`
	AgentContext string   `json:"agent_context,omitempty"` // Brief intro from the invoking agent
}

// CouncilTask is a single action item extracted from the council synthesis.
type CouncilTask struct {
	Agent    string `json:"agent"`
	Task     string `json:"task"`
	Priority string `json:"priority,omitempty"`
}

// CouncilResult holds the result summary of a council session.
type CouncilResult struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Rounds    int           `json:"rounds"`
	Synthesis string        `json:"synthesis,omitempty"`
	Tasks     []CouncilTask `json:"tasks,omitempty"`
	Status    string        `json:"status"`
}
