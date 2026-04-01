# Tool Result Persistence

## Summary

Add disk persistence for large tool results. When a tool result exceeds a configurable size threshold, save the full content to disk and send a ~2KB preview to the LLM with a file path reference. The model can access the full result via `read_file` when needed. Includes per-tool threshold overrides and a per-turn aggregate budget to prevent context blowout from multiple large results.

Informed by Claude Code's battle-tested `toolResultStorage.ts` implementation.

## Motivation

- **Context window overflow**: A single `web_fetch` or large `read_file` result can consume tens of thousands of tokens, leaving little room for reasoning.
- **Token cost**: Verbose results sent in full waste tokens on content the model may never need.
- **Combinatorial blowup**: Multiple tools in one turn can each return results under individual limits but together exceed the context budget.
- **No current protection**: Today, tool results go straight into message history as-is. The only limit is shell output truncation at 1MB.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Threshold model | Two-tier: global default + per-tool config overrides | Flexibility of Claude Code without every tool declaring limits |
| Preview strategy | Head truncation ~2KB at newline boundary | Claude Code's proven approach. Simple, model can `read_file` for more |
| Storage location | Session-scoped: `sessions/{sessionID}/tool-results/{toolUseID}.txt` | Natural isolation, cleanup with session expiry |
| Full result access | Existing `read_file` tool | Zero new tools needed |
| Aggregate budget | Per-turn character cap across all results | Prevents combinatorial context blowout |
| Error handling | Transparent errors, no fallbacks | Per CLAUDE.md project principle |

## Architecture

### Component 1: ResultStore

New package `pkg/tools/resultstore/` — persist large results to disk and generate previews.

**Types:**

```go
type ResultStore struct {
    baseDir string // .../sessions/{sessionID}/tool-results/
}

type PersistedResult struct {
    FilePath string // Full path to persisted file
    Preview  string // First ~2KB, cut at newline boundary
    OrigSize int    // Original result size in bytes
}
```

**Methods:**

- `NewResultStore(sessionDir string) (*ResultStore, error)` — creates the store, ensures directory exists
- `Persist(toolUseID, content string) (*PersistedResult, error)` — writes content to `{toolUseID}.txt`, generates preview
- `generatePreview(content string, maxBytes int) string` — returns first ~2KB cut at last newline before the limit

**Preview format returned to LLM:**

```xml
<persisted-tool-result path="/home/user/.spawnbot/workspace/sessions/abc123/tool-results/tool_use_xyz.txt" original_size="125430">
[first ~2KB of content here]
</persisted-tool-result>
This tool result was too large to include in full (125430 bytes). A preview is shown above. Use read_file to access the complete output.
```

### Component 2: Size Gate

Interception logic in the tool execution path. Not a separate package — a function in `pkg/agent/` that wires ResultStore into the existing flow.

**Configuration** (added to `pkg/config/config.go`):

```go
type ToolResultPersistenceConfig struct {
    Enabled            bool           `json:"enabled"`
    DefaultMaxChars    int            `json:"default_max_chars"`
    PerTurnBudgetChars int            `json:"per_turn_budget_chars"`
    PreviewSizeBytes   int            `json:"preview_size_bytes"`
    ToolOverrides      map[string]int `json:"tool_overrides,omitempty"`
}
```

**Gate function:**

```go
func maybePersistResult(
    store *resultstore.ResultStore,
    cfg config.ToolResultPersistenceConfig,
    toolName string,
    toolCallID string,
    result *tools.ToolResult,
    turnBudgetUsed *int,
) error
```

**Logic:**

1. If result is error (`IsError`) or `ForLLM` is empty — skip, never persist errors
2. Get threshold: `ToolOverrides[toolName]` if set, else `DefaultMaxChars`
3. Check individual size: `len(result.ForLLM) > threshold`
4. Check aggregate: `*turnBudgetUsed + len(result.ForLLM) > PerTurnBudgetChars`
5. If either triggers: call `store.Persist(toolCallID, result.ForLLM)`, replace `result.ForLLM` with preview message
6. Always: `*turnBudgetUsed += len(result.ForLLM)` (tracks original or preview size)

The turn budget counter is a local `int` scoped to the tool execution loop within a single turn — resets naturally.

### Component 3: Integration Points

**`pkg/agent/loop.go`** — Wire into tool execution:

- Lazy-init `ResultStore` on first tool execution in a turn (only if persistence enabled in config)
- Initialize `turnBudgetUsed := 0` before the tool execution loop
- After each `ExecuteWithContext()` returns and before result is added to messages/session, call `maybePersistResult()`
- ResultStore session dir derived from memory base dir + session key

**`pkg/config/config.go`** — Add `ToolResultPersistence ToolResultPersistenceConfig` field to main `Config` struct.

**`pkg/config/defaults.go`** — Defaults:

| Field | Default | Rationale |
|-------|---------|-----------|
| `Enabled` | `true` | On by default, matching deferred loading |
| `DefaultMaxChars` | `50000` | Claude Code's proven default |
| `PerTurnBudgetChars` | `200000` | Claude Code's per-message aggregate |
| `PreviewSizeBytes` | `2000` | Claude Code's preview size |
| `ToolOverrides` | `nil` | No overrides by default |

**No changes to:**

- `ToolResult` struct — we mutate `ForLLM` in place, same type
- Provider layer — sees the same `Message` format
- Session store — persists the preview content, full result lives on disk separately

### Data Flow

```
Tool executes
  ↓
ExecuteWithContext() returns ToolResult
  ↓
maybePersistResult() checks size
  ├── Under threshold AND under aggregate budget
  │     → Pass through unchanged
  └── Over threshold OR aggregate exceeded
        ↓
      ResultStore.Persist(toolCallID, content)
        ↓
      Write to: sessions/{sessionID}/tool-results/{toolCallID}.txt
        ↓
      Generate 2KB preview (head, newline boundary)
        ↓
      Replace result.ForLLM with preview + XML wrapper + file path
        ↓
      Track preview size in turn budget
  ↓
Result added to messages + session (original or preview)
  ↓
Model sees preview, can read_file for full content
```

## Error Handling

All errors transparent, no fallbacks (per project CLAUDE.md):

| Scenario | Behavior |
|----------|----------|
| Disk write fails | Return error to agent as tool result error |
| Directory creation fails | Same — transparent error |
| ResultStore not initialized (startup/config issue) | Skip persistence, pass through as-is, log warning |
| Preview generation on short content | Return content as-is (gate shouldn't have triggered) |

## Testing Strategy

1. **ResultStore unit tests** — Persist writes file correctly, preview truncates at newline boundaries, handles empty/short content, directory creation
2. **Size gate unit tests** — Individual threshold triggers, aggregate budget triggers, error results skipped, per-tool overrides respected, budget tracking across multiple tools
3. **Integration test** — Full flow: tool returns large result → persisted to disk → preview in message history → `read_file` can access full result
4. **Config tests** — Defaults applied, overrides merged correctly

## Files Created

- `pkg/tools/resultstore/store.go` — ResultStore implementation
- `pkg/tools/resultstore/store_test.go` — ResultStore unit tests
- `pkg/agent/result_persist.go` — Size gate function
- `pkg/agent/result_persist_test.go` — Size gate unit tests

## Files Modified

- `pkg/config/config.go` — Add ToolResultPersistenceConfig
- `pkg/config/defaults.go` — Add defaults
- `pkg/agent/loop.go` — Wire maybePersistResult into tool execution loop
