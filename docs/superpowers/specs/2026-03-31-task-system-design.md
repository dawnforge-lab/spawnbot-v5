# Task System

**Date:** 2026-03-31
**Status:** Approved
**Scope:** New `pkg/tasks/` package, task store with JSON persistence, `tasks` tool, system prompt integration, heartbeat integration

## Overview

Spawnbot currently has no way to track work across sessions. Subagent tasks are in-memory only (lost on restart), cron jobs are scheduling-only, and HEARTBEAT.md is a static checklist. This design adds a persistent task system — the agent can create tasks, track them across conversations and heartbeats, and follow up autonomously.

## Task Data Model

```go
type Task struct {
    ID          string `json:"id"`          // auto-generated, 8 hex chars
    Title       string `json:"title"`       // required, short description
    Description string `json:"description"` // optional, detailed context
    Status      string `json:"status"`      // pending | in_progress | completed | failed
    Priority    string `json:"priority"`    // low | medium | high (default: medium)
    AgentType   string `json:"agent_type"`  // which agent worked on it (optional)
    Result      string `json:"result"`      // outcome when completed/failed
    CreatedAt   int64  `json:"created_at"`  // unix milliseconds
    UpdatedAt   int64  `json:"updated_at"`  // unix milliseconds
}
```

### Status Transitions

- `pending` → `in_progress` → `completed` | `failed`
- `pending` → `completed` (skip in_progress for quick tasks)
- `pending` → `failed` (couldn't even start)
- No transition back from `completed`/`failed` — create a new task instead
- `update` action rejects changes to tasks with status `completed` or `failed`

### Flat Structure

No parent/child relationships, no dependencies. The planner agent breaks complex work into separate flat tasks.

### TTL Cleanup

Completed and failed tasks are automatically removed when older than 7 days. Cleanup runs at load time (startup) and checked before each save.

## Task Store (`pkg/tasks/`)

### Package Structure

```
pkg/tasks/
  task.go        # Task struct, validation, status constants
  store.go       # TaskStore — CRUD, persistence, TTL cleanup
  store_test.go  # Tests
  summary.go     # Summary generation for system prompt + heartbeat
```

### TaskStore

```go
type TaskStore struct {
    mu       sync.RWMutex
    tasks    map[string]*Task
    filePath string           // e.g., ~/.spawnbot/workspace/tasks.json
    warning  string           // set if tasks.json failed to load
}
```

### Operations

| Method | Description |
|--------|-------------|
| `NewTaskStore(filePath)` | Creates store, loads from disk, runs TTL cleanup |
| `Create(title, description, priority)` | Creates task with auto-generated ID, saves to disk |
| `Get(id)` | Lookup by ID, returns nil if not found |
| `List(status)` | List tasks, optionally filtered by status (empty = all) |
| `Update(id, fields)` | Update status, result, agent_type, description |
| `Complete(id, result)` | Shortcut: set status=completed + result |
| `Fail(id, result)` | Shortcut: set status=failed + result |
| `Save()` | Atomic write to JSON file (tmp + rename) |
| `Load()` | Read from disk, apply TTL cleanup |
| `Summary(maxFull)` | Formatted summary for system prompt injection |
| `PendingSummary()` | Compact summary for heartbeat injection |

### Persistence

Single `tasks.json` file at `~/.spawnbot/workspace/tasks.json`. Atomic writes using tmp file + rename (same pattern as cron and sessions). Load on startup, save after every mutation.

### TTL Cleanup

On `Load()`, remove any completed/failed tasks where `UpdatedAt` is older than 7 days. Save immediately if anything was cleaned up.

## The `tasks` Tool

Single tool with `action` parameter, matching the existing `cron` tool pattern.

### Actions

| Action | Required Params | Optional Params | Description |
|--------|----------------|-----------------|-------------|
| `create` | `title` | `description`, `priority` | Create a new task |
| `list` | | `status` | List tasks, optionally filtered |
| `get` | `id` | | Get full task details |
| `update` | `id` | `status`, `description`, `result`, `agent_type` | Update task fields |
| `complete` | `id` | `result` | Set status=completed + result |
| `fail` | `id` | `result` | Set status=failed + result |

### Tool Input Examples

```json
{
  "action": "create",
  "title": "Optimize database query performance",
  "priority": "high"
}
```

```json
{
  "action": "complete",
  "id": "a3f8b2c1",
  "result": "Rewrote query to use index scan, 10x faster"
}
```

## System Prompt Integration

In `BuildSystemPrompt()`, after the agents summary, inject task summary via `cb.taskStore.Summary(10)`.

### Summary Format (under 10 tasks — full list)

```
Active tasks:
- [a3f8b2c1] HIGH Optimize database queries (in_progress)
- [b7e2d4f0] MEDIUM Set up monitoring alerts (pending)
- [c1a9e3b5] LOW Clean up old log files (pending)
```

### Summary Format (10+ tasks — count + top 5)

```
You have 14 tasks (3 in_progress, 8 pending, 2 failed, 1 completed).

Top 5 by priority:
- [a3f8b2c1] HIGH Optimize database queries (in_progress)
- [d4c2e8f1] HIGH Fix auth token expiry (pending)
- [b7e2d4f0] MEDIUM Set up monitoring alerts (pending)
- [e5a1b3c7] MEDIUM Write API documentation (in_progress)
- [c1a9e3b5] MEDIUM Deploy staging environment (pending)

Use tasks list for full details.
```

### Sort Order

in_progress first, then pending, then failed, then completed. Within each status group, high → medium → low.

### Load Warning

If `tasks.json` is corrupted, the summary includes:
```
WARNING: tasks.json failed to load: <detail>. The file is at ~/.spawnbot/workspace/tasks.json — inspect and fix it.
```

The agent sees this in context and can use `read_file`/`write_file` to fix the corruption.

## Heartbeat Integration

When building the heartbeat prompt, after HEARTBEAT.md content, append pending task context:

```go
if taskStore != nil {
    pending := taskStore.PendingSummary()
    if pending != "" {
        prompt += "\n\n# Pending Tasks\n\n" + pending +
            "\n\nReview these tasks. Follow up on any that need action — " +
            "start working on pending tasks, check on in_progress tasks, " +
            "or mark tasks as completed/failed."
    }
}
```

### PendingSummary Format

Only pending + in_progress tasks (no completed/failed):

```
- [a3f8b2c1] HIGH Optimize database queries (in_progress)
- [b7e2d4f0] MEDIUM Set up monitoring alerts (pending)
```

### No Automatic Execution

The heartbeat presents tasks to the agent. The agent decides what to do — start work, spawn subagents, update status, or skip. No magic background processing.

## Error Handling

All errors returned as tool results to the main agent. Transparent errors, no fallbacks.

| Error | Behavior |
|-------|----------|
| Task not found | ErrorResult: `"task 'abc123' not found"` |
| Invalid action | ErrorResult: `"unknown action 'foo'. Valid: create, list, get, update, complete, fail"` |
| Missing required field | ErrorResult: `"title is required for create action"` |
| Invalid status | ErrorResult: `"invalid status 'done'. Valid: pending, in_progress, completed, failed"` |
| Invalid priority | ErrorResult: `"invalid priority 'urgent'. Valid: low, medium, high"` |
| File write failure | ErrorResult: `"failed to save tasks: <detail>"` |
| Invalid status transition | ErrorResult: `"cannot update completed task 'abc123' — create a new task instead"` |
| Corrupted tasks.json at startup | Log warning, start with empty store, inject warning into system prompt summary so agent can fix it |

## What This Design Does NOT Include

- **Parent/child task hierarchy** — flat tasks only, planner agent decomposes
- **Task dependencies / blockedBy** — not needed for autonomous operation
- **Task assignment to specific agents** — agent_type tracks who worked, not who should
- **Due dates / deadlines** — can be added later if needed
- **Task notifications / reminders** — heartbeat handles follow-up
- **Web UI for tasks** — future feature
