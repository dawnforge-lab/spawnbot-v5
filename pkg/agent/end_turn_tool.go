package agent

import (
	"context"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// endTurnTool lets the model declare its continuation intent as the final
// action of a turn. Execution records the declared Continuation on the active
// turnState (via turnStateFromContext). The supervisor in runAgentLoop reads
// that Continuation after runTurn returns and dispatches the side effect
// (self-steer, timer, etc.).
type endTurnTool struct{}

func newEndTurnTool() *endTurnTool { return &endTurnTool{} }

func (t *endTurnTool) Name() string { return "end_turn" }

func (t *endTurnTool) Description() string {
	return "Finish the current turn and declare what should happen next. " +
		"Call this last, after any other tool calls and your user-facing " +
		"message, to either stop (done) or schedule a self-continuation " +
		"(continue_now / wait / schedule / await_event). Only the most " +
		"recent end_turn call in a turn takes effect. Use continue_now when " +
		"you want to immediately take another step without waiting for the " +
		"user. Use wait with after_ms to pause before resuming. Use " +
		"schedule with an RFC3339 at timestamp for a specific time. " +
		"Provide a concrete intent describing what the next turn should do."
}

func (t *endTurnTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"continuation": map[string]any{
				"type":        "string",
				"enum":        []string{"done", "continue_now", "wait", "schedule", "await_event"},
				"description": "What the supervisor should do after this turn ends.",
			},
			"intent": map[string]any{
				"type":        "string",
				"description": "Self-directed instruction describing what the next turn should do. Required when continuation is continue_now, wait, or schedule.",
			},
			"after_ms": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "For continuation=wait: delay in milliseconds before re-entering a new turn.",
			},
			"at": map[string]any{
				"type":        "string",
				"description": "For continuation=schedule: RFC3339 timestamp at which to re-enter. If both at and after_ms are provided, at wins.",
			},
			"event": map[string]any{
				"type":        "string",
				"description": "For continuation=await_event: name of the external event to wait for (placeholder; not yet wired).",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Short rationale for the chosen continuation. Surfaced in logs.",
			},
		},
		"required":             []string{"continuation"},
		"additionalProperties": false,
	}
}

func (t *endTurnTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	cont, err := parseContinuationArgs(args)
	if err != nil {
		return tools.ErrorResult(err.Error()).WithError(err)
	}

	ts := turnStateFromContext(ctx)
	if ts == nil {
		return tools.SilentResult("Continuation noted (no active turn to record it on).")
	}
	ts.setContinuation(cont)
	return tools.SilentResult("Continuation set: " + string(cont.Kind))
}
