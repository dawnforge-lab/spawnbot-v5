# Spawnbot

A lightweight, self-evolving personal AI agent. Single binary, 16 channels, 25+ LLM providers, semantic memory, skills, MCP, and autonomous operation.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/dawnforge-lab/spawnbot-v5/main/scripts/install.sh | bash
```

Requires [Go 1.25+](https://go.dev/dl/). Installs to `~/.spawnbot/bin/` and adds to PATH automatically.

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

### Skills

Extensible capability system with 3-tier priority (workspace > global > builtin):

- **SKILL.md** with YAML frontmatter defines each skill's metadata, triggers, and instructions
- **Progressive disclosure**: metadata loaded at startup, body loaded on activation, resources loaded on demand
- **Skill creator**: Built-in skill with scaffolding scripts (`init_skill.py`, `package_skill.py`) and reference docs for creating new skills
- **ClawHub registry**: Remote skill search and installation

### Tools

Built-in tools with configurable approval modes (YOLO / approval / review):

| Tool | Purpose |
|------|---------|
| `read_file`, `write_file`, `edit_file`, `append_file` | Workspace file operations |
| `list_dir` | Directory listing |
| `exec`, `shell` | Shell command execution |
| `send_file`, `message` | Channel communication |
| `spawn` | Sub-agent parallel execution |
| `connect_mcp`, `disconnect_mcp`, `list_mcp` | Runtime MCP server management |
| `memory_store`, `memory_search` | Semantic memory operations |
| `search_tools` | Skill/tool discovery |
| `skills_install`, `skills_search` | Skill management |
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

- **Heartbeat**: Periodic prompt execution (configurable interval), with main session context injection for situational awareness
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
  skills/     Skill loading, discovery, registry
  memory/     JSONL session store + SQLite semantic memory
  session/    Session management (JSONL backend)
  config/     Configuration loading, validation, migration
  routing/    Model routing (light/heavy) + agent binding
  mcp/        MCP server lifecycle management
  autonomy/   Idle monitor, RSS poller
  heartbeat/  Periodic autonomous tasks
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
    HEARTBEAT.md         Periodic tasks
    memory/              Daily notes + long-term memory
      MEMORY.md
      YYYYMM/YYYYMMDD.md
    sessions/            Conversation history (JSONL)
    skills/              Installed skills
    state/               Persistent state
```

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

```bash
spawnbot onboard              # Interactive setup wizard
spawnbot agent                # CLI chat mode
spawnbot agent -m "message"   # Single message mode
spawnbot gateway              # Start all channels
spawnbot cron list            # List scheduled tasks
spawnbot skills install NAME  # Install skill from registry
spawnbot migrate              # Migrate config from older versions
spawnbot version              # Show version info
```

## License

MIT (based on [PicoClaw](https://github.com/sipeed/picoclaw) by Sipeed)
