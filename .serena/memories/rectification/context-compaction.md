# Context Compaction — COMPLETED (2026-04-02)

## Commits
- `f19ca28` feat(agent): add tiered context budget thresholds (normal/warning/compact)
- `fa2f2e1` feat(agent): rewrite forceCompression with LLM summarization and circuit breaker
- `31d23f8` feat(agent): wire tiered context budget thresholds into turn loop
- `171f3a5` test(agent): add tests for compaction with LLM summarization

## What
Replaced the blunt drop-50%-of-turns compression with a two-stage compaction system:

1. **Memory flush** (already existed) — `flushMemoryPreCompaction` extracts key facts to daily notes
2. **Summarize-then-replace** (new) — `summarizeForCompaction` uses LLM to summarize dropped messages, injects summary + deferred tools as a `[SYSTEM]` message
3. **Post-compact restoration** — re-injects `BuildDeferredToolsAnnouncement` so agent retains tool awareness
4. **Circuit breaker** — `compactionFailures` counter on AgentLoop, skips summarization after 3 consecutive failures, falls back to blunt drop
5. **Tiered thresholds** — `checkContextBudgetTier` returns Normal (<80%), Warning (80-90% with log), Compact (>90% triggers compaction)

## Fallback behavior
If LLM summarization fails, falls back to original behavior (drop without summary, record compression note in session summary). No hidden failures.

## Files Changed
- `pkg/agent/context_budget.go` — ContextBudgetTier type, checkContextBudgetTier function
- `pkg/agent/loop.go` — compactionSummaryPrompt constant, compactionFailures field, rewritten forceCompression, new summarizeForCompaction, tiered threshold wiring
- `pkg/agent/context_budget_test.go` — 3 tier threshold tests
- `pkg/agent/loop_test.go` — compaction integration test (3-turn scenario, verifies summary injection)
