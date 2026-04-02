# Deferred Tool Loading — COMPLETED

Worktree: `/home/eugen-dev/Workflows/picoclaw/.worktrees/deferred-tools`
Branch: `feature/deferred-tool-loading` — merged to main (a5f3223)
Base: `1a066a2`

## Task Status

- [x] Task 1: Registry — remove TTL, add session-persistent discovery (commit 7f7f421)
- [x] Task 2: Update existing search tool tests (handled in Task 1)
- [x] Task 3: Unified search_tools tool (commit 33b4e62)
- [x] Task 4: Deferred tools announcement (commit d571fe8)
- [x] Task 5: Wire integration — loop, MCP, config (commit a13fdab)
- [x] Task 6: Config cleanup (commit 1219682)
- [x] Task 7: Integration test (commit d426789)
- [x] Task 8: Clean up old search tool code (commit 9d551a0)
- [x] Task 9: Full build verification — PASS

## Architecture

- `ToolRegistry.discovered map[string]bool` replaces TTL-based visibility
- `PromoteTools(names)` sets discovered=true (session-persistent, no expiry)
- `search_tools` (always core) replaces `tool_search_tool_bm25` + `tool_search_tool_regex`
  - `select:name1,name2` mode: exact activation
  - keyword mode: scored search (name +10, hint +4, desc +2), no auto-activate
- `<available-deferred-tools>` XML block prepended to user messages each turn
- MCP tools always registered as hidden with search hints from description
- Non-core builtin tools (web, spawn, skills, tasks, agents, mcp mgmt) registered hidden
- Core tools: read_file, write_file, list_dir, edit_file, append_file, exec, message, send_file, create_hook, search_tools, i2c, spi

## Key Files Changed
- `pkg/tools/registry.go` — discovered map, GetDeferredNames/Tools, RegisterHiddenWithHint
- `pkg/tools/search_tool_unified.go` — new SearchTools implementation
- `pkg/tools/search_tool.go` — old RegexSearchTool/BM25SearchTool removed
- `pkg/agent/context.go` — BuildDeferredToolsAnnouncement, simplified getDiscoveryRule
- `pkg/agent/turn.go` — announcement injection in buildMessages
- `pkg/agent/loop.go` — search_tools as core, non-core as hidden, TickTTL removed
- `pkg/agent/loop_mcp.go` — MCP tools always hidden with hints, old discovery block removed
- `pkg/config/config.go` — ToolDiscoveryConfig simplified (TTL, UseBM25, UseRegex removed)

## Notes
- 5 pre-existing media test failures in pkg/agent/ (unrelated)
- Pre-existing embed error in pkg/workspace/deploy.go (missing dist files)
- Discovery enabled by default in config defaults
