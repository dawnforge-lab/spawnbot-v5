# Tool Loop Intervention — COMPLETED (2026-04-02)

## Commits
- `b9e5af3` feat(agent): add tool repetition warning for loop detection
- `a93ffd9` test(agent): add tests for tool repetition warning injection

## What
Per-tool call counter in `runTurn()` (loop.go). When same tool called 3+ times in a turn, injects a `[SYSTEM]` warning message prompting the agent to reconsider its approach. One warning per tool per turn (no spam). Advisory only — doesn't block execution.

## Files Changed
- `pkg/agent/loop.go` — new `toolRepetitionWarning` constant, `toolCallCounts`/`toolLoopWarned` maps, injection logic after `consecutiveToolErrors` block
- `pkg/agent/loop_test.go` — `repeatedToolProvider`, `twoToolProvider`, mock tools, 2 tests

## Tests
- `TestAgentLoop_ToolRepetitionWarningInjected` — 4 calls to same tool, exactly 1 warning injected
- `TestAgentLoop_ToolRepetitionCountersIndependent` — alternating 2 tools (2 each), no warning
