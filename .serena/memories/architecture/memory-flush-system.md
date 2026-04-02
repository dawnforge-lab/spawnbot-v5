# Memory Flush System

Added in commit `9bb3095`. Preserves conversation context across sessions and compaction.

## Components
- `pkg/agent/memory_flush.go` — Core implementation
- `pkg/agent/memory_flush_test.go` — 6 unit tests
- `pkg/agent/instance.go` — `MemoryStore` field added to `AgentInstance`
- `pkg/agent/loop.go` — Two integration points at turn-end

## How It Works
1. **Pre-compaction flush**: In `summarizeSession()`, before messages are truncated, `flushMemoryPreCompaction()` extracts key facts via LLM and writes to daily notes (`YYYYMMDD.md`)
2. **Periodic flush**: `maybePeriodicFlush()` triggers every 15 messages (checkpoint-based), runs async in background goroutine
3. **Deduplication**: LLM prompt includes existing daily notes; returns `NOTHING_NEW` if all facts already covered. Output validated for bullet-point format.

## Key Constants
- `PeriodicFlushInterval = 15`
- `memoryFlushTimeout = 30s`
- `memoryFlushMaxRetries = 2`
- Message content truncated to 500 chars for extraction prompt

## Session Architecture
- Sessions never auto-rotate (no TTL/expiry)
- JSONL backend with `.meta.json` for summary/skip offset
- Two-tier compaction: proactive (background summarize at 20 msgs or 75% tokens) + reactive (emergency forceCompression)
- `MemoryStore` reads daily notes on demand via tools, not injected into system prompt

## forceCompression (rewritten in commit `fa2f2e1`)
- Drops oldest ~50% of turns (same split logic as before)
- **Stage 1**: `flushMemoryPreCompaction()` saves key facts to daily notes before messages are lost
- **Stage 2**: `summarizeForCompaction()` calls LLM (30s timeout, 2 retries) to summarize dropped messages
  - On success: injects `[SYSTEM] Previous conversation was compacted...` message with summary + deferred tools announcement
  - On failure: falls back to original behavior (compression note in session summary)
- **Circuit breaker**: `compactionFailures` field on `AgentLoop` tracks consecutive failures; after 3 failures, skips summarization entirely (avoids wasting tokens on a broken provider)
- `compactionSummaryPrompt` constant formats the summarization prompt
- Only user/assistant messages included in summarization input; tool results skipped; messages truncated to `agent.ContextWindow` chars

## Test Coverage — compaction with summarization (commit `171f3a5`)
- `TestAgentLoop_CompactionInjectsSummary` in `pkg/agent/loop_test.go`
- Uses `compactionTestProvider` + `compactionTestTool` helpers
- Pattern: 3 turns across a 4000-token context window
  - Turn 1 & 2: generate large tool results filling history above threshold
  - Turn 3: triggers proactive budget check (`>90%`) → `forceCompression` → `summarizeForCompaction`
- Asserts history contains a "Previous conversation was compacted" user message with "Summary:" content
- Falls back to `t.Skip` if summaryCalls==0 (provider never called for summarization)
- Key insight: proactive check runs once per turn at turn start with existing history, so multiple turns are needed to accumulate history before compaction fires
