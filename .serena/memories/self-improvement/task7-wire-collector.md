# Task 7: Wire Collector into Agent Loop — Completed

## What was done
Wired the `pkg/struggles.Collector` into the agent execution loop so struggle signals are captured at runtime.

## Changes
- **pkg/agent/instance.go**: Added `StruggleCollector *struggles.Collector` field to `AgentInstance` struct
- **pkg/agent/loop.go**: 
  - Added `struggles *struggles.Collector` field to `AgentLoop` struct
  - Added `SetStruggleCollector()` setter method
  - Lazy-set collector on agents in `processMessage()` after route resolution
  - Hook in `processMessage()`: calls `OnUserMessage()` before processOptions construction to detect user corrections
  - Hook in `runTurn()`: calls `OnToolResult()` after tool error tracking block when `toolResult.IsError`
  - Added `toolCallCounts` map in `runTurn()`, incremented before each tool execution
  - Hook before `TurnPhaseFinalizing`: calls `OnTurnEnd(toolCallCounts)` to detect repeated tool usage

## Commit
`563064d` on branch `feature/self-improvement`
