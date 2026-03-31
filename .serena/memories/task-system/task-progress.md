# Task System Worktree — Task Progress

Worktree: `/home/eugen-dev/Workflows/picoclaw/.worktrees/task-system`
Branch: `feature/task-system`

## Task Status

- [x] Task 1: Task Struct + Validation — DONE (commit e211a33)
  - Created `pkg/tasks/task.go` — Task struct, Validate(), ApplyDefaults(), IsTerminal(), IsExpired(), GenerateID()
  - Created `pkg/tasks/task_test.go` — 7 tests, all passing
  - Fixed `.gitignore`: scoped `tasks/` to `/tasks/` so `pkg/tasks/` is tracked
- [x] Task 2: TaskStore CRUD + persistence — DONE (commit c1c169f)
  - Created `pkg/tasks/store.go` — TaskStore with Create, Get, List, Update, Complete, Fail, Load, Save, TTL cleanup on load
  - Created `pkg/tasks/store_test.go` — 11 tests, all passing
  - Atomic save via .tmp + rename; warning field captures load errors
- [x] Task 3: Summary generation — DONE
- [x] Task 4: Tasks tool — DONE (commit 5f8e0c2)
  - Created `pkg/tools/tasks_tool.go` — TasksTool with create, list, get, update, complete, fail actions
  - Created `pkg/tools/tasks_tool_test.go` — 8 tests, all passing
  - Uses `joinLines` helper; follows same Execute/ToolResult pattern as other tools
- [ ] Task 5: Integrate into AgentInstance + system prompt
- [ ] Task 6: Heartbeat integration
- [ ] Task 7: Integration tests
- [ ] Task 8: Full build verification

## Key Design Decisions
- Status constants: pending, in_progress, completed, failed
- Priority constants: low, medium, high (default: medium)
- TTL: 7 days for terminal tasks (completed/failed)
- ID: 8-char hex from 4 random bytes
- Timestamps: UnixMilli (int64)
