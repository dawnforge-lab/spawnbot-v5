# Markdown-Based Agent Definitions

**Date:** 2026-03-31
**Status:** Approved
**Scope:** New `pkg/agents/` package, agent registry, builtin agents, `create_agent` tool, integration with existing subagent system

## Overview

Spawnbot's subagents currently use a single hardcoded prompt with no specialization. This design introduces markdown-based agent definitions — each agent type has its own system prompt, tool restrictions, model override, and execution limits. Agents are defined as `AGENT.md` files with YAML frontmatter, loaded from embedded builtins and the user's workspace. The main agent can also create new agent types autonomously.

## Agent Definition Format

### Directory Structure

```
~/.spawnbot/workspace/agents/
  researcher/
    AGENT.md          # required
    references/       # optional bundled resources
  coder/
    AGENT.md
  reviewer/
    AGENT.md
  planner/
    AGENT.md
```

Each agent is a directory containing an `AGENT.md` file. The directory may also contain reference materials the agent can access via its `BaseDir`.

### AGENT.md Schema

```yaml
---
name: researcher                    # required, unique identifier
description: "Gathers information"  # required, used in context summary
model: ""                           # optional, empty = inherit parent model
tools_allow: []                     # optional, allowlist (empty = all tools)
tools_deny:                         # optional, denylist applied after allowlist
  - write_file
  - edit_file
  - exec
max_iterations: 20                  # optional, default 20
timeout: 5m                         # optional, default 5m
---

System prompt markdown body goes here.
The full markdown content below the frontmatter becomes the agent's system prompt.
```

### Tool Filtering Rules

- If both `tools_allow` and `tools_deny` are set: start with allowlist, then remove denylist entries
- If only `tools_deny`: start with all tools, remove denylist entries
- If only `tools_allow` (non-empty): only those tools are available
- If neither, or `tools_allow` is empty/omitted: all tools minus `spawn` and `subagent`

**Note:** An empty `tools_allow: []` is treated the same as omitted — it means "all tools". To restrict to specific tools, list them explicitly.

**Recursive spawn prevention:** `spawn` and `subagent` are always removed from the subagent's tool set unless explicitly included in `tools_allow`.

**Empty toolset:** If filtering results in zero tools, the spawn/subagent tool returns an error to the main agent rather than running a toolless agent.

### Validation

- `name`: alphanumeric + hyphens, max 64 chars, required
- `description`: max 1024 chars, required
- `model`: empty string or valid model identifier
- `max_iterations`: positive integer, default 20
- `timeout`: valid Go duration string, default "5m"

## Agent Registry (`pkg/agents/`)

### Package Structure

```
pkg/agents/
  definition.go   # AgentDefinition struct
  registry.go     # Registry struct — Get, List, Summary, Register, Reload
  loader.go       # AGENT.md parsing, frontmatter extraction
  builtin.go      # Embedded builtin agent definitions via go:embed
```

### AgentDefinition

```go
type AgentDefinition struct {
    Name            string
    Description     string
    SystemPrompt    string
    Model           string
    ToolsAllow      []string
    ToolsDeny       []string
    MaxIterations   int
    Timeout         time.Duration
    Source          string    // "builtin" | "workspace"
    BaseDir         string   // directory path for resource access
}
```

### Registry

```go
type Registry struct {
    mu     sync.RWMutex
    agents map[string]*AgentDefinition
}
```

**Operations:**

| Method | Description |
|--------|-------------|
| `NewRegistry()` | Creates empty registry |
| `LoadBuiltins()` | Registers the 4 embedded agents |
| `LoadFromDir(dir)` | Scans `dir/*/AGENT.md`, parses each |
| `Get(name)` | Lookup by name, returns nil if not found |
| `List()` | Returns all loaded agents |
| `Summary()` | Formatted summary for system prompt injection (includes load warnings) |
| `Register(def)` | Add or overwrite an agent at runtime |
| `Reload()` | Re-scan workspace directory for new/changed agents |

**Loading priority:** Builtins loaded first, then workspace. Workspace agents with the same name override builtins.

### Summary Format

Injected into the main agent's system prompt alongside the skills summary:

```
Available agents:
- researcher: Gathers information from web and files without making changes
- coder: Writes code, scripts, and MCP servers
- reviewer: Reviews work for issues and improvements
- planner: Breaks down complex tasks into structured plans
```

If any workspace agents failed to load, a warning line is appended:
```
WARNING: agents/foo/AGENT.md failed to load: missing required field "description"
```

This lets the main agent see and fix broken agent definitions autonomously.

## Integration with Existing Subagent System

### Tool Changes

The `spawn` and `subagent` tools gain an `agent_type` parameter:

```json
{
  "task": "Research the best Go TUI libraries",
  "agent_type": "researcher"
}
```

**When `agent_type` is provided:**
1. Look up `AgentDefinition` from registry
2. Clone parent tool registry
3. Apply allow/deny filters
4. Remove `spawn`/`subagent` (unless explicitly in `tools_allow`)
5. Construct `SubTurnConfig`:
   - `SystemPrompt` = agent's markdown body
   - `Model` = agent's model override (or inherit parent)
   - `MaxIterations` = from agent definition
   - `Timeout` = from agent definition
   - `Tools` = filtered tool set

**When `agent_type` is omitted:** Falls back to current behavior (hardcoded lean prompt, all tools minus spawn). Backward compatible.

### Context Building

In `pkg/agent/loop.go`, during system prompt assembly, inject `registry.Summary()` in the same location where `BuildSkillsSummary()` is called. The main agent sees both available skills and available agents in its context.

### Subagent Memory

Subagents are stateless workers. They do not have access to the main agent's memory store. They receive a task, execute it, and return results. The main agent decides what to persist in its own memory.

## Builtin Agents

Four agents embedded in the binary via `//go:embed`.

### researcher

- **Purpose:** Gather information without side effects
- **tools_deny:** `write_file`, `edit_file`, `append_file`, `exec`, `message`, `send_file`, `spawn`, `subagent`, `connect_mcp`, `disconnect_mcp`
- **Prompt:** Read files, web search, web fetch, memory search. Report findings in structured format. Never modify anything.

### coder

- **Purpose:** Write code, scripts, config files
- **tools_deny:** `message`, `send_file`, `spawn`, `subagent`
- **Prompt:** Write clean, working code. Use exec to test. Read existing code before modifying. No user communication — code and report back.

### reviewer

- **Purpose:** Review work for bugs, issues, improvements
- **tools_deny:** `write_file`, `edit_file`, `append_file`, `exec`, `message`, `send_file`, `spawn`, `subagent`
- **Prompt:** Read-only analysis. Check for bugs, security issues, logic errors. Return structured review with severity ratings.

### planner

- **Purpose:** Break down complex tasks into structured plans
- **tools_deny:** `exec`, `message`, `send_file`, `spawn`, `subagent`
- **Prompt:** Analyze the task, research what's needed, produce a step-by-step plan with dependencies. Can write plan files to workspace via `write_file`.

## Autonomous Agent Creation

### `create_agent` Tool

```go
type CreateAgentInput struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    SystemPrompt string   `json:"system_prompt"`
    ToolsAllow   []string `json:"tools_allow"`
    ToolsDeny    []string `json:"tools_deny"`
    Model        string   `json:"model"`
    MaxIters     int      `json:"max_iterations"`
    Timeout      string   `json:"timeout"`
}
```

**Execution flow:**
1. Validate name format (alphanumeric + hyphens, max 64 chars)
2. Reject if name collides with a builtin agent
3. Build AGENT.md content from frontmatter + system prompt
4. Create directory `~/.spawnbot/workspace/agents/<name>/`
5. Write `AGENT.md`
6. Parse it back through the loader (validates the output)
7. Register in registry — agent immediately available
8. Return success

**Builtin protection:** `create_agent` cannot overwrite builtin agents. To customize a builtin, the agent must use a different name (e.g., `researcher-v2`).

**No delete tool initially.** Agents are evolved by creating new versions. Deletion can be added later if needed.

## Error Handling

All errors are returned as tool results to the main agent. The main agent is autonomous — it fixes its own problems.

| Error | Behavior |
|-------|----------|
| Agent not found | Error result: `"agent type 'foo' not found. Available: researcher, coder, reviewer, planner"` |
| Invalid AGENT.md at startup | Skip agent, append warning to `registry.Summary()` for main agent to see and fix |
| Empty toolset after filtering | Error result: `"agent 'foo' has no tools after filtering — check tools_allow/tools_deny"` |
| Timeout | Context cancelled, error result: `"agent 'researcher' timed out after 5m"` |
| create_agent name collision | Error result: `"cannot overwrite builtin agent 'researcher' — use a different name"` |
| create_agent invalid name | Error result: `"agent name must be alphanumeric + hyphens, max 64 chars"` |
| create_agent parse failure | Error result with detail, file left on disk for main agent to inspect and fix |

No silent fallbacks. No error suppression. No cleanup of failed files.

## What This Design Does NOT Include

- **Per-agent persistent memory** — subagents are stateless, main agent owns memory
- **Permission/approval system** — autonomous agent, no gates
- **Project-level agents** — personal agent, workspace only
- **Agent deletion tool** — not needed initially
- **Hot-reload on file change** — `Reload()` is called explicitly, not via filesystem watcher
