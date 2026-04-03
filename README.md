# Spawnbot

A lightweight, self-evolving personal AI agent. Single binary, 16 channels, 25+ LLM providers, semantic memory, skills, and autonomous operation.

## Quick Start

```bash
curl -fsSL https://raw.githubusercontent.com/dawnforge-lab/spawnbot-v5/main/scripts/install.sh | bash
spawnbot onboard
spawnbot gateway
```

## CLI Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `spawnbot onboard` | Interactive first-run setup wizard |
| `spawnbot gateway` | Start gateway (all channels + background services) |
| `spawnbot gateway -d` | Start gateway with debug logging |
| `spawnbot agent` | Interactive CLI chat |
| `spawnbot agent -m "msg"` | Single message (non-interactive) |
| `spawnbot agent --model <name>` | Chat with a specific model |
| `spawnbot status` | Show agent status |
| `spawnbot version` | Show version and build info |
| `spawnbot model` | Show or change default model |
| `spawnbot auth` | Manage authentication and credentials |
| `spawnbot migrate` | Migrate config from older versions |

### Service Management

Spawnbot runs as two systemd user services:

| Service | Port | Purpose |
|---------|------|---------|
| `spawnbot-gateway.service` | 18790 | Gateway (API, channels, agent) |
| `spawnbot-web.service` | 18800 | Web UI (React SPA) |

```bash
# Start / stop / restart
systemctl --user start spawnbot-gateway
systemctl --user stop spawnbot-gateway
systemctl --user restart spawnbot-gateway

# Same for web UI
systemctl --user restart spawnbot-web

# Check status
systemctl --user status spawnbot-gateway
systemctl --user status spawnbot-web

# View logs
journalctl --user -u spawnbot-gateway -f
journalctl --user -u spawnbot-web -f

# Enable on boot
systemctl --user enable spawnbot-gateway spawnbot-web
```

Health check: `curl http://localhost:18790/health`

### Scheduled Tasks

| Command | Description |
|---------|-------------|
| `spawnbot cron list` | List all scheduled tasks |
| `spawnbot cron add` | Add a new scheduled task |
| `spawnbot cron remove <id>` | Remove a scheduled task |
| `spawnbot cron enable <id>` | Enable a task |
| `spawnbot cron disable <id>` | Disable a task |

### Heartbeat

| Command | Description |
|---------|-------------|
| `spawnbot heartbeat status` | Show heartbeat config |
| `spawnbot heartbeat set-interval -m <min>` | Set interval (minimum 5 min) |

### Skills

| Command | Description |
|---------|-------------|
| `spawnbot skills install <name>` | Install from registry |

### Reset and Uninstall

| Command | Description |
|---------|-------------|
| `spawnbot reset` | Delete configs, memories, sessions. Keeps binary. |
| `spawnbot reset -y` | Skip confirmation |
| `spawnbot nuke` | Full uninstall (removes `~/.spawnbot/`, services, PATH) |
| `spawnbot nuke -y` | Skip confirmation |

---

## Features

### LLM Providers (25+)

Protocol-based routing with automatic fallback chains:

| Protocol | Providers |
|----------|-----------|
| OpenAI-compatible | OpenAI, OpenRouter, Groq, Ollama, DeepSeek, Mistral, Moonshot, Cerebras, NVIDIA, Qwen, VolcEngine, LiteLLM, vLLM |
| Native | Anthropic (Claude), Gemini |
| Cloud | Azure OpenAI, AWS Bedrock |
| Other | GitHub Copilot, Antigravity (Google Cloud OAuth) |

Model routing sends simple queries to a cheaper model and complex queries to the primary model.

### Channels (16)

All channels share a unified message bus and can be independently enabled:

Telegram, Discord, Slack, WhatsApp, Matrix (E2EE), WeChat, Wecom, DingTalk, Feishu, QQ, Line, IRC, OneBot, Pico (Web), MaixCam, WebSocket

### Identity

Markdown files in the workspace define the agent's personality and behavior:

| File | Purpose |
|------|---------|
| `SOUL.md` | Core identity and personality (guarded against self-modification) |
| `AGENTS.md` | Behavioral instructions and operating procedures |
| `USER.md` | User profile and preferences |
| `GOALS.md` | Current objectives |
| `HEARTBEAT.md` | Periodic autonomous tasks |

### Memory

1. **Session history** -- JSONL per-conversation logs. Two-tier compaction: proactive summarization at 20 messages or 75% context, emergency compression as fallback.
2. **Daily notes** (`memory/YYYYMM/YYYYMMDD.md`) + long-term (`memory/MEMORY.md`) -- Key facts flushed from conversation before compaction. LLM-based deduplication.
3. **Semantic memory** (SQLite + vector embeddings) -- FTS5 full-text search plus vector similarity with temporal decay. Configurable embedding provider (Gemini, OpenAI).

### Tasks

Persistent task tracking across sessions:
- JSON-backed store with atomic writes
- Actions: create, list, get, update, complete, fail
- Active tasks shown in agent context
- Pending tasks injected into heartbeat for autonomous follow-up
- 7-day TTL auto-cleanup

### Skills

Extensible capabilities with 3-tier priority (workspace > global > builtin):

- `SKILL.md` with YAML frontmatter per skill
- Argument substitution (`${ARGUMENTS}`, `${ARG1}`, etc.)
- Execution contexts: inline (default), fork (sync subagent), spawn (async subagent)
- Slash commands: `/skill name args`
- ClawHub registry for remote search and install
- Built-in skill creator with scaffolding tools

**Default skills:**

| Skill | Purpose |
|-------|---------|
| `weather` | Weather lookups via wttr.in and Open-Meteo |
| `agent-browser` | Browser automation via Chrome CDP (requires `npm i -g agent-browser`) |
| `poller` | Create background polling jobs (RSS, email, web changes) via cron |
| `wallet` | Coinbase agentic wallet (send USDC, trade, x402 marketplace) |
| `summarize` | Conversation summarization |
| `skill-creator` | Scaffold new skills |
| `github` | GitHub operations |
| `tmux` | Terminal multiplexing |

### Agent Definitions

Specialized subagents as markdown files with YAML frontmatter. Each has its own system prompt, tool restrictions, model override, and limits.

| Agent | Purpose |
|-------|---------|
| `researcher` | Gather information (read-only) |
| `coder` | Write code (file + exec, no messaging) |
| `reviewer` | Review work (read-only) |
| `planner` | Break down tasks (read + write, no exec) |
| `self-improver` | Analyze struggles and create skills/agents |

Custom agents: `~/.spawnbot/workspace/agents/<name>/AGENT.md`

### Tools

Built-in tools with configurable approval modes:

| Category | Tools |
|----------|-------|
| Files | `read_file`, `write_file`, `edit_file`, `append_file`, `list_dir` |
| Execution | `exec` (shell commands) |
| Communication | `message`, `send_file` |
| Agents | `spawn` (async), `subagent` (sync), `create_agent` |
| Memory | `memory_store`, `memory_search` |
| Skills | `use_skill`, `search_tools`, `skills_install`, `skills_search` |
| Tasks | `tasks` (create, list, get, update, complete, fail) |
| Scheduling | `cron` (one-time, recurring, cron expressions) |
| Wallet | `wallet` (status, login, verify, balance, send, trade, x402) |
| MCP | `connect_mcp`, `disconnect_mcp`, `list_mcp` |
| Hardware | `i2c`, `spi` (Linux embedded) |

### MCP (Model Context Protocol)

Connect MCP servers at runtime -- no restart required. Supports stdio, SSE, and HTTP transports. Hot-reload via `connect_mcp` / `disconnect_mcp`.

### Autonomy

- **Heartbeat**: Periodic prompt execution (configurable interval, min 5 min). HEARTBEAT_OK suppression, deduplication, structured events, retry.
- **Self-improvement**: Daily review of struggles. Collector logs tool failures, corrections, repeated patterns. Self-improver agent creates skills/agents to fix recurring issues.
- **Idle triggers**: Fire after channel inactivity threshold.
- **RSS polling**: Monitor feeds, notify on new items.
- **Cron**: Standard cron expressions, shell command execution, direct message delivery.

### Security

- AES-256-GCM credential encryption with SSH key derivation
- Workspace restriction with configurable read/write path whitelists
- Approval modes: YOLO, Approval, Review
- Sensitive data filtering in tool output

### Voice

Audio transcription via Groq Whisper, ElevenLabs Scribe, or audio-capable LLM models.

### Web UI

React SPA at http://localhost:18800 with streaming chat, config editor, credential management, skill browser, onboarding wizard, and real-time log viewer.

---

## Configuration

```
~/.spawnbot/
  config.json              Main configuration
  .security.yml            Encrypted credentials
  workspace/
    SOUL.md                Agent identity
    AGENTS.md              Behavioral instructions
    USER.md                User profile
    GOALS.md               Objectives
    HEARTBEAT.md           Periodic tasks
    tasks.json             Task store
    struggles.jsonl        Struggle signals
    memory/                Daily notes + long-term memory
    sessions/              Conversation history (JSONL)
    skills/                Installed skills
    agents/                Custom agent definitions
    state/                 Persistent state
```

### Key Config Sections

```jsonc
{
  "agents": {
    "defaults": {
      "provider": "gemini-3-flash-preview",  // Default LLM provider
      "max_tokens": 32768,
      "max_tool_iterations": 30,
      "approval_mode": "yolo"                // yolo | approval | review
    }
  },
  "heartbeat": {
    "enabled": true,
    "interval": 30                           // Minutes (min 5)
  },
  "self_improve": {
    "enabled": true,
    "hour": 3,                               // Daily review hour (0-23)
    "max_creations": 3,
    "max_retries": 2
  },
  "tools": {
    "wallet": {
      "enabled": true,
      "email": "agent@example.com",          // Wallet auth email
      "chain": "base",                       // base | base-sepolia
      "max_send_amount": 100.0,
      "max_trade_amount": 50.0,
      "max_pay_amount": 1.0
    }
  }
}
```

---

## Architecture

```
cmd/spawnbot/                CLI (Cobra)
cmd/spawnbot-launcher-tui/   TUI launcher
web/backend/                 Go HTTP + WebSocket server
web/frontend/                React SPA (embedded in binary)

pkg/
  agent/       Core loop, turn state, context building, memory flush
  bus/         Message routing (inbound/outbound)
  channels/    16 communication adapters
  providers/   25+ LLM providers + fallback chains
  tools/       Tool registry + built-in tools + MCP wrappers
  agents/      Agent definitions and registry
  tasks/       Persistent task store
  skills/      Skill loading and discovery
  memory/      JSONL session store + SQLite semantic memory
  config/      Configuration, validation, migration
  routing/     Model routing (light/heavy)
  mcp/         MCP server lifecycle
  autonomy/    Idle monitor, RSS poller
  heartbeat/   Periodic tasks + self-improvement trigger
  struggles/   Struggle signal collection
  cron/        Job scheduling
  voice/       Audio transcription
  identity/    SOUL.md loading
  credential/  SSH-key encryption
  workspace/   Workspace deployment
```

## Building from Source

```bash
git clone https://github.com/dawnforge-lab/spawnbot-v5.git
cd spawnbot-v5

make build                # Build to ./build/
make install              # Build + install to ~/.spawnbot/bin/
CGO_ENABLED=1 make build  # With semantic memory (requires SQLite headers)
make build-all            # Cross-compile all platforms
make docker-build         # Minimal Alpine container
```

**Platforms**: linux/amd64, linux/arm64, linux/arm, linux/riscv64, linux/loong64, linux/mipsle, darwin/arm64, windows/amd64, netbsd/amd64, netbsd/arm64

## License

MIT (based on [PicoClaw](https://github.com/sipeed/picoclaw) by Sipeed)
