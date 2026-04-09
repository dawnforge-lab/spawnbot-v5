# Council Feature Design Spec

A persistent "boardroom" where the main agent can convene specialist agents for collaborative planning discussions. Council sessions are saved with metadata (title, description, roster, status), can be reopened and continued, and are viewable live in the web UI with token-by-token streaming.

## Interaction Model

- **Always-on advisory board**: The main agent can autonomously decide to convene a council when facing a complex planning task. The user can also explicitly request a council.
- **Open floor**: A standing roster of agents (all registered non-heartbeat agents by default). Each round, every roster member speaks once. Agents see the full shared transcript and self-select whether they have something substantive to contribute.
- **Convergence-based**: The main agent moderates and decides when the discussion has produced enough value. No fixed round limit — runs until the moderator determines consensus or sufficient coverage.
- **Persistent and continuable**: Council sessions are saved and can be reopened. The main agent or user can revisit a council and agents pick up where they left off.

## Architecture

### Council Tool (`pkg/tools/council.go`)

New tool available to the main agent. Parameters:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | `"start"`, `"resume"`, or `"list"` |
| `title` | string | for start | Council session name |
| `description` | string | for start | What this council is about |
| `topic` | string | for start/resume | Opening prompt or new agenda item |
| `roster` | string[] | no | Agent types to include (default: all non-heartbeat) |
| `council_id` | string | for resume | ID of council to reopen |

The tool is synchronous — it blocks the main agent's turn while the council runs. The main agent receives the council result (synthesis) when it completes.

Return value: `ForUser` contains a summary of the council outcome. `ForLLM` contains the full synthesis plus metadata (council_id, rounds, participants) so the main agent can reference or resume it later.

### Council Engine (`pkg/council/engine.go`)

New package with its own orchestration loop. Calls LLM providers directly rather than spawning subturns — avoids subturn lifecycle complexity, depth limits, and parent-child coupling. The engine receives a `providers.Provider` (or provider factory) from the council tool at invocation time, using the same provider the agent loop uses.

**Round loop:**

1. For each agent in roster:
   - Resolve agent definition from registry (system prompt, model, tool access)
   - Build messages: agent-specific system prompt + full shared transcript as conversation history
   - Call LLM provider directly
   - Stream response tokens, emitting `council_agent_delta` events
   - Append completed response to shared transcript
   - Emit `council_agent_end` event
2. After all agents speak, emit `council_round_end`
3. Moderator assessment: lightweight LLM call with the transcript asking the main agent's model to decide "continue or conclude?" with structured output
4. If continue: go to step 1. If conclude: generate synthesis from transcript, emit `council_end`

**User interjection**: If a user message arrives during a live council (via bus), it is appended to the transcript as a `user` role message before the next agent speaks.

**Configuration:**

```go
type CouncilConfig struct {
    ID          string            // auto-generated for new, provided for resume
    Title       string
    Description string
    Topic       string            // opening prompt / agenda
    Roster      []string          // agent type names
    Model       string            // moderator model (inherited from main agent)
}
```

**Result:**

```go
type CouncilResult struct {
    ID        string
    Title     string
    Rounds    int
    Synthesis string   // moderator's final summary
    Status    string   // "closed" after normal end
}
```

### Council Session Persistence (`pkg/council/store.go`)

Stored in `workspace/councils/`:

```
workspace/councils/
  {council_id}/
    meta.json          # session metadata
    transcript.jsonl   # shared message history
```

**Meta schema (`meta.json`):**

```json
{
  "id": "council-abc123",
  "title": "API Redesign Strategy",
  "description": "Plan the v3 API migration with backwards compat",
  "roster": ["researcher", "coder", "planner"],
  "status": "active",
  "rounds": 0,
  "created_at": "2026-04-09T20:00:00Z",
  "updated_at": "2026-04-09T20:05:00Z"
}
```

Status values: `"active"` (council running), `"paused"` (council stopped, can resume), `"closed"` (council concluded with synthesis).

**Transcript JSONL entries:**

Each line is one message in the shared transcript:

```json
{"role": "system", "content": "Council started: API Redesign Strategy", "round": 0, "ts": "..."}
{"role": "agent", "agent_id": "researcher", "agent_type": "researcher", "content": "Based on my analysis...", "round": 1, "ts": "..."}
{"role": "agent", "agent_id": "coder", "agent_type": "coder", "content": "I agree with versioning...", "round": 1, "ts": "..."}
{"role": "agent", "agent_id": "planner", "agent_type": "planner", "content": "Breaking into phases...", "round": 1, "ts": "..."}
{"role": "moderator", "content": "Continue — need more detail on auth migration.", "round": 1, "ts": "..."}
{"role": "user", "content": "What about OAuth2?", "round": 2, "ts": "..."}
{"role": "agent", "agent_id": "researcher", "agent_type": "researcher", "content": "OAuth2 migration would...", "round": 2, "ts": "..."}
{"role": "synthesis", "content": "Final plan: ...", "round": 2, "ts": "..."}
```

**Resume**: Loads full transcript from JSONL. The new `topic` is appended as a user message. Agents see all prior context when they speak.

### Event Emission

The council engine emits events through the existing `EventBus` for live view and observability:

| Event Kind | Payload | When |
|------------|---------|------|
| `council_start` | id, title, description, roster | Council session created |
| `council_round_start` | council_id, round | New round begins |
| `council_agent_start` | council_id, agent_id, agent_type, round | Agent about to speak |
| `council_agent_delta` | council_id, agent_id, content_delta | Streaming token chunk |
| `council_agent_end` | council_id, agent_id, content, round | Agent finished speaking |
| `council_round_end` | council_id, round, decision | Round complete, moderator decision |
| `council_end` | council_id, rounds, synthesis, status | Council concluded |

New event kinds added to `pkg/agent/events.go`.

### Web Backend API (`web/backend/api/council.go`)

REST endpoints for council session management:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/councils` | List all council sessions (meta only, sorted by updated_at desc) |
| `GET` | `/api/councils/{id}` | Get council meta + full transcript |
| `DELETE` | `/api/councils/{id}` | Delete a council session |

**Live streaming via Pico WebSocket:**

New Pico protocol message types (server → client):

| Type | Payload | Purpose |
|------|---------|---------|
| `council.start` | `{id, title, description, roster}` | Council session begun |
| `council.agent.start` | `{council_id, agent_id, agent_type, round}` | Agent about to speak |
| `council.agent.delta` | `{council_id, agent_id, delta}` | Streaming token chunk |
| `council.agent.end` | `{council_id, agent_id, content, round}` | Agent message complete |
| `council.round.end` | `{council_id, round, decision}` | Round complete |
| `council.end` | `{council_id, rounds, synthesis}` | Council concluded |

The gateway subscribes to EventBus council events and broadcasts them to all Pico WebSocket connections.

### Web Frontend

**New route: `/councils`** — Council list page:
- Card/row per council session showing title, description, status badge, roster agent tags, round count, timestamps
- Status indicators: green dot = active, orange = paused, grey = closed
- Click to open transcript view
- "New Council" button (opens dialog with title, description, topic, roster selection → sends command through Pico to trigger the council tool)

**New route: `/councils/{id}`** — Council transcript view:
- Chat-style linear transcript with agent-colored left borders
- Agent name badge above each message, colored by agent type
- Round separator bars between rounds
- Live streaming: when council is active, tokens stream in for the current speaker
- Typing indicator showing which agent is currently speaking (e.g., "Researcher is thinking...")
- User interjection input at the bottom
- Header: title, description, status badge, roster tags, round counter
- Auto-scroll to bottom during live council

**Jotai state additions** (`store/council.ts`):
- `councilListAtom` — list of council meta objects
- `activeCouncilAtom` — currently viewed council transcript + streaming state
- `councilStreamAtom` — current streaming agent, accumulated delta content

**Pico protocol handler additions** (`features/chat/protocol.ts`):
- Handle `council.*` message types
- Update council Jotai atoms on each event

### Pico Streaming Support

As part of this feature, implement `BeginStream` on the Pico channel (`pkg/channels/pico/pico.go`):

- Pico channel implements `StreamingCapable` interface
- `BeginStream(ctx, chatID)` returns a `picoStreamer`
- `picoStreamer.Update(ctx, content)` broadcasts `message.update` with accumulated content to the session's WebSocket connections
- `picoStreamer.Finalize(ctx, content)` broadcasts final `message.update` and marks stream complete
- Throttle updates (50ms min interval) to avoid flooding the WebSocket

This enables token-by-token streaming for regular web chat conversations too, not just councils.

## Scope Boundaries

**In scope:**
- Council tool (start, resume, list actions)
- Council engine with round loop and moderator convergence
- Council persistence (meta.json + transcript.jsonl)
- Council event pipeline through EventBus
- Web backend REST API for council CRUD
- Pico WebSocket council event broadcasting
- Council list page in frontend
- Council transcript view with live streaming
- User interjection during live council
- Pico channel streaming support (BeginStream)

**Out of scope (future iterations):**
- Parallel agent speaking (agents speak sequentially in roster order)
- Tool execution within council agents (agents discuss, no tool calls)
- Council templates or preset agendas
- Export council transcript to markdown
- Council notifications on Telegram/other channels
- Agent self-selection (skip turn if nothing to add) — all agents speak each round for v1

## File Map

| New File | Purpose |
|----------|---------|
| `pkg/council/engine.go` | Council orchestration loop |
| `pkg/council/store.go` | Council session persistence |
| `pkg/council/types.go` | Council config, result, transcript types |
| `pkg/tools/council.go` | Council tool for the main agent |
| `web/backend/api/council.go` | REST API endpoints |
| `web/frontend/src/store/council.ts` | Jotai state atoms |
| `web/frontend/src/routes/councils/index.tsx` | Council list page |
| `web/frontend/src/routes/councils/$id.tsx` | Council transcript view |
| `web/frontend/src/components/council/` | Council UI components |

| Modified File | Change |
|----------------|--------|
| `pkg/agent/events.go` | Add council event kinds + payloads |
| `pkg/agent/loop.go` | Register council tool |
| `pkg/channels/pico/pico.go` | Implement BeginStream (StreamingCapable) |
| `pkg/channels/pico/protocol.go` | Add council.* message types |
| `web/frontend/src/features/chat/protocol.ts` | Handle council.* messages |
| `web/backend/api/router.go` | Register council API routes |
