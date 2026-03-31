# Agent Definitions Worktree Progress

Branch: feature/agent-definitions
Worktree: /home/eugen-dev/Workflows/picoclaw/.worktrees/agent-definitions

## Completed Tasks

### Task 1: AgentDefinition Struct + Validation (commit: e5886f0 area)
- `pkg/agents/definition.go` — AgentDefinition struct, Validate(), ApplyDefaults(), FilterTools()

### Task 2: AGENT.md Loader
- `pkg/agents/loader.go` — ParseAgentMD(), LoadFromDir()

### Task 3: Agent Registry
- `pkg/agents/registry.go` — Registry with Register, Get, List, IsBuiltin, Reload, Summary

### Task 4: Builtin Agent Definitions (commit: fe9d6bf)
- `pkg/agents/builtin.go` — go:embed + LoadBuiltins() using ParseAgentMD
- `pkg/agents/builtin/researcher/AGENT.md` — read-only, 20 iter, 5m, denies write/exec/message/spawn/mcp
- `pkg/agents/builtin/coder/AGENT.md` — 20 iter, 10m, denies message/send_file/spawn/subagent
- `pkg/agents/builtin/reviewer/AGENT.md` — read-only, 15 iter, 5m, denies write/exec/message/spawn
- `pkg/agents/builtin/planner/AGENT.md` — 20 iter, 5m, denies exec/message/send_file/spawn/subagent

All 25 tests pass: `go test ./pkg/agents/ -v -count=1`

### Task 5: Integrate Registry into AgentInstance
- `pkg/agent/instance.go` — Added AgentRegistry field to AgentInstance
- `pkg/agent/loop.go` — Loads builtin + workspace agent definitions at startup, sets on agent

### Task 6: Add agent_type to spawn/subagent tools (commit: b93c22a)
- `pkg/tools/subagent.go` — Added AgentType to tools.SubTurnConfig, agent_type param to SubagentTool
- `pkg/tools/spawn.go` — Added agent_type param to SpawnTool, passed through to SubTurnConfig
- `pkg/agent/subturn.go` — Added AgentType to agent.SubTurnConfig, registry resolution in spawnSubTurn()
  - Resolves agent definition from registry when agent_type is set
  - Overrides system prompt, model, timeout, and filters tools via allow/deny lists
  - Backward compatible: omitting agent_type preserves existing behavior

## Pending Tasks
- Task 7: create_agent tool
- Task 8: Register create_agent in agent startup
- Task 9: End-to-end integration tests
- Task 10: Full build verification
