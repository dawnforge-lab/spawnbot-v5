package council

// EventKind identifies the type of council event.
type EventKind int

const (
	EventCouncilStart EventKind = iota
	EventCouncilRoundStart
	EventCouncilAgentStart
	EventCouncilAgentDelta
	EventCouncilAgentEnd
	EventCouncilRoundEnd
	EventCouncilEnd
)

// Event is a council-scoped event emitted during a council session.
type Event struct {
	Kind    EventKind
	Payload any
}

// EventEmitter is the interface the engine uses to broadcast events.
// The agent package provides a concrete adapter that maps these to agent.Event.
type EventEmitter interface {
	EmitCouncilEvent(evt Event)
}

// Council event payload types.

type CouncilStartPayload struct {
	CouncilID   string   `json:"council_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Roster      []string `json:"roster"`
}

type CouncilRoundStartPayload struct {
	CouncilID string `json:"council_id"`
	Round     int    `json:"round"`
}

type CouncilAgentStartPayload struct {
	CouncilID string `json:"council_id"`
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
	Round     int    `json:"round"`
}

type CouncilAgentDeltaPayload struct {
	CouncilID string `json:"council_id"`
	AgentID   string `json:"agent_id"`
	Delta     string `json:"delta"`
}

type CouncilAgentEndPayload struct {
	CouncilID string `json:"council_id"`
	AgentID   string `json:"agent_id"`
	Content   string `json:"content"`
	Round     int    `json:"round"`
}

type CouncilRoundEndPayload struct {
	CouncilID string `json:"council_id"`
	Round     int    `json:"round"`
	Decision  string `json:"decision"`
}

type CouncilEndPayload struct {
	CouncilID string `json:"council_id"`
	Rounds    int    `json:"rounds"`
	Synthesis string `json:"synthesis"`
	Status    string `json:"status"`
}
