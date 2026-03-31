package struggles

import "time"

const (
	TypeToolError      = "tool_error"
	TypeUserCorrection = "user_correction"
	TypeRepeatedTool   = "repeated_tool"
)

// Signal represents a single struggle event detected during a conversation.
type Signal struct {
	Timestamp time.Time `json:"ts"`
	Type      string    `json:"type"`
	Tool      string    `json:"tool,omitempty"`
	Error     string    `json:"error,omitempty"`
	Session   string    `json:"session,omitempty"`
	Context   string    `json:"context,omitempty"`
	Count     int       `json:"count,omitempty"` // for repeated_tool signals
}
