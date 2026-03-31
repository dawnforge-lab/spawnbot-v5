# Spawnbot

A lightweight, self-evolving personal AI agent. Single binary, 16 channels, 25+ LLM providers, semantic memory, skills, MCP, and autonomous operation.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/dawnforge-lab/spawnbot-v5/main/scripts/install.sh | bash
```

Installs to `~/.spawnbot/bin/` and adds to PATH automatically. Go is installed locally if not found.

Then:

```bash
spawnbot onboard          # Interactive setup wizard
spawnbot agent -m "Hello" # Chat via CLI
spawnbot gateway          # Start all channels
```

## Features

### Multi-Provider LLM Support

25+ providers through protocol-based routing. Configure multiple models with automatic fallback chains:

| Protocol | Providers |
|----------|-----------|
| OpenAI-compatible | OpenAI, OpenRouter, Groq, Ollama, LiteLLM, vLLM, DeepSeek, Mistral, Moonshot, Cerebras, NVIDIA, Qwen, VolcEngine |
| Anthropic native | Anthropic (Claude) |
| Azure | Azure OpenAI (MSI + key auth) |
| AWS | Bedrock |
| Other | GitHub Copilot, Antigravity (Google Cloud OAuth) |

**Model routing**: Automatically routes simple queries to a cheaper/faster light model and complex queries to the primary model based on a configurable complexity threshold.

**Fallback chains**: If the primary provider fails, the system automatically tries fallback models with cooldown tracking and round-robin load balancing.

### Communication Channels (16)

All channels share a unified message bus. Each can be independently enabled:

| Channel | | Channel | | Channel |
|---------|---|---------|---|---------|
| Telegram | | Discord | | Slack |
| WhatsApp | | Matrix (E2EE) | | WeChat |
| Wecom | | DingTalk | | Feishu |
| QQ | | Line | | IRC |
| OneBot | | Pico (Web) | | MaixCam |
| WebSocket | | | | |

### Identity System

The agent's personality and rules live in markdown files in the workspace:

| File | Purpose |
|------|---------|
| `SOUL.md` | Core identity, personality, rules (guarded against self-modification) |
| `USER.md` | User profile, preferences, timezone |
| `GOALS.md` | Current objectives and priorities |
| `PLAYBOOK.md` | Standard operating procedures, skill creation guides |
| `HEARTBEAT.md` | Periodic autonomous tasks |

### Memory (3-Tier)

1. **Session history** (JSONL) -- Per-conversation message logs with append-only crash-safe storage. Two-tier compaction: proactive summarization at 20 messages or 75% context window, emergency compression as fallback.

2. **Daily notes** (`memory/YYYYMM/YYYYMMDD.md`) + long-term memory (`memory/MEMORY.md`) -- Key facts automatically flushed from conversation before compaction and every 15 messages. LLM-based deduplication prevents redundant entries. Included in agent context via on-demand tool reads.

3. **Semantic memory** (SQLite + vector embeddings, requires CGO) -- FTS5 full-text search plus vector similarity with temporal decay scoring. Configurable embedding provider (Gemini, OpenAI).

### Task Tracking

Persistent task system for tracking work across sessions and heartbeats:

- **JSON-backed store** (`~/.spawnbot/workspace/tasks.json`) with atomic writes
- **Single `tasks` tool** with actions: `create`, `list`, `get`, `update`, `complete`, `fail`
- **System prompt integration**: active tasks shown in agent context (full list under 10, count + top 5 above)
- **Heartbeat integration**: pending tasks injected into heartbeat prompt for autonomous follow-up
- **7-day TTL**: completed/failed tasks auto-cleaned on startup
- **Agent tracking**: records which agent type worked on each task
- **Corruption recovery**: if `tasks.json` is corrupted, warning shown in system prompt so the agent can fix it

### Skills

Extensible capability system with 3-tier priority (workspace > global > builtin):

- **SKILL.md** with YAML frontmatter defines each skill's metadata, triggers, and instructions
- **Progressive disclosure**: metadata loaded at startup, body loaded on activation, resources loaded on demand
- **Skill creator**: Built-in skill with scaffolding scripts (`init_skill.py`, `package_skill.py`) and reference docs for creating new skills
- **ClawHub registry**: Remote skill search and installation

### Agent Definitions

Specialized subagents defined as markdown files with YAML frontmatter. Each agent type has its own system prompt, tool restrictions, model override, and execution limits.

**4 builtin agents:**

| Agent | Purpose | Tool Access |
|-------|---------|-------------|
| `researcher` | Gather information without side effects | Read-only (no writes, no exec, no messaging) |
| `coder` | Write code, scripts, config files | Full file + exec access, no messaging |
| `reviewer` | Review work for bugs and security issues | Read-only |
| `planner` | Break down tasks into structured plans | Read + write (for plan files), no exec |

**Custom agents:** Create new agent types in `~/.spawnbot/workspace/agents/<name>/AGENT.md` or let the agent create them autonomously via the `create_agent` tool.

**AGENT.md format:**
```yaml
---
name: sql-expert
description: Specializes in database queries and schema design
tools_deny:
  - message
  - send_file
max_iterations: 20
timeout: 5m
---

You are a SQL expert agent. Focus on database queries and schema design...
```

Spawn agents via tools: `spawn` (async) or `subagent` (sync) with `agent_type` parameter. Workspace agents override builtins with the same name.

### Tools

Built-in tools with configurable approval modes (YOLO / approval / review):

| Tool | Purpose |
|------|---------|
| `read_file`, `write_file`, `edit_file`, `append_file` | Workspace file operations |
| `list_dir` | Directory listing |
| `exec`, `shell` | Shell command execution |
| `send_file`, `message` | Channel communication |
| `spawn`, `subagent` | Sub-agent execution (async/sync) with agent type selection |
| `create_agent` | Create new agent type definitions at runtime |
| `connect_mcp`, `disconnect_mcp`, `list_mcp` | Runtime MCP server management |
| `memory_store`, `memory_search` | Semantic memory operations |
| `search_tools` | Skill/tool discovery |
| `skills_install`, `skills_search` | Skill management |
| `tasks` | Persistent task tracking (create, list, update, complete, fail) |
| `cron` | Scheduled task management |
| `i2c`, `spi` | Hardware I/O (Linux embedded) |

### MCP (Model Context Protocol)

Connect any MCP server at runtime -- no restart required:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "postgres",
        "command": ["python", "-m", "mcp_postgres"],
        "args": ["--db-url", "postgresql://..."],
        "transport": "stdio"
      }
    ]
  }
}
```

- Supports stdio, SSE, and HTTP transports
- Hot-reload: the agent can `connect_mcp` / `disconnect_mcp` during conversation
- Tool discovery with TTL-based promotion
- Environment variable injection

### Autonomy

The agent can operate without user interaction:

- **Heartbeat**: Periodic prompt execution (configurable interval, minimum 5 minutes, default 30 minutes), with main session context injection for situational awareness
  - **HEARTBEAT_OK suppression**: When the agent has nothing to report, it replies `HEARTBEAT_OK` which is silently suppressed instead of sent to the user
  - **Deduplication**: Identical alert messages are suppressed within a 24-hour window to prevent spam
  - **Structured events**: Every heartbeat run emits a structured event (`sent`, `ok`, `skipped`, `failed`) with duration, preview, skip reason, and channel -- enabling monitoring and UI integration
  - **Retry**: A retry channel allows re-triggering heartbeats when the main agent queue clears
  - **Runtime interval changes**: The heartbeat interval can be changed at runtime via `SetInterval()` or the CLI
- **Self-improvement loop**: Heartbeat reviews recent conversations for repeated patterns, missing tools, and struggles -- creates skills or MCP servers to address them
- **Idle triggers**: Fire after channel inactivity threshold
- **RSS polling**: Monitor feeds, summarize new items
- **Cron scheduling**: Standard cron expressions for recurring tasks

### Security

- **Credential encryption**: AES-256-GCM with SSH key derivation (HKDF-SHA256). Supports plaintext, file refs, encrypted, and environment variable formats
- **Workspace restrictions**: Tools confined to workspace by default, configurable read/write path whitelists
- **Approval modes**: YOLO (auto-approve), Approval (ask before dangerous ops), Review (audit all)
- **Sensitive data filtering**: Credentials stripped from tool output before sending to LLM

### Voice

Audio transcription via Groq Whisper, ElevenLabs Scribe, or audio-capable LLM models (GPT-4o-audio).

### Web UI

React SPA (Vite + TanStack Router) with:
- Streaming chat interface
- Configuration editor with validation
- Credential management
- Model/provider setup
- Skill browser
- Channel configuration
- Onboarding wizard
- Real-time log viewer

## Architecture

```
cmd/spawnbot/          CLI entry point (Cobra)
cmd/spawnbot-launcher-tui/  TUI launcher
web/backend/           Go HTTP + WebSocket server
web/frontend/          React SPA (embedded in binary)

pkg/
  agent/      Core loop, turn state machine, context building, memory flush
  bus/        Inbound/outbound message routing
  channels/   16 communication adapters
  providers/  25+ LLM provider implementations + fallback chains
  tools/      Tool registry + built-in tools + MCP wrappers
  agents/     Agent definitions, registry, AGENT.md loader, builtins
  tasks/      Persistent task store, summary generation, TTL cleanup
  skills/     Skill loading, discovery, registry
  memory/     JSONL session store + SQLite semantic memory
  session/    Session management (JSONL backend)
  config/     Configuration loading, validation, migration
  routing/    Model routing (light/heavy) + agent binding
  mcp/        MCP server lifecycle management
  autonomy/   Idle monitor, RSS poller
  heartbeat/  Periodic autonomous tasks (dedup, events, HEARTBEAT_OK suppression)
  voice/      Audio transcription
  identity/   SOUL.md loading
  credential/ SSH-key encryption
  workspace/  Workspace deployment (shared CLI + web)
  state/      Persistent channel state
  commands/   Built-in slash commands
```

### Message Flow

```
Channel (Telegram, Discord, web, CLI, ...)
    |
MessageBus (inbound)
    |
AgentLoop.Incoming()
    |-- Resolve agent via bindings
    |-- Load session history + summary
    |-- Build context (identity + skills + tools + history)
    |-- Route to light or heavy model
    |
    |  TURN LOOP:
    |    LLM call --> tool calls --> execute --> feed results back
    |    (up to MaxIterations)
    |
    |-- Save session
    |-- Maybe summarize (background)
    |-- Maybe flush memory (every 15 msgs)
    |
MessageBus (outbound)
    |
Channel.Send()
```

## Configuration

```
~/.spawnbot/
  config.json            Main configuration
  .security.yml          Encrypted credentials
  workspace/
    SOUL.md              Agent identity
    USER.md              User profile
    GOALS.md             Objectives
    PLAYBOOK.md          Operating procedures
    HEARTBEAT.md         Periodic tasks (user-editable checklist)
    heartbeat.log        Heartbeat execution log
    memory/              Daily notes + long-term memory
      MEMORY.md
      YYYYMM/YYYYMMDD.md
    sessions/            Conversation history (JSONL)
      heartbeat/         Isolated heartbeat session store
    skills/              Installed skills
    agents/              Custom agent definitions (AGENT.md)
    state/               Persistent state
```

### Heartbeat Configuration

In `config.json`:

```json
{
  "heartbeat": {
    "enabled": true,
    "interval": 30
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable/disable the heartbeat service |
| `interval` | int | `30` | Interval in minutes between heartbeat runs (minimum 5) |

Environment variables: `SPAWNBOT_HEARTBEAT_ENABLED`, `SPAWNBOT_HEARTBEAT_INTERVAL`

The heartbeat reads tasks from `HEARTBEAT.md` in the workspace. If the file is missing, a default template is created. If the file has no user tasks (nothing below the marker line), the heartbeat is silently skipped. The agent runs with a lightweight clone (5 iterations max, read-only tools: `read_file`, `list_dir`, `message`) to keep executions fast and safe.

## Building from Source

```bash
git clone https://github.com/dawnforge-lab/spawnbot-v5.git
cd spawnbot-v5

make build              # Build binary to ./build/
make install            # Build + install to ~/.spawnbot/bin/

# Full build with semantic memory (requires SQLite dev headers)
CGO_ENABLED=1 make build

# Cross-compile all platforms
make build-all

# Web launcher (includes React frontend)
make build-launcher

# Docker
make docker-build       # Minimal Alpine
make docker-build-full  # With Node.js for MCP servers
```

**Supported platforms**: linux/amd64, linux/arm64, linux/arm, linux/riscv64, linux/loong64, linux/mipsle, darwin/arm64, windows/amd64, netbsd/amd64, netbsd/arm64

**Build tags**: `goolm` (Matrix crypto), `stdjson` (standard JSON), `fts5` (SQLite FTS), `whatsapp_native` (native WhatsApp)

## CLI Commands

### Core

| Command | Description |
|---------|-------------|
| `spawnbot onboard` | Interactive first-run setup wizard |
| `spawnbot agent` | CLI chat mode (interactive) |
| `spawnbot agent -m "message"` | Single message mode (non-interactive) |
| `spawnbot gateway` | Start all channels and background services |
| `spawnbot status` | Show agent status |
| `spawnbot version` | Show version and build info |
| `spawnbot migrate` | Migrate config from older versions |

### Heartbeat

| Command | Description |
|---------|-------------|
| `spawnbot heartbeat status` | Show current heartbeat configuration (enabled, interval) |
| `spawnbot heartbeat set-interval -m <minutes>` | Set heartbeat interval in minutes (minimum 5). Requires gateway restart. |

### Scheduled Tasks (Cron)

| Command | Description |
|---------|-------------|
| `spawnbot cron list` | List all scheduled tasks |
| `spawnbot cron add` | Add a new scheduled task |
| `spawnbot cron remove <id>` | Remove a scheduled task |
| `spawnbot cron enable <id>` | Enable a scheduled task |
| `spawnbot cron disable <id>` | Disable a scheduled task |

### Skills

| Command | Description |
|---------|-------------|
| `spawnbot skills install <name>` | Install a skill from the registry |

### Model Configuration

| Command | Description |
|---------|-------------|
| `spawnbot model` | Manage model configuration |

### Authentication

| Command | Description |
|---------|-------------|
| `spawnbot auth` | Manage authentication and credentials |

### Reset & Uninstall

| Command | Description |
|---------|-------------|
| `spawnbot reset` | Delete all configs, memories, sessions, skills, and cron jobs. Keeps the binary and Go runtime so you can re-run `spawnbot onboard`. Stops and removes systemd services. |
| `spawnbot reset -y` | Same as above, skip confirmation prompt |
| `spawnbot nuke` | Completely remove spawnbot from the system: deletes `~/.spawnbot/` entirely, removes systemd services, SSH encryption key, and PATH entry from shell rc files. |
| `spawnbot nuke -y` | Same as above, skip confirmation prompt |

## License

MIT (based on [PicoClaw](https://github.com/sipeed/picoclaw) by Sipeed)
