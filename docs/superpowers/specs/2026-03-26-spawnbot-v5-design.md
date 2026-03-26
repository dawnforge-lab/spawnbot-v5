# Spawnbot v5 Design Spec

## Overview

Spawnbot v5 is a fork of spawnbot (Go, ~15MB binary, <10MB RAM) replacing the previous Goose-based Rust build (250MB binary). The fork is fully rebranded — module path, binary, config dirs, UI, all references.

**Repo:** `github.com/dawnforge-lab/spawnbot-v5`
**Module:** `github.com/dawnforge-lab/spawnbot-v5`

## Principles

- Build well, not fast — full scope, no shortcuts
- Simplicity is the USP — guided onboarding, no JSON fighting
- Prompt control is non-negotiable — SOUL.md is single source of truth
- No fallbacks — transparent errors that can be fixed
- Everything spawnbot has stays — no stripping

## Architecture

Spawnbot v5 inherits spawnbot's three-layer architecture:

```
Presentation:  Web UI (enhanced chat) | Telegram | CLI | Discord | Slack | ...
                         |
Business:      AgentLoop → ContextBuilder → Providers (25+)
               ToolRegistry (filesystem, shell, edit, web, spawn, mcp, memory)
               HookManager (observers, interceptors, approvers)
               SubTurn engine (async sub-agents, depth/concurrency limits)
                         |
Data:          SQLite (FTS5 + sqlite-vec) | JSONL sessions | Workspace files
```

### What spawnbot already provides

- 25+ LLM providers (Anthropic, OpenAI, OpenRouter, Gemini, LiteLLM, Ollama, etc.)
- Full MCP client (stdio, SSE, HTTP) with BM25 tool discovery
- 17 channels (Telegram, Discord, Slack, WhatsApp, Matrix, etc.)
- Coding tools: read_file, write_file, edit_file, list_dir, exec (shell with PTY/sandbox)
- Web tools: web_search (DuckDuckGo free, Brave, Tavily, etc.), web_fetch
- Sub-agents: spawn/spawn_status with SubTurn engine (depth limits, concurrency, ephemeral sessions)
- Cron scheduling with persistence
- Session history (JSONL) with auto-summarization
- Skills system (SKILL.md files in workspace)
- Identity bootstrap: SOUL.md, USER.md, IDENTITY.md loaded into system prompt
- Model routing (light vs heavy model auto-dispatch)
- Hook system: EventObserver, LLMInterceptor, ToolInterceptor, ToolApprover
- Web UI launcher (config, channels, gateway, chat) + TUI + CLI
- Voice transcription (ElevenLabs, Groq)
- Context caching (Anthropic ephemeral blocks, mtime-based invalidation)
- Streaming responses, thinking/reasoning support

### What we add

1. **Semantic memory system** (new `pkg/memory/sqlite.go`)
2. **Autonomy daemon** (idle triggers + feed polling)
3. **Enhanced web UI chat** (markdown rendering, tool call display, session sidebar)
4. **Onboarding wizard** (CLI + web, creates identity files + config)
5. **Two-mode approval** (YOLO / Approval, chosen at onboarding)
6. **Full rebrand** (spawnbot everywhere)

## 1. Identity System

Only SOUL.md is injected into the system prompt. It is the single source of truth for the agent's identity and contains references to the other files, which the agent reads via `read_file` when needed. This keeps the system prompt lean.

PicoClaw's (upstream) existing `LoadBootstrapFiles()` loads AGENT.md, AGENTS.md, SOUL.md, USER.md, and IDENTITY.md. We strip out AGENT.md, AGENTS.md, and IDENTITY.md entirely — remove the loading code, the `AgentDefinition` struct, `definition.go`, and any references. These concepts don't exist in Spawnbot and would create confusion. Only SOUL.md loading remains.

### Workspace files (created at onboarding)

| File | Injected into prompt | Writable by agent | Purpose |
|------|---------------------|-------------------|---------|
| `SOUL.md` | Yes | No (user-owned) | Agent personality, core directives, references to other files |
| `USER.md` | No (read on demand) | Yes | Who the user is, preferences |
| `GOALS.md` | No (read on demand) | Yes | Current objectives, priorities |
| `PLAYBOOK.md` | No (read on demand) | No (user-owned) | Standard operating procedures |
| `HEARTBEAT.md` | No (read on demand) | Yes | Autonomy triggers, proactive behaviors |

SOUL.md references the others by path — the agent knows they exist and reads them when relevant:

```markdown
# Spawnbot

You are Spawnbot, a personal AI agent for {user_name}.

## Reference Files
These files contain important context. Read them with read_file when relevant:
- workspace/USER.md — what you know about the user (you can update this)
- workspace/GOALS.md — current objectives (you can update this)
- workspace/PLAYBOOK.md — how you operate
- workspace/HEARTBEAT.md — your autonomy triggers and proactive behaviors
```

### Code changes

1. **`pkg/agent/context.go` — `LoadBootstrapFiles()`**: Rewrite to load only SOUL.md. Remove all AGENT.md/AGENTS.md/IDENTITY.md loading logic.

2. **`pkg/agent/definition.go`**: Delete entirely. The `AgentDefinition` struct and its `trackedPaths()` method are no longer needed.

3. **`pkg/agent/context.go` — `getIdentity()`**: Replace the hardcoded spawnbot identity string with spawnbot branding. The core rules (always use tools, be helpful, memory instructions) stay the same.

4. **`pkg/agent/context.go` — `sourcePaths()`**: Update to track SOUL.md directly instead of going through the deleted `AgentDefinition`.

## 2. Memory System

Replace spawnbot's flat MEMORY.md approach with a full indexed memory store backed by SQLite with FTS5 and sqlite-vec.

### Architecture

```
Workspace markdown files (MEMORY.md, daily notes, any .md)
        |
        v
  Markdown Indexer (pkg/memory/indexer.go)
  - Watches workspace for changes
  - Chunks by ## headings
  - SHA-256 change detection (skip unchanged chunks)
  - Sends new/changed chunks to store
        |
        v
  SQLite Store (pkg/memory/sqlite.go)
  - mattn/go-sqlite3 (CGO) with FTS5 enabled
  - asg017/sqlite-vec-go-bindings/cgo (vector search)
  - Note: spawnbot uses modernc.org/sqlite elsewhere — the memory
    package uses its own mattn/go-sqlite3 connection with a separate
    driver name ("sqlite3_memory") to avoid conflicts. The two drivers
    coexist via Go's database/sql driver registry.
  - FTS5 table for keyword search
  - vec0 table for embedding-based semantic search
  - Temporal decay scoring (newer = higher weight)
  - ULID primary keys + SHA-256 content dedup
        |
        v
  Embeddings Provider (pkg/memory/embeddings.go)
  - Configurable separately from chat model
  - HTTP call to Gemini/OpenAI/any embeddings API
  - f32 BLOB serialization for sqlite-vec
        |
        v
  Hybrid Search (pkg/memory/search.go)
  - Reciprocal Rank Fusion: FTS5 keyword + vec cosine
  - Configurable weights (default: 0.5 FTS / 0.5 vec)
  - Temporal decay multiplier
  - Returns ranked chunks with source attribution
```

### Database schema

```sql
CREATE TABLE memory_chunks (
    id TEXT PRIMARY KEY,           -- ULID
    source_file TEXT NOT NULL,     -- relative path to source markdown
    heading TEXT,                  -- ## heading this chunk belongs to
    content TEXT NOT NULL,         -- chunk text
    content_hash TEXT NOT NULL,    -- SHA-256 for dedup
    created_at TEXT NOT NULL,      -- ISO 8601
    updated_at TEXT NOT NULL,
    UNIQUE(content_hash)
);

CREATE VIRTUAL TABLE memory_fts USING fts5(
    content,
    content='memory_chunks',
    content_rowid='rowid'
);

-- Dimension is set at schema creation time from embeddings.dimensions config.
-- Changing embedding provider after DB creation requires re-indexing (drop + recreate vec table).
CREATE VIRTUAL TABLE memory_vec USING vec0(
    chunk_id TEXT PRIMARY KEY,
    embedding float[{dimensions}]  -- e.g. 768 for Gemini, 1536 for OpenAI small, 3072 for OpenAI large
);
```

### Agent tools

| Tool | Description |
|------|-------------|
| `memory_store` | Store a new memory (agent decides something is worth remembering) |
| `memory_search` | Hybrid search across all memories (keyword + semantic) |
| `memory_recall` | Retrieve memories by source file or heading |

These are registered as standard tools in the ToolRegistry alongside filesystem/shell/etc.

### Integration with system prompt

The `ContextBuilder.GetMemoryContext()` method is updated to query the SQLite store for recent/relevant memories instead of reading flat MEMORY.md. The top-N most relevant memories are injected into the system prompt's memory section.

The `sourcePaths()` method in `context.go` is updated to track the SQLite DB file path (`workspace/memory/spawnbot.db`) instead of `MEMORY.md`, so the mtime-based prompt cache invalidates correctly when memories are written.

Daily notes (`memory/YYYYMM/YYYYMMDD.md`) continue to work as-is — the indexer picks them up automatically.

## 3. Autonomy

### Cron (existing)

PicoClaw's (upstream) cron system stays as-is. Supports one-time reminders, recurring tasks, cron expressions, and direct shell command execution.

### Idle triggers (new)

A lightweight timer in the gateway that fires when no user interaction has occurred for a configurable duration (e.g., 8 hours). When triggered, it sends an internal message to the agent: "It's been {duration} since last interaction. Check if there's anything proactive to do based on GOALS.md."

Implementation: add an `IdleMonitor` to `pkg/gateway/` that tracks last interaction timestamp per channel and fires after threshold.

### Feed polling (new)

A daemon goroutine in the gateway that periodically checks configured RSS/Atom feeds and other data sources. When new items are found, it invokes the agent with a summary of new content. The agent decides what to do with it (notify user, store in memory, ignore).

Key design: **the poller activates the agent, not the agent doing the polling.** This saves tokens — no LLM calls for empty feeds.

Implementation: `pkg/autonomy/poller.go` using `mmcdole/gofeed` (Go RSS/Atom parser). Config-driven feed list in `config.json` under `autonomy.feeds`.

```json
{
  "autonomy": {
    "idle_trigger": {
      "enabled": true,
      "threshold_hours": 8
    },
    "feeds": [
      {
        "url": "https://example.com/feed.xml",
        "check_interval_minutes": 30,
        "notify_channel": "telegram",
        "notify_chat_id": "123456789"
      }
    ]
  }
}
```

## 4. Approval Modes

Two modes, chosen at onboarding:

| Mode | Behavior |
|------|----------|
| **YOLO** | All tools auto-approved. Agent acts autonomously. |
| **Approval** | Dangerous tools (exec, write_file, edit_file) require user confirmation via Telegram or web UI before execution. |

Implementation: uses spawnbot's existing `ToolApprover` hook. A built-in approver checks the mode from config. In Approval mode, it sends a confirmation request to the user's active channel and blocks until approved/rejected.

### Timeout behavior

PicoClaw's (upstream) hook system has a default 60-second approval timeout (`hooks.defaults.approval_timeout_ms`). For human-in-the-loop approval, this is too short. The built-in approver overrides the timeout to 5 minutes (configurable). On timeout, the tool is denied with a user-visible notification: "Tool {name} was denied — approval timed out after {duration}."

Stored in config:

```json
{
  "agents": {
    "defaults": {
      "approval_mode": "yolo",
      "approval_timeout_seconds": 300
    }
  }
}
```

## 5. Onboarding

Two entry points, same flow:

### CLI (`spawnbot onboard`)

Terminal wizard using `charmbracelet/huh` (Go interactive forms library, added to `go.mod`):

1. **Provider** — pick from: OpenRouter (recommended), Anthropic, OpenAI, or custom
2. **API key** — enter and validate
3. **Your name** — "What should I call you?"
4. **Agent personality** — brief description or use default SOUL.md
5. **Approval mode** — YOLO or Approval
6. **Telegram** (optional) — enter bot token to connect
7. **Embedding provider** — pick provider + API key for memory embeddings (can be same or different from chat)

Writes: `config.json`, `.security.yml`, workspace files (SOUL.md, USER.md, GOALS.md, PLAYBOOK.md, HEARTBEAT.md)

### Web wizard

First run of web UI detects no config and shows a step-by-step setup flow with the same steps. Same config files written.

## 6. Web UI Enhancements

Keep spawnbot's existing management pages (config, channels, models, credentials, logs, skills, tools). Enhance the chat page:

### Chat page redesign

- **Session history sidebar** — list of past sessions, click to load
- **Markdown rendering** — proper rendering of agent responses (code blocks, lists, headers)
- **Tool call display** — collapsible cards showing tool name, input, output, status
- **Streaming** — real-time token streaming display
- **Status indicators** — agent thinking, tool executing, sub-agent spawned

The existing Pico WebSocket channel and REST API provide the backend — this is a frontend enhancement.

## 7. Embeddings Provider

Configured separately from the chat model in config:

```json
{
  "embeddings": {
    "provider": "gemini",
    "model": "text-embedding-004",
    "api_key_env": "GEMINI_API_KEY",
    "dimensions": 768
  }
}
```

Supported providers (via simple HTTP calls, no SDK needed):
- Gemini (free tier available)
- OpenAI
- Any OpenAI-compatible endpoint (e.g., local models via Ollama)

Implementation: `pkg/memory/embeddings.go` with a `EmbeddingProvider` interface and factory.

## 8. Full Rebrand

### Scope

| What | From | To |
|------|------|----|
| Go module | `github.com/dawnforge-lab/spawnbot-v5` | `github.com/dawnforge-lab/spawnbot-v5` |
| Binary | `spawnbot` | `spawnbot` |
| Config dir | `~/.spawnbot/` | `~/.spawnbot/` |
| Config env prefix | `SPAWNBOT_*` | `SPAWNBOT_*` |
| System prompt identity | "spawnbot" | "spawnbot" |
| Web UI title/branding | Spawnbot | Spawnbot |
| Package references | All `spawnbot` imports | All `spawnbot` imports |

### Method

Global find-replace across all Go files, configs, frontend, and documentation. Then `go mod tidy` to verify.

## Non-goals for v5

- No changes to spawnbot's provider system (25+ providers work as-is)
- No changes to MCP client (works as-is)
- No changes to sub-agent system (spawn/SubTurn work as-is, sub-agents intentionally don't get SOUL)
- No stripping of any spawnbot features (all channels, tools, providers stay)
- No custom TUI (web UI is the primary interface, CLI for headless)

## File changes summary

### New files

| Path | Purpose |
|------|---------|
| `pkg/memory/sqlite.go` | SQLite store with FTS5 + sqlite-vec |
| `pkg/memory/indexer.go` | Markdown chunker with SHA-256 change detection |
| `pkg/memory/search.go` | Hybrid search (RRF: FTS5 + vec + temporal decay) |
| `pkg/memory/embeddings.go` | Embedding provider interface + implementations |
| `pkg/memory/tools.go` | memory_store, memory_search, memory_recall tools |
| `pkg/autonomy/idle.go` | Idle trigger monitor |
| `pkg/autonomy/poller.go` | Feed polling daemon |
| `pkg/autonomy/config.go` | Autonomy configuration |

### Modified files

| Path | Change |
|------|--------|
| `pkg/agent/context.go` | Update `getIdentity()` for spawnbot branding, simplify `LoadBootstrapFiles()` to SOUL.md only, update `GetMemoryContext()` to use SQLite store, update `sourcePaths()` to track SQLite DB file |
| `pkg/agent/instance.go` | Register memory tools in ToolRegistry |
| `pkg/agent/approval.go` | New file: built-in ToolApprover for YOLO/Approval modes with configurable timeout (registered via instance.go) |
| `pkg/gateway/gateway.go` | Wire idle monitor + feed poller |
| `pkg/config/config.go` | Add autonomy + embeddings + approval config sections |
| `pkg/config/defaults.go` | Default values for new config |
| `cmd/spawnbot/internal/onboard/` | New onboarding wizard using charmbracelet/huh |
| `cmd/spawnbot/main.go` | Rename entry point |
| `web/frontend/` | Enhanced chat page + web onboarding wizard |
| `go.mod` | New module path + mattn/go-sqlite3 + sqlite-vec + charmbracelet/huh + mmcdole/gofeed |
| Every file | `dawnforge-lab/spawnbot-v5` → `dawnforge-lab/spawnbot-v5` |
