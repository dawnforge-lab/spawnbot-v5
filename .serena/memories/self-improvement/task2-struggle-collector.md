# Task 2: Struggle Collector — COMPLETED (2026-03-31)

## Commit
`20d5e31` feat(struggles): add collector for tool errors, user corrections, repeated tools

## Files Created
- `pkg/struggles/collector.go` — Collector struct with OnToolResult, OnUserMessage, OnTurnEnd methods
- `pkg/struggles/reader.go` — ReadLog function reads JSONL signal log

## Files Modified
- `pkg/struggles/collector_test.go` — Added `path/filepath` import + 6 new TestCollector_* test functions

## Key Implementation Details
- `correctionPatterns` regex detects user corrections at message start (case-insensitive)
- `repeatedToolThreshold = 3` — tools called 3+ times in a turn emit TypeRepeatedTool signal
- `maxContextLen = 200` — truncates args/error context stored in signals
- Uses `logger.ErrorCF` for transparent error logging (no fallbacks/silent failures)
- `ReadLog` returns nil (not error) when file doesn't exist

## Tests
All 8 tests pass (2 pre-existing + 6 new collector tests)
