#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/dawnforge-lab/spawnbot-v5.git"
SPAWNBOT_HOME="${SPAWNBOT_HOME:-$HOME/.spawnbot}"
BIN_DIR="$SPAWNBOT_HOME/bin"
GO_VERSION="1.25.8"
TMP_DIR="$(mktemp -d)"

# Go stores modules as read-only; must fix perms before rm.
cleanup() {
    if [[ -d "$TMP_DIR" ]]; then
        find "$TMP_DIR" -type d -exec chmod u+wx {} + 2>/dev/null || true
        rm -rf "$TMP_DIR"
    fi
}
trap cleanup EXIT

# Keep all Go caches inside TMP_DIR so cleanup is self-contained.
export GOPATH="$TMP_DIR/gopath"
export GOMODCACHE="$TMP_DIR/gomod"
export GOCACHE="$TMP_DIR/gobuild"

echo "Installing Spawnbot to $SPAWNBOT_HOME ..."

# Check for git
if ! command -v git &>/dev/null; then
    echo "Error: git is required. Install it first."
    exit 1
fi

# Install Go locally if not found
GO_CMD="go"
if ! command -v go &>/dev/null; then
    echo "Go not found. Installing Go $GO_VERSION locally..."

    ARCH="$(uname -m)"
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$ARCH" in
        x86_64)  GOARCH="amd64" ;;
        aarch64|arm64) GOARCH="arm64" ;;
        armv7l|armv6l) GOARCH="armv6l" ;;
        *) echo "Error: unsupported architecture $ARCH"; exit 1 ;;
    esac

    GO_TAR="go${GO_VERSION}.${OS}-${GOARCH}.tar.gz"
    GO_URL="https://go.dev/dl/${GO_TAR}"
    GO_LOCAL="$SPAWNBOT_HOME/go"

    mkdir -p "$SPAWNBOT_HOME"
    echo "Downloading $GO_URL ..."
    curl -fsSL "$GO_URL" -o "$TMP_DIR/$GO_TAR"
    rm -rf "$GO_LOCAL"
    tar -C "$SPAWNBOT_HOME" -xzf "$TMP_DIR/$GO_TAR"

    export PATH="$GO_LOCAL/bin:$PATH"
    GO_CMD="$GO_LOCAL/bin/go"
    echo "Go $GO_VERSION installed to $GO_LOCAL"
fi

# Clone and build
echo "Cloning and building..."
git clone --depth 1 "$REPO" "$TMP_DIR/src" 2>&1 | tail -1
cd "$TMP_DIR/src"
CGO_ENABLED=0 "$GO_CMD" generate ./pkg/workspace/
CGO_ENABLED=0 "$GO_CMD" build -tags goolm,stdjson -ldflags "-s -w" -o "$TMP_DIR/spawnbot" ./cmd/spawnbot/

# Install binary
mkdir -p "$BIN_DIR"
mv "$TMP_DIR/spawnbot" "$BIN_DIR/spawnbot"
chmod +x "$BIN_DIR/spawnbot"

echo ""
echo "Spawnbot installed to $BIN_DIR/spawnbot"

# Add to PATH — detect the user's actual shell, not just which rc files exist
SHELL_RC=""
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    case "${SHELL:-}" in
        */zsh)  SHELL_RC="$HOME/.zshrc" ;;
        */bash) SHELL_RC="$HOME/.bashrc" ;;
        *)
            # Fallback: check which rc file exists
            if [[ -f "$HOME/.bashrc" ]]; then
                SHELL_RC="$HOME/.bashrc"
            elif [[ -f "$HOME/.zshrc" ]]; then
                SHELL_RC="$HOME/.zshrc"
            fi
            ;;
    esac

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
if [[ -n "${SHELL_RC:-}" ]]; then
    echo "      (restart your shell or run: source $SHELL_RC)"
fi
