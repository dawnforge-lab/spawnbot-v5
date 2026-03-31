# Agent Definitions Worktree — Task Progress

Worktree: `/home/eugen-dev/Workflows/picoclaw/.worktrees/agent-definitions`
Branch: `feature/agent-definitions`

## Task Status

- [x] Task 1: AgentDefinition Struct + Validation — DONE (commit 777d46a)
  - Created `pkg/agents/definition.go` — AgentDefinition struct, Validate(), ApplyDefaults(), FilterTools()
  - Created `pkg/agents/definition_test.go` — 10 tests, all passing
- [x] Task 2: AGENT.md Loader — DONE (commit a3d75c1)
  - Created `pkg/agents/loader.go` — ParseAgentMD(), LoadFromDir(), splitFrontmatter()
  - Created `pkg/agents/loader_test.go` — 6 tests, all passing (16 total in pkg)
- [ ] Task 3: Agent Registry
- [ ] Task 4: Builtin Agent Definitions
- [ ] Task 5: Integrate Registry into AgentInstance
- [ ] Task 6: Add agent_type to spawn/subagent tools
- [ ] Task 7: create_agent tool
- [ ] Task 8: Register create_agent in agent startup
- [ ] Task 9: End-to-end integration tests
- [ ] Task 10: Full build verification

## Key Design Decisions
- `FilterTools`: no explicit allow list → spawn/subagent are denied by default
- `FilterTools`: explicit allow list → deny list still applies on top
- Name validation: `^[a-zA-Z0-9][a-zA-Z0-9\-]*$`, max 64 chars
- Description max: 1024 chars
- Defaults: MaxIterations=20, Timeout=5min
