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
- Two-tier compaction: proactive (background summarize at 20 msgs or 75% tokens) + reactive (emergency drop oldest 50% turns)
- `MemoryStore` reads daily notes on demand via tools, not injected into system prompt
