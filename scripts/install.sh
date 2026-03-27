#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/dawnforge-lab/spawnbot-v5.git"
SPAWNBOT_HOME="${SPAWNBOT_HOME:-$HOME/.spawnbot}"
BIN_DIR="$SPAWNBOT_HOME/bin"
TMP_DIR="$(mktemp -d)"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

echo "Installing Spawnbot to $SPAWNBOT_HOME ..."

# Check for Go
if ! command -v go &>/dev/null; then
    echo "Error: Go is required but not found. Install it from https://go.dev/dl/"
    exit 1
fi

# Clone and build
echo "Cloning and building..."
git clone --depth 1 "$REPO" "$TMP_DIR/src" 2>&1 | tail -1
cd "$TMP_DIR/src"
CGO_ENABLED=0 go generate ./pkg/workspace/
CGO_ENABLED=0 go build -v -tags goolm,stdjson -ldflags "-s -w" -o "$TMP_DIR/spawnbot" ./cmd/spawnbot/

# Install binary
mkdir -p "$BIN_DIR"
mv "$TMP_DIR/spawnbot" "$BIN_DIR/spawnbot"
chmod +x "$BIN_DIR/spawnbot"

echo ""
echo "Spawnbot installed to $BIN_DIR/spawnbot"

# Add to PATH if not already there
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    SHELL_RC=""
    if [[ -f "$HOME/.zshrc" ]]; then
        SHELL_RC="$HOME/.zshrc"
    elif [[ -f "$HOME/.bashrc" ]]; then
        SHELL_RC="$HOME/.bashrc"
    fi

    if [[ -n "$SHELL_RC" ]]; then
        if ! grep -q 'spawnbot/bin' "$SHELL_RC" 2>/dev/null; then
            echo 'export PATH="$HOME/.spawnbot/bin:$PATH"' >> "$SHELL_RC"
            echo "Added to PATH in $SHELL_RC"
        fi
    else
        echo "Add this to your shell profile:"
        echo '  export PATH="$HOME/.spawnbot/bin:$PATH"'
    fi
fi

echo ""
echo "Next: run 'spawnbot onboard' to set up your agent."
echo "      (you may need to restart your shell or run: source $SHELL_RC)"
