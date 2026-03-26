# Spawnbot Launcher TUI

This directory contains the terminal-based TUI launcher for `spawnbot`.
It provides a lightweight, terminal-native user interface for managing, configuring, and interacting with the core `spawnbot` engine, without requiring a web browser or graphical environment.

## Architecture

The TUI launcher is implemented purely in Go with no external runtime dependencies:
* **`main.go`**: Application entry point, handles initialization and main event loop
* **`ui/`**: TUI interface components built on tview + tcell framework:
  - `home.go`: Main dashboard with navigation menu
  - `schemes.go`: AI model scheme management
  - `users.go`: User and API key management for model providers
  - `channels.go`: Communication channel (Telegram/Discord/WeChat etc.) configuration editor
  - `gateway.go`: Spawnbot gateway daemon lifecycle management (start/stop/status)
  - `app.go`: Core TUI application framework and navigation logic
  - `models.go`: Data structures and state management
* **`config/`**: Configuration management layer, integrates with the core spawnbot configuration system

## Getting Started

### Prerequisites

* Go 1.25+
* Terminal with 256-color support (most modern terminals are compatible)

### Development

Run the TUI launcher directly in development mode:

```bash
# From project root
go run ./cmd/spawnbot-launcher-tui

# Or from this directory
go run .
```

### Build

Build the standalone TUI launcher binary:

```bash
# From project root (recommended)
make build-launcher-tui

# Output will be at:
# build/spawnbot-launcher-tui-<platform>-<arch>
# with symlink build/spawnbot-launcher-tui

# Or build directly from this directory
go build -o spawnbot-launcher-tui .
```

### Key Features

* 🖥️ Terminal-native interface - works over SSH, on headless servers, and in low-resource environments
* ⚙️ AI model scheme and API key management
* 📱 Communication channel configuration editor (Telegram/Discord/WeChat etc.)
* 🔄 Spawnbot gateway daemon management (start/stop/status monitoring)
* 💬 One-click launch of interactive AI chat session
* 🎯 Keyboard-first design with intuitive shortcuts

### Other Commands

```bash
# Run with custom config file path
go run . /path/to/custom/config.json
```
