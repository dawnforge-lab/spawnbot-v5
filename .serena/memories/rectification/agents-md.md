# AGENTS.md — Operational Instructions — COMPLETED (2026-04-02)

## Commits
- `8e0e043` feat(agent): load AGENTS.md as second bootstrap file in system prompt
- `3ebd4ed` feat(workspace): add AGENTS.md template, trim SOUL.md to identity-only

## What
Created AGENTS.md as a second bootstrap file loaded alongside SOUL.md into the system prompt. SOUL.md is now pure identity (personality, values). AGENTS.md carries operational instructions: rules, communication style, capabilities (heartbeat, self-improvement, cron, subagents), skills & tools guidance, memory layers explanation, workspace file references.

## Key Design Decisions
- AGENTS.md is optional — no error if missing (graceful for existing installs)
- "Always use tools" rule removed — caused tool loops
- All workspace files noted as self-modifiable by the agent
- Path safety guidance included as rule #3
- AGENTS.md added to sourcePaths() for cache invalidation
- PLAYBOOK.md and SYSTEM_CAPABILITIES.md content absorbed but files remain on disk

## Files Changed
- `pkg/agent/context.go` — LoadBootstrapFiles loads both files, sourcePaths includes both
- `pkg/agent/context_soul_test.go` — 3 tests (both files, no AGENTS.md, no SOUL.md)
- `workspace/AGENTS.md` — new template
- `workspace/SOUL.md` — trimmed to identity-only
- `~/.spawnbot/workspace/AGENTS.md` — live copy
- `~/.spawnbot/workspace/SOUL.md` — live copy trimmed
