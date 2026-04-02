# Task 6: Implement CommandHook — COMPLETED

## Commit: 5313966
`feat(hooks): add CommandHook type for one-shot shell script hooks`

## Files Created
- `pkg/agent/hook_command.go` — CommandHook implementation
- `pkg/agent/hook_command_test.go` — 7 tests covering all modes

## Architecture

### CommandHook
One-shot shell script hook — spawns a new process per event (contrast with ProcessHook which is a long-running daemon).

Two modes:
- **observe** (`mode: "observe"`) — implements `EventObserver`, fire-and-forget, errors are logged not propagated
- **intercept** (`mode: "intercept"`) — implements `ToolInterceptor`, synchronous, can block/modify tool calls

### Wire Protocol
- **stdin**: JSON `commandHookInput` struct (event, hook_name, tool, arguments, turn_id, session, timestamp)
- **stdout**: JSON `commandHookOutput` struct (action, arguments, reason) — optional
- **exit codes**:
  - `0` → continue (or modify if output.Action == "modify" and output.Arguments != nil)
  - `1` → deny_tool (BeforeTool) or abort_turn (AfterTool); reason from output.Reason if present
  - `>=2` → error, propagated as Go error

### Key Types Used
- `CommandHookOptions` — Name, ScriptPath, Mode, Events map[string]struct{}, Tools map[string]struct{}, Timeout, Shell
- Default shell: `bash`
- Default timeout: 10s (intercept), 5s (observe)
- Event/tool filtering: empty map = match all; non-empty = exact key lookup

### Helper: extractToolInfo
Extracts (tool, arguments) from `ToolExecStartPayload` or `ToolExecEndPayload`; returns ("", nil) for other payload types.

## Test Results
All 7/7 PASS:
- `TestCommandHook_Observer_ReceivesJSON` — script receives JSON on stdin
- `TestCommandHook_Observer_EventFilter` — filtered events don't fire script
- `TestCommandHook_Observer_ToolFilter` — non-matching tools don't fire script
- `TestCommandHook_Observer_Timeout` — timeout suppressed (observer doesn't propagate errors)
- `TestCommandHook_Interceptor_Continue` — exit 0, no output → HookActionContinue
- `TestCommandHook_Interceptor_Deny` — exit 1 + JSON → HookActionDenyTool with reason
- `TestCommandHook_Interceptor_Modify` — exit 0 + modify JSON → modified Arguments

## Notes
- `writeTestScript` helper defined in test file; check for conflicts if other test files define it (none found at time of writing)
- Timeout test takes ~10s wall time because bash `sleep 10` is killed by context but `cmd.Run()` waits for process exit
- Next: Task 7 wires CommandHook loading into `hook_mount.go` config-driven path
