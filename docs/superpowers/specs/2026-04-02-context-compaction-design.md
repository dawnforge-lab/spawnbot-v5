# Context Compaction — Two-Stage with Post-Compact Restoration

## Problem

When conversation exceeds context budget, Spawnbot drops the oldest 50% of turns entirely. The pre-compaction memory flush saves key facts to daily notes, but the conversation history itself is lost — the agent loses awareness of what was discussed, what tools are available, and what it was working on.

## Solution

Replace the blunt drop-50% approach with a two-stage compaction system inspired by Claude Code:

1. **Memory flush** (already exists) — extract key facts to daily notes
2. **Summarize-then-replace** (new) — use LLM to summarize the dropped messages, inject the summary as a single message so the agent retains conversation context
3. **Post-compact restoration** (new) — re-inject deferred tools announcement and active task context after compaction
4. **Multi-tier thresholds** (new) — warn before forcing compaction
5. **Circuit breaker** (new) — stop retrying after consecutive failures

## Architecture

### What exists today

- `forceCompression()` in `loop.go:2969-3043` — drops oldest ~50% of turns
- `flushMemoryPreCompaction()` in `memory_flush.go:49-74` — extracts key facts before drop
- `summarizeSession()` in `loop.go:3134-3233` — LLM summarization for session archiving
- `isOverContextBudget()` in `context_budget.go:161-176` — single threshold check
- `estimateMessageTokens()` / `estimateToolTokens()` — heuristic token estimation

### What changes

#### 1. Summarize-then-replace (`forceCompression` rewrite)

Replace the current `forceCompression()` logic:

**Before:** Drop oldest 50% of turns, record note in session summary.

**After:**
1. Run `flushMemoryPreCompaction()` (already exists)
2. Identify messages to drop (oldest ~50% of turns, same boundary logic)
3. Call LLM to summarize the messages being dropped (reuse `summarizeSession` pattern: 30s timeout, 0.3 temp, 2 retries)
4. Replace the dropped messages with a single user-role message: `[SYSTEM] Previous conversation was compacted. Summary of what was discussed:\n\n{summary}`
5. If LLM summarization fails, fall back to current behavior (drop without summary)

#### 2. Post-compact restoration

After replacing messages with summary, re-inject:
- **Deferred tools announcement** — call `BuildDeferredToolsAnnouncement()` and append to the summary message so the agent knows what tools are available
- **Active tasks** — if task system is active, append task summary

This prevents the agent from losing awareness of its tools and current work after compaction.

#### 3. Multi-tier thresholds

Replace single `isOverContextBudget()` with tiered checking:

| Tier | Threshold | Action |
|------|-----------|--------|
| **Normal** | < 80% of context window | No action |
| **Warning** | 80-90% of context window | Log warning, agent continues |
| **Compact** | > 90% of context window | Trigger compaction |

The warning tier gives visibility into approaching limits without forcing action.

#### 4. Circuit breaker

Track consecutive compaction failures. After 3 consecutive failures (LLM timeout, API error), stop attempting compaction and fall back to the current blunt drop behavior. Reset counter on successful compaction.

## Files Changed

- `pkg/agent/loop.go` — rewrite `forceCompression()`, add circuit breaker state
- `pkg/agent/context_budget.go` — add tiered threshold checking
- `pkg/agent/context.go` — expose `BuildDeferredToolsAnnouncement` for post-compact restoration (may already be public)

## Design Decisions

- Summary injected as `user` role message (matches existing wind-down warning pattern)
- LLM summarization failure falls back to blunt drop (no fallbacks that hide errors — the drop itself is transparent)
- Deferred tools re-injected in the summary message itself, not as a separate message
- Circuit breaker resets on success, not on timer
- No session memory layer (Claude Code's stage 1) — our periodic memory flush already handles this role
- Existing turn boundary logic preserved — we're changing what happens to dropped messages, not how we select them

## Testing

- Unit test: compaction produces summary message when LLM succeeds
- Unit test: compaction falls back to drop when LLM fails
- Unit test: circuit breaker stops after 3 consecutive failures
- Unit test: circuit breaker resets on success
- Unit test: tiered threshold returns correct tier
- Unit test: post-compact message includes deferred tools
