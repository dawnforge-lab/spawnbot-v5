package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// CouncilRunner abstracts council.Engine.Run to break the import cycle
// (tools -> council -> agent -> memory -> tools).
type CouncilRunner interface {
	Run(ctx context.Context, cfg CouncilRunConfig) (*CouncilRunResult, error)
}

// CouncilLister abstracts council.Store.List to break the import cycle.
type CouncilLister interface {
	List() ([]*CouncilMetaSummary, error)
}

// CouncilRunConfig mirrors council.CouncilConfig without importing the package.
type CouncilRunConfig struct {
	ID           string   `json:"id,omitempty"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	Roster       []string `json:"roster"`
	AgentContext string   `json:"agent_context,omitempty"` // Brief self-introduction from the invoking agent
}

// CouncilRunResult mirrors council.CouncilResult without importing the package.
type CouncilRunResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Rounds    int    `json:"rounds"`
	Synthesis string `json:"synthesis,omitempty"`
	Status    string `json:"status"`
}

// CouncilMetaSummary mirrors council.CouncilMeta fields needed for listing.
type CouncilMetaSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Rounds int    `json:"rounds"`
}

// CouncilTool wraps the council engine for the main agent tool interface.
type CouncilTool struct {
	engine CouncilRunner
	lister CouncilLister
}

// NewCouncilTool creates a new CouncilTool backed by the given engine.
func NewCouncilTool(engine CouncilRunner) *CouncilTool {
	return &CouncilTool{engine: engine}
}

// SetEngine sets or replaces the council engine (for deferred setup).
func (t *CouncilTool) SetEngine(engine CouncilRunner) {
	t.engine = engine
}

// SetLister sets the store lister (for list operations without an engine).
func (t *CouncilTool) SetLister(lister CouncilLister) {
	t.lister = lister
}

func (t *CouncilTool) Name() string { return "council" }

func (t *CouncilTool) Description() string {
	return "Start, resume, or list multi-agent council discussions. A council convenes multiple specialist agents to deliberate on a topic and produce a synthesized result."
}

func (t *CouncilTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"start", "resume", "list"},
				"description": "Action to perform: start a new council, resume an existing one, or list all councils.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Title for the new council (required for start).",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Description of the council's purpose.",
			},
			"topic": map[string]any{
				"type":        "string",
				"description": "The topic or question to discuss.",
			},
			"roster": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of agent names to participate in the council.",
			},
			"agent_context": map[string]any{
				"type":        "string",
				"description": "A brief self-introduction (1-2 sentences) so council agents understand who is asking. Include your name, role, and relevant context for the discussion.",
			},
			"council_id": map[string]any{
				"type":        "string",
				"description": "Council ID to resume (required for resume).",
			},
		},
		"required": []string{"action"},
	}
}

func (t *CouncilTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	switch action {
	case "list":
		return t.list()
	case "start":
		return t.start(ctx, args)
	case "resume":
		return t.resume(ctx, args)
	default:
		return ErrorResult(fmt.Sprintf("unknown action %q; valid actions: start, resume, list", action))
	}
}

func (t *CouncilTool) list() *ToolResult {
	if t.lister == nil {
		return ErrorResult("council store not configured")
	}

	metas, err := t.lister.List()
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to list councils: %s", err))
	}

	if len(metas) == 0 {
		return &ToolResult{
			ForUser: "No councils found.",
			ForLLM:  `{"councils":[]}`,
		}
	}

	// ForUser: human-readable list
	var lines string
	for i, m := range metas {
		if i > 0 {
			lines += "\n"
		}
		lines += fmt.Sprintf("- [%s] %s (%s, %d rounds)", m.ID, m.Title, m.Status, m.Rounds)
	}
	forUser := fmt.Sprintf("%d council(s):\n%s", len(metas), lines)

	// ForLLM: JSON
	data, _ := json.Marshal(map[string]any{"councils": metas})
	return &ToolResult{
		ForUser: forUser,
		ForLLM:  string(data),
	}
}

func (t *CouncilTool) start(ctx context.Context, args map[string]any) *ToolResult {
	if t.engine == nil {
		return ErrorResult("council engine not configured")
	}

	title, _ := args["title"].(string)
	if title == "" {
		return ErrorResult("title is required for start action")
	}

	description, _ := args["description"].(string)
	topic, _ := args["topic"].(string)

	var roster []string
	if rosterRaw, ok := args["roster"].([]any); ok {
		for _, v := range rosterRaw {
			if s, ok := v.(string); ok {
				roster = append(roster, s)
			}
		}
	}

	agentContext, _ := args["agent_context"].(string)

	cfg := CouncilRunConfig{
		Title:        title,
		Description:  description,
		Topic:        topic,
		Roster:       roster,
		AgentContext: agentContext,
	}

	result, err := t.engine.Run(ctx, cfg)
	if err != nil {
		return ErrorResult(fmt.Sprintf("council failed: %s", err))
	}

	return t.formatResult(result)
}

func (t *CouncilTool) resume(ctx context.Context, args map[string]any) *ToolResult {
	if t.engine == nil {
		return ErrorResult("council engine not configured")
	}

	councilID, _ := args["council_id"].(string)
	if councilID == "" {
		return ErrorResult("council_id is required for resume action")
	}

	topic, _ := args["topic"].(string)

	cfg := CouncilRunConfig{
		ID:    councilID,
		Topic: topic,
	}

	result, err := t.engine.Run(ctx, cfg)
	if err != nil {
		return ErrorResult(fmt.Sprintf("council resume failed: %s", err))
	}

	return t.formatResult(result)
}

func (t *CouncilTool) formatResult(result *CouncilRunResult) *ToolResult {
	forUser := fmt.Sprintf("Council %q completed (%d rounds).\n\nSynthesis:\n%s",
		result.Title, result.Rounds, result.Synthesis)

	data, _ := json.Marshal(result)
	return &ToolResult{
		ForUser: forUser,
		ForLLM:  string(data),
	}
}
