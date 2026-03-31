# Self-Improver Builtin Agent (Task 4)

## Status
DONE. Commit: 703d234 (branch: feature/self-improvement)

## Files
- `pkg/agents/builtin/self-improver/AGENT.md` — agent definition
- `pkg/agents/builtin.go` — added selfImproverAgentMD embed + map entry
- `pkg/agents/integration_test.go` — updated count from 4 → 5, added "self-improver" to expected names
- `pkg/struggles/integration_test.go` — TestCollector_MultipleSignals_Integration

## Agent Config
```yaml
name: self-improver
max_iterations: 30
timeout: 10m
tools_allow: [read_file, list_dir, write_file, create_agent, search_tools, skills_search, memory_search, subagent]
tools_deny: [message, send_file, exec, shell, connect_mcp, disconnect_mcp, spawn]
```

## Behavior
1. Groups struggle signals by pattern (filters <2 occurrences)
2. Decides: skill (behavior fix) vs agent (specialized tool access)
3. Checks for duplicates before creating
4. Creates skill (SKILL.md) or agent (create_agent tool)
5. Validates via subagent test (up to 2 retries)
6. Deletes failed creations; reports unresolved patterns
7. Max 3 creations per run (cost control)

## Integration Points
- Spawned by heartbeat service during daily reviews (Task 6)
- Reads struggle log written by Collector (pkg/struggles)
- Creates in workspace skills/ and agents/ directories
