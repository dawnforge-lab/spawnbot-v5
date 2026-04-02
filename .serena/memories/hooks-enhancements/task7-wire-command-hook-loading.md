# Task 7: Wire CommandHook into Config-Driven Loading — COMPLETED

## Commit: 1b66fea
`feat(hooks): wire CommandHook into config-driven loading`

## File Changed
- `pkg/agent/hook_mount.go`

## Changes

### Import added
`"path/filepath"` added to the import block.

### `loadConfiguredHooks` — new block (after process hooks loop)
Iterates `enabledCommandHookNames(al.cfg.Hooks.Commands)` and for each:
1. Calls `commandHookFromConfig(name, spec, al.cfg.WorkspacePath())` to build a `*CommandHook`
2. Mounts via `al.MountHook(HookRegistration{Source: HookSourceInProcess, ...})`
3. Appends name to `mounted` (cleaned up on error via the deferred rollback)

### New helpers added (after `validHookEventKinds`)

**`enabledCommandHookNames(specs map[string]config.CommandHookConfig) []string`**
- Returns sorted list of hook names where `spec.Enabled == true`
- Returns nil for empty map

**`commandHookFromConfig(name string, spec config.CommandHookConfig, workspace string) (*CommandHook, error)`**
- Validates: script non-empty, mode is "observe" or "intercept"
- Resolves relative script paths to `filepath.Join(workspace, scriptPath)`
- Validates each event in spec.Events against `validHookEventKinds()`
- Builds `events map[string]struct{}` and `tools map[string]struct{}`
- Converts `spec.TimeoutMS` to `time.Duration`
- Calls `NewCommandHook(CommandHookOptions{...})` and returns it

## Build & Test
- `go build ./pkg/agent/...` — clean, no errors
- `go test ./pkg/agent/ -run TestHookMount -v -count=1` — PASS (no matching tests yet; tests will be added in Task 10)

## Notes
- Pattern exactly mirrors the builtin/process hook loading blocks
- Next: Task 8 implements the `create_hook` tool
