# pkg/struggles Package

## Status
Task 3 (Log Reader) completed. Commit: 42b70c8

## Files
- `pkg/struggles/types.go` — Signal struct and type constants
- `pkg/struggles/collector.go` — Collector (Task 2)
- `pkg/struggles/collector_test.go` — tests for Signal, type constants, and Collector
- `pkg/struggles/reader.go` — ReadLog, ReadLogCapped, ReadLogContent, TruncateLog, MaxLogBytes
- `pkg/struggles/reader_test.go` — 6 reader tests

## Signal struct
```go
type Signal struct {
    Timestamp time.Time `json:"ts"`
    Type      string    `json:"type"`
    Tool      string    `json:"tool,omitempty"`
    Error     string    `json:"error,omitempty"`
    Session   string    `json:"session,omitempty"`
    Context   string    `json:"context,omitempty"`
    Count     int       `json:"count,omitempty"` // for repeated_tool signals
}
```

## Constants
- `TypeToolError = "tool_error"`
- `TypeUserCorrection = "user_correction"`
- `TypeRepeatedTool = "repeated_tool"`

## Reader API (Task 3)
- `MaxLogBytes = 100 * 1024` — exported cap constant
- `ReadLog(path)` — reads all signals (delegates to ReadLogCapped with 0)
- `ReadLogCapped(path, capBytes)` — seeks to last capBytes, skips partial first line
- `ReadLogContent(path, capBytes)` — returns raw string content, capped
- `TruncateLog(path)` — clears the log file (writes nil)
- `scanSignals(scanner)` — internal helper, skips malformed JSON lines

## Integration Test (Task 4)
- `pkg/struggles/integration_test.go` — TestCollector_MultipleSignals_Integration
  - Covers all 3 signal types: tool_error, user_correction, repeated_tool
  - Verifies ReadLog returns correct count, all types present
  - Verifies TruncateLog clears the log

## Next
Task 5: Configuration
