# Autonomy V2 Design

**Goal:** Improve the `end_turn` continuation system from MVP to production-ready across three areas: backend config hardening, chat UI visibility, and pending-continuation observability.

**Architecture:** Reuse existing patterns — tool call rendering in chat, REST polling for state, in-memory tracking with shutdown warnings. No new event infrastructure.

**Tech Stack:** Go (backend), React/TypeScript (web/frontend), existing tool call UI components.

---

## Group A: Backend Config & Tool Discovery

### Configurable depth cap

Add `MaxAutoContinueDepth int` to `AgentDefaults` in `pkg/config/config.go`:

```go
MaxAutoContinueDepth int `json:"max_auto_continue_depth,omitempty" env:"SPAWNBOT_AGENTS_DEFAULTS_MAX_AUTO_CONTINUE_DEPTH"`
```

Add accessor with fallback:

```go
func (d *AgentDefaults) GetMaxAutoContinueDepth() int {
    if d.MaxAutoContinueDepth > 0 {
        return d.MaxAutoContinueDepth
    }
    return 5
}
```

In `pkg/agent/continuation.go`, replace the hardcoded `maxAutoContinueDepth` constant with `al.config.defaults.GetMaxAutoContinueDepth()`. Delete the constant.

Expose in Agent Config UI alongside `max_tool_iterations`.

### Auto-promotion of end_turn

In `pkg/agent/loop.go`, immediately after registering the three infrastructure tools as hidden, promote them:

```go
agent.Tools.RegisterHidden(newEndTurnTool())
agent.Tools.RegisterHidden(newFireEventTool())
agent.Tools.RegisterHidden(newListEventsTool())
agent.Tools.PromoteTools([]string{"end_turn", "fire_event", "list_events"})
```

This removes the `search_tools` discovery round-trip. These are infrastructure tools — they should always be available, not gated behind discovery.

---

## Group B: Continuation Tracking & REST API

### In-memory tracker

Add to `pkg/agent/continuation.go`:

```go
type PendingContinuation struct {
    ID         string           // random UUID
    AgentID    string
    SessionKey string
    Kind       ContinuationKind // wait, schedule, await_event
    Intent     string
    CreatedAt  time.Time
    FiresAt    *time.Time       // set for schedule kind
    EventName  string           // set for await_event kind
}
```

`AgentLoop` gains a `pendingConts sync.Map` field. `dispatchContinuation` registers a `PendingContinuation` when starting a goroutine for `wait`/`schedule`/`await_event`, and removes it on completion or cancellation.

On `Stop()`, log a warning if any entries remain:

```go
al.pendingConts.Range(func(_, v any) bool {
    pc := v.(*PendingContinuation)
    logger.WarnCF("agent", "Dropping pending continuation on shutdown",
        map[string]any{"kind": pc.Kind, "intent": pc.Intent, "agent_id": pc.AgentID})
    return true
})
```

`AgentLoop` exposes:

```go
func (al *AgentLoop) GetPendingContinuations(agentID string) []PendingContinuation
```

### REST endpoint

New endpoint in `web/backend/api/`: `GET /api/agents/{id}/continuations`

Returns:
```json
[
  {
    "id": "uuid",
    "kind": "schedule",
    "intent": "Check feed in 5 min",
    "created_at": "2026-04-19T14:00:00Z",
    "fires_at": "2026-04-19T14:05:00Z",
    "event_name": ""
  }
]
```

Returns `[]` when no continuations are pending. The web server calls `agentLoop.GetPendingContinuations(agentID)`.

---

## Group C: UI — Chat Visibility

### end_turn as visible tool call row

When the frontend receives a tool call row where `tool_name === "end_turn"`:

**Collapsed state:**
```
↻ Agent continued — continue_now: "Monitoring feed"
```

**Expanded state:**
```
end_turn
  continuation: continue_now
  intent:       "Monitoring feed"
  reason:       "User asked me to monitor and report back"
  timestamp:    14:02:31
```

The `[self-continue]` prefix is stripped from the steering message before display in the continuation turn — the `end_turn` row itself is the visual separator between turns.

No new backend work required — tool calls already flow through the message stream. Only frontend rendering changes in the tool call component.

---

## Group D: UI — Pending Badge & Panel

### Sidebar badge

While polling `GET /api/agents/{id}/continuations` (every 5s), if the response is non-empty, show a small animated dot badge on the agent's sidebar row.

### Pending panel

Clicking the badge opens a slide-over panel (overlay anchored to sidebar, not a new route):

```
Pending Continuations — main agent
────────────────────────────────────
↻  schedule     "Check feed in 5 min"     fires in 4:32
⏳  await_event  "Waiting for user reply"  no deadline
────────────────────────────────────
```

Columns: kind icon, intent, time remaining (or "no deadline").

Polling: every 2s while panel is open, stops when closed. Read-only in v1 — no cancel action.

---

## Group E: Integration Tests

File: `pkg/agent/continuation_integration_test.go`

Uses the existing mock LLM pattern from the codebase.

### Test 1: `TestContinuation_ContinueNow`

Mock LLM sequence:
- Turn 1: returns `end_turn(continue_now, intent="test")`
- Turn 2 (injected): returns `end_turn(done)`

Assert:
- Two turns fired
- Turn 2 context contains `[self-continue]` steering message
- Depth counter resets after done

### Test 2: `TestContinuation_DepthCapEnforced`

Mock LLM always returns `end_turn(continue_now)`.

Assert:
- Agent stops after exactly `MaxAutoContinueDepth` continuation turns
- No goroutine leak after stop

### Test 3: `TestContinuation_ShutdownDrainsPending`

Setup: Start a `schedule` continuation with a 1-hour timer, call `Stop()` immediately.

Assert:
- Warning log emitted listing the dropped continuation
- No goroutine leak (verified with `goleak` or equivalent)

---

## File Map

**Modified:**
- `pkg/config/config.go` — add `MaxAutoContinueDepth` to `AgentDefaults`
- `pkg/agent/continuation.go` — add `PendingContinuation`, tracker, `GetPendingContinuations`, shutdown warning
- `pkg/agent/loop.go` — auto-promote infrastructure tools; use configurable depth cap
- `web/backend/api/` — add `GET /api/agents/{id}/continuations` handler
- `web/frontend/src/` — tool call component (`end_turn` rendering), sidebar badge, pending panel

**Created:**
- `pkg/agent/continuation_integration_test.go`
- `web/frontend/src/features/continuations/` — pending panel component
