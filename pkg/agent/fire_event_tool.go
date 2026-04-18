package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// fireEventTool lets a running turn resolve named waiters registered via
// await_event continuations. Useful for cross-agent signaling, self-notes
// that unblock a waiting turn, or synthetic triggers during testing.
type fireEventTool struct{}

func newFireEventTool() *fireEventTool { return &fireEventTool{} }

func (t *fireEventTool) Name() string { return "fire_event" }

func (t *fireEventTool) Description() string {
	return "Fire a named event, resuming any waiters registered via " +
		"await_event continuations. All waiters on the name resolve and " +
		"their next turn receives the payload you supply. No-op if no " +
		"waiters are pending."
}

func (t *fireEventTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Event name to fire. Case-insensitive; trimmed.",
			},
			"payload": map[string]any{
				"type":        "string",
				"description": "Optional short payload delivered to each waiter as part of their resumption context.",
			},
		},
		"required":             []string{"name"},
		"additionalProperties": false,
	}
}

func (t *fireEventTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	name, _ := args["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		err := fmt.Errorf("name is required")
		return tools.ErrorResult(err.Error()).WithError(err)
	}
	payload, _ := args["payload"].(string)

	al := AgentLoopFromContext(ctx)
	if al == nil {
		return tools.SilentResult("fire_event noted (no AgentLoop in context; nothing resumed).")
	}
	resolved := al.FireEvent(ctx, name, payload)
	return tools.SilentResult(fmt.Sprintf("Fired %q; resumed %d waiter(s).", name, resolved))
}
