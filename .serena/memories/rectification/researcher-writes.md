# Researcher Agent Writes — COMPLETED (2026-04-02)

## Commit
`094d8d3` feat(agents): allow researcher agent to write files

## Change
Removed `write_file`, `edit_file`, `append_file` from researcher's `tools_deny` list. Agent runs in a separate environment so file restrictions are unnecessary. Updated AGENT.md description and prompt to reflect the new capability. Updated integration test to match.

## Files Changed
- `pkg/agents/builtin/researcher/AGENT.md` — removed 3 tools from deny list, updated description/prompt
- `pkg/agents/integration_test.go` — removed write_file/edit_file/append_file from researcher deny assertions
