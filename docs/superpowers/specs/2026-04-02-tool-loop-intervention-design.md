# Tool Loop Intervention

## Problem

The agent sometimes calls the same tool repeatedly in a single turn (e.g. `read_file` 10+ times) without making progress. The struggle collector (`pkg/struggles/collector.go`) detects this pattern but only logs it to JSONL for post-hoc analysis. There is no real-time intervention to break the loop.

## Solution

Add a per-tool call counter inside `runTurn()` in `pkg/agent/loop.go`. When any tool is called 3+ times in a single turn, inject a `[SYSTEM]` warning message into the conversation, prompting the LLM to reconsider its approach. The warning fires once per tool per turn.

This mirrors the existing `consecutiveToolErrors` pattern which already injects wind-down messages on repeated failures.

## Design

### New constant

Next to `toolWindDownWarning` (~line 107 in `loop.go`):

```go
toolRepetitionWarning = "[SYSTEM] You have called '%s' %d times this turn. You may be stuck in a loop. Pause and consider: why do you keep needing this tool? Try a different approach to achieve your goal."
```

### New state in `runTurn()`

Next to `consecutiveToolErrors` (~line 1778):

```go
toolCallCounts := make(map[string]int)
toolLoopWarned := make(map[string]bool)
const repeatedToolThreshold = 3
```

### Injection point

After the `consecutiveToolErrors` block (~line 2658), before steering message dequeue:

```go
toolCallCounts[toolName]++
if toolCallCounts[toolName] >= repeatedToolThreshold && !toolLoopWarned[toolName] {
    toolLoopWarned[toolName] = true
    warning := providers.Message{
        Role:    "user",
        Content: fmt.Sprintf(toolRepetitionWarning, toolName, toolCallCounts[toolName]),
    }
    ts.agent.Sessions.AddFullMessage(ts.sessionKey, warning)
    messages = append(messages, warning)
    logger.WarnCF("agent", "Tool repetition warning injected", map[string]any{
        "tool":  toolName,
        "count": toolCallCounts[toolName],
    })
}
```

## Behaviour

- Counter tracks per-tool calls within a single turn (local variable, resets naturally)
- Threshold: 3 calls of the same tool triggers the warning
- `toolLoopWarned` map ensures one warning per tool per turn — no spam
- Warning is advisory — does not block tool execution
- Warning uses `fmt.Sprintf` with tool name and count for specificity
- Independent of the struggles collector (collector logs for analysis, loop intervenes in real-time)
- No config knob for threshold (matches `maxConsecutiveToolErrors` which is also hardcoded)

## Files changed

- `pkg/agent/loop.go` — new constant, new state variables, injection logic

## Testing

- Unit test: mock a turn with 3+ calls to the same tool, verify warning message is injected into session
- Unit test: verify warning fires only once per tool (4th call doesn't inject a second warning)
- Unit test: verify different tools each get their own independent counter
