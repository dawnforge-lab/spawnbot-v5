# Task 4: Remove Hardcoded Struggles from loop.go — COMPLETED

## Commit: f86974c
`refactor(hooks): remove hardcoded struggles from loop.go, mount via HookManager`

## Files Changed
- `pkg/agent/loop.go` — removed: `struggles` import, `struggles *struggles.Collector` field, `SetStruggleCollector` method, lazy-set block (`al.struggles != nil && agent.StruggleCollector == nil`), `HandleTurnStart` call, `toolCallCounts` map declaration and increment, `HandleToolEnd` call, `HandleTurnEnd` call
- `pkg/agent/instance.go` — removed: `struggles` import, `StruggleCollector *struggles.Collector` field
- `pkg/gateway/gateway.go` — replaced `agentLoop.SetStruggleCollector(struggleCollector)` with `agentLoop.MountHook(agent.NamedHook("struggles", agent.NewStrugglesObserver(struggleCollector)))` (with error return)
- `cmd/spawnbot/internal/agent/helpers.go` — same replacement as gateway.go; error logged via `logger.ErrorCF` (non-fatal since CLI path)

## Architecture After Task 4
- Struggles collector receives events: EventBus → HookManager → StrugglesObserver → Handle* methods
- No direct coupling between loop.go/instance.go and the struggles package
- `agent` package already imported in gateway.go; no new imports needed there
- `struggles` import kept in both gateway.go and helpers.go since `struggles.NewCollector` is still called there

## Test Results
- `go test ./pkg/struggles/ -v -count=1` → 15/15 PASS
- `go test ./pkg/agent/ -run TestStrugglesObserver -v -count=1` → 4/4 PASS
- `go build ./pkg/agent/... ./pkg/gateway/... ./cmd/spawnbot/internal/agent/...` → EXIT:0

## Notes
- `go build ./...` shows pre-existing embed errors in `pkg/workspace/deploy.go` and `web/backend/embed.go` (missing dist files) — not related to this task
- `toolCallCounts` was only used to increment per tool call and was not passed anywhere after Task 3 changes; safe to remove entirely
