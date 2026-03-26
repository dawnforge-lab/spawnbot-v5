# Spawnbot

Your personal AI agent.

## Features

- 25+ LLM providers (OpenRouter, Anthropic, OpenAI, Gemini, Ollama, LiteLLM, etc.)
- Semantic memory (SQLite FTS5 + vector search with temporal decay)
- Identity system (SOUL.md defines who the agent is)
- Autonomy (cron scheduling, idle triggers, RSS feed monitoring)
- 17 messaging channels (Telegram, Discord, Slack, WhatsApp, Matrix, etc.)
- MCP client (connect any MCP server)
- Sub-agents for parallel task execution
- Web UI + CLI + TUI
- YOLO/Approval tool modes
- Single binary, <15MB, sub-second boot

## Quickstart

```bash
# Install
curl -fsSL https://github.com/dawnforge-lab/spawnbot-v5/releases/latest/download/install.sh | bash

# Or build from source
git clone https://github.com/dawnforge-lab/spawnbot-v5.git
cd spawnbot-v5
CGO_ENABLED=1 make build

# Setup
./spawnbot onboard

# Run
./spawnbot agent        # CLI chat
./spawnbot gateway      # Start all channels
```

## Configuration

- Config: `~/.spawnbot/config.json`
- Secrets: `~/.spawnbot/.security.yml`
- Workspace: `~/.spawnbot/workspace/`
- Identity: `~/.spawnbot/workspace/SOUL.md`
- Memory: `~/.spawnbot/workspace/memory/`

## Building

```bash
CGO_ENABLED=1 go build -tags "goolm,stdjson,fts5" -o spawnbot ./cmd/spawnbot/
```

## License

Apache-2.0 (based on PicoClaw by Sipeed)
