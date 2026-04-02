# Task 3: StrugglesObserver Bridge — COMPLETED

## Commit: 1562198
`feat(hooks): add StrugglesObserver bridge for EventObserver interface`

## Files Changed
- `pkg/agent/struggles_observer.go` — new: adapts struggles.Collector to EventObserver interface
- `pkg/agent/struggles_observer_test.go` — new: 4 tests (ToolError, TurnStart_Correction, TurnEnd_RepeatedTool, IgnoresIrrelevantEvents)
- `pkg/agent/loop.go` — fixed broken call sites from Task 2 rename (OnUserMessage→HandleTurnStart, OnToolResult→HandleToolEnd, OnTurnEnd→HandleTurnEnd)

## Notes
- Task 2 renamed the collector methods but left loop.go broken; Task 3 fixed those 3 call sites as a prerequisite for the build to compile
- OnUserMessage previously passed prevAssistant content; HandleTurnStart only takes userMessage+session — correction detection is now purely regex-based on the user message (no context comparison)
- toolCallCounts local variable in loop.go is now only used to increment the count per tool call (line 2479); HandleTurnEnd no longer receives it — the Collector manages its own turnToolCounts internally
- Task 4 (Remove Hardcoded Struggles from loop.go) still needs to remove the remaining 3 direct StruggleCollector call sites and wire via the observer instead
