# Spawnbot → Spawnbot Infrastructure Rebrand

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the spawnbot → spawnbot rebrand across all infrastructure files (Docker, scripts, configs, docs, templates). Go source and web frontend are already done.

**Architecture:** Pure find-and-replace across ~170 files. No logic changes. The Go module is already `github.com/dawnforge-lab/spawnbot-v5`, binaries are `spawnbot`/`spawnbot-launcher`/`spawnbot-launcher-tui`, config paths are `~/.spawnbot/`. Only infrastructure files lag behind.

**Tech Stack:** Dockerfiles, shell scripts, Makefiles, markdown docs, JSON configs, Inno Setup, .desktop files

---

### Task 1: Fix broken web/Makefile import path (BUILD BLOCKER)

**Files:**
- Modify: `web/Makefile:16`

- [ ] **Step 1: Fix CONFIG_PKG**

```makefile
# OLD (broken — module doesn't exist):
CONFIG_PKG=github.com/dawnforge-lab/spawnbot-v5/pkg/config

# NEW:
CONFIG_PKG=github.com/dawnforge-lab/spawnbot-v5/pkg/config
```

- [ ] **Step 2: Fix binary output name**

In `web/Makefile:63` and `:81`:
```makefile
# OLD:
	@if [ ! -f $(BUILD_DIR)/spawnbot-launcher ] || [ ! -d backend/dist ]; then \
# ...
	${WEB_GO} build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/spawnbot-launcher ./backend/

# NEW:
	@if [ ! -f $(BUILD_DIR)/spawnbot-launcher ] || [ ! -d backend/dist ]; then \
# ...
	${WEB_GO} build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/spawnbot-launcher ./backend/
```

- [ ] **Step 3: Commit**

```bash
git add web/Makefile
git commit -m "fix: web/Makefile broken CONFIG_PKG import path and binary name"
```

---

### Task 2: Rebrand all Docker files

**Files:**
- Modify: `docker/Dockerfile`
- Modify: `docker/Dockerfile.full`
- Modify: `docker/Dockerfile.heavy`
- Modify: `docker/Dockerfile.goreleaser`
- Modify: `docker/Dockerfile.goreleaser.launcher`
- Modify: `docker/docker-compose.yml`
- Modify: `docker/docker-compose.full.yml`
- Modify: `docker/entrypoint.sh`

**Replacement rules (apply to ALL files above):**

| Old | New |
|-----|-----|
| `spawnbot` (binary name, user, group, paths) | `spawnbot` |
| `Spawnbot` (comments) | `Spawnbot` |
| `.spawnbot` (config dir) | `.spawnbot` |
| `dawnforge-lab/spawnbot-v5` (Docker Hub image) | `dawnforge-lab/spawnbot` |
| `SPAWNBOT_GATEWAY_HOST` (env var) | `SPAWNBOT_GATEWAY_HOST` |
| `spawnbot-workspace` (volume name) | `spawnbot-workspace` |
| `spawnbot-npm-cache` (volume name) | `spawnbot-npm-cache` |
| `spawnbot-agent` (service/container name) | `spawnbot-agent` |
| `spawnbot-gateway` (service/container name) | `spawnbot-gateway` |
| `spawnbot-launcher` (service/container name) | `spawnbot-launcher` |

- [ ] **Step 1: Rebrand docker/Dockerfile**

Replace all `spawnbot` → `spawnbot` and `Spawnbot` → `Spawnbot`:
- Comment: "Build the spawnbot binary"
- COPY line: `/src/build/spawnbot /usr/local/bin/spawnbot`
- User/group: `spawnbot`
- Onboard: `/usr/local/bin/spawnbot onboard`
- ENTRYPOINT: `["spawnbot"]`

- [ ] **Step 2: Rebrand docker/Dockerfile.full**

Same pattern:
- Comment: "Build the spawnbot binary"
- COPY line: `/src/build/spawnbot /usr/local/bin/spawnbot`
- Comment: "Create spawnbot home directory"
- Onboard: `/usr/local/bin/spawnbot onboard`
- ENTRYPOINT: `["spawnbot"]`

- [ ] **Step 3: Rebrand docker/Dockerfile.heavy**

Same pattern plus:
- User rename: `spawnbot` → `spawnbot` (addgroup, adduser)
- USER directive: `spawnbot`
- Workspace COPY: `/home/spawnbot/.spawnbot/workspace/`
- VOLUME: `/home/spawnbot/.spawnbot/workspace`

- [ ] **Step 4: Rebrand docker/Dockerfile.goreleaser**

- COPY: `$TARGETPLATFORM/spawnbot /usr/local/bin/spawnbot`

- [ ] **Step 5: Rebrand docker/Dockerfile.goreleaser.launcher**

- COPY lines: `spawnbot`, `spawnbot-launcher`, `spawnbot-launcher-tui`
- ENTRYPOINT: `["spawnbot-launcher"]`

- [ ] **Step 6: Rebrand docker/docker-compose.yml**

Full replacement of all service names, image refs, container names, volume paths, env vars, comments. Replace `dawnforge-lab/spawnbot-v5` with `dawnforge-lab/spawnbot`.

- [ ] **Step 7: Rebrand docker/docker-compose.full.yml**

Same pattern. Replace all service names, container names, volume names, paths, comments.

- [ ] **Step 8: Rebrand docker/entrypoint.sh**

Replace `.spawnbot` → `.spawnbot` and `spawnbot` → `spawnbot` in commands and paths.

- [ ] **Step 9: Commit**

```bash
git add docker/
git commit -m "chore: rebrand all Docker files from spawnbot to spawnbot"
```

---

### Task 3: Rebrand scripts

**Files:**
- Modify: `scripts/build-macos-app.sh`
- Modify: `scripts/setup.iss`
- Modify: `scripts/test-irc.sh`
- Modify: `scripts/test-docker-mcp.sh`

- [ ] **Step 1: Rebrand scripts/build-macos-app.sh**

| Old | New |
|-----|-----|
| `Spawnbot Launcher` | `Spawnbot Launcher` |
| `spawnbot-launcher` | `spawnbot-launcher` |
| `spawnbot` (binary ref) | `spawnbot` |
| `com.spawnbot.launcher` | `com.spawnbot.launcher` |
| `Spawnbot.app` | `Spawnbot.app` |
| `Spawnbot` in messages | `Spawnbot` |

- [ ] **Step 2: Rebrand scripts/setup.iss**

| Old | New |
|-----|-----|
| `Spawnbot Launcher` | `Spawnbot Launcher` |
| `Spawnbot` (publisher) | `Spawnbot` |
| `https://github.com/dawnforge-lab/spawnbot-v5` | `https://github.com/dawnforge-lab/spawnbot-v5` |
| `spawnbot-launcher.exe` | `spawnbot-launcher.exe` |
| `DefaultDirName={autopf}\Spawnbot` | `DefaultDirName={autopf}\Spawnbot` |
| `SpawnbotSetup` | `SpawnbotSetup` |
| `spawnbot.exe` binary refs | `spawnbot.exe` |

- [ ] **Step 3: Rebrand scripts/test-irc.sh**

| Old | New |
|-----|-----|
| `spawnbot-test-ergo` | `spawnbot-test-ergo` |
| `~/.spawnbot/config.json` | `~/.spawnbot/config.json` |
| `spawnbot` (run commands) | `spawnbot` |
| `packages/spawnbot` | remove or update path |

- [ ] **Step 4: Update scripts/test-docker-mcp.sh if needed**

Check for spawnbot references and replace.

- [ ] **Step 5: Commit**

```bash
git add scripts/
git commit -m "chore: rebrand all scripts from spawnbot to spawnbot"
```

---

### Task 4: Rebrand config, env, and ignore files

**Files:**
- Modify: `.env.example`
- Modify: `.gitignore`
- Modify: `.dockerignore`
- Modify: `config/config.example.json`

- [ ] **Step 1: Rebrand .env.example**

```
# OLD:
SPAWNBOT_CHANNELS_FEISHU_APP_ID=cli_xxx
SPAWNBOT_CHANNELS_FEISHU_APP_SECRET=xxx
SPAWNBOT_CHANNELS_FEISHU_RANDOM_REACTION_EMOJI=Typing,OneSecond

# NEW:
SPAWNBOT_CHANNELS_FEISHU_APP_ID=cli_xxx
SPAWNBOT_CHANNELS_FEISHU_APP_SECRET=xxx
SPAWNBOT_CHANNELS_FEISHU_RANDOM_REACTION_EMOJI=Typing,OneSecond
```

- [ ] **Step 2: Rebrand .gitignore**

```
# OLD:
/spawnbot
/spawnbot-test
# Spawnbot
.spawnbot/

# NEW:
/spawnbot
/spawnbot-test
# Spawnbot
.spawnbot/
```

Note: `.spawnbot/` → `.spawnbot/` (though Go code already uses `.spawnbot`, keep ignore for backward compat — actually just use `.spawnbot/`)

- [ ] **Step 3: Rebrand .dockerignore**

```
# OLD:
.spawnbot/

# NEW:
.spawnbot/
```

- [ ] **Step 4: Rebrand config/config.example.json**

Replace `~/.spawnbot/workspace` → `~/.spawnbot/workspace`

- [ ] **Step 5: Commit**

```bash
git add .env.example .gitignore .dockerignore config/config.example.json
git commit -m "chore: rebrand config/env/ignore files from spawnbot to spawnbot"
```

---

### Task 5: Rename and rebrand desktop/manifest files

**Files:**
- Rename: `web/spawnbot-launcher.desktop` → `web/spawnbot-launcher.desktop`
- Rename: `web/spawnbot-launcher.png` → `web/spawnbot-launcher.png`
- Modify: `web/frontend/public/site.webmanifest`

- [ ] **Step 1: Rename and rebrand desktop file**

```bash
git mv web/spawnbot-launcher.desktop web/spawnbot-launcher.desktop
git mv web/spawnbot-launcher.png web/spawnbot-launcher.png
```

Update `web/spawnbot-launcher.desktop` content:
```ini
[Desktop Entry]
Name=Spawnbot Launcher
Comment=Web-based configuration and management UI for Spawnbot
Exec=spawnbot-launcher
Icon=spawnbot-launcher
Terminal=true
Type=Application
Categories=Utility;Network;
Keywords=spawnbot;ai;agent;bot;
```

- [ ] **Step 2: Update site.webmanifest**

```json
{
  "name": "Spawnbot",
  "short_name": "Spawnbot",
  ...
}
```

- [ ] **Step 3: Commit**

```bash
git add web/
git commit -m "chore: rename desktop/manifest files to spawnbot"
```

---

### Task 6: Rebrand root Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Rename SPAWNBOT_HOME variable**

```makefile
# OLD:
SPAWNBOT_HOME?=$(HOME)/.spawnbot
WORKSPACE_DIR?=$(SPAWNBOT_HOME)/workspace
# ...
@rm -rf $(SPAWNBOT_HOME)
@echo "Removed workspace: $(SPAWNBOT_HOME)"

# NEW:
SPAWNBOT_HOME?=$(HOME)/.spawnbot
WORKSPACE_DIR?=$(SPAWNBOT_HOME)/workspace
# ...
@rm -rf $(SPAWNBOT_HOME)
@echo "Removed workspace: $(SPAWNBOT_HOME)"
```

- [ ] **Step 2: Fix macOS app target text**

```makefile
# OLD:
## build-macos-app: Build Spawnbot macOS .app bundle (no terminal window)
# ...
@echo "macOS .app bundle created: $(BUILD_DIR)/Spawnbot.app"

# NEW:
## build-macos-app: Build Spawnbot macOS .app bundle (no terminal window)
# ...
@echo "macOS .app bundle created: $(BUILD_DIR)/Spawnbot.app"
```

- [ ] **Step 3: Fix docker make targets to use new compose service names**

In the Makefile, the `docker-build` and `docker-run` targets reference `spawnbot-agent` and `spawnbot-gateway` — these will match after Task 2 renames compose services. Verify they align.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "chore: rebrand Makefile variables from PICOCLAW to SPAWNBOT"
```

---

### Task 7: Replace CLI banner ASCII art

**Files:**
- Modify: `cmd/spawnbot/main.go:54-65`

- [ ] **Step 1: Replace PICO CLAW banner with SPAWNBOT**

Replace the existing `banner` const with a SPAWNBOT ASCII art banner. Use a single color scheme (the existing blue+red dual-color is fine, or simplify to one color).

Generate new banner text for "SPAWNBOT" using the same block letter style.

- [ ] **Step 2: Verify binary runs**

```bash
go run ./cmd/spawnbot version
```

- [ ] **Step 3: Commit**

```bash
git add cmd/spawnbot/main.go
git commit -m "chore: replace PICO CLAW CLI banner with SPAWNBOT"
```

---

### Task 8: Rebrand workspace AGENT.md

**Files:**
- Modify: `workspace/AGENT.md`

- [ ] **Step 1: Replace Spawnbot identity**

```markdown
---
name: spawnbot
description: >
  The default general-purpose assistant for everyday conversation, problem
  solving, and workspace help.
---

You are Spawnbot, the default assistant for this workspace.

## Role

You are a lightweight, self-evolving personal AI assistant written in Go, designed to
be practical, accurate, and efficient.
```

Remove the `🦞` emoji and `Spawnbot` name references. Replace with `Spawnbot`.

- [ ] **Step 2: Commit**

```bash
git add workspace/AGENT.md
git commit -m "chore: rebrand workspace AGENT.md to Spawnbot identity"
```

---

### Task 9: Rebrand LICENSE, CONTRIBUTING, and GitHub templates

**Files:**
- Modify: `LICENSE`
- Modify: `CONTRIBUTING.md`
- Modify: `CONTRIBUTING.zh.md`
- Modify: `.github/ISSUE_TEMPLATE/bug_report.md`

- [ ] **Step 1: Update LICENSE**

```
Copyright (c) 2026 Spawnbot contributors
```

- [ ] **Step 2: Update CONTRIBUTING.md and CONTRIBUTING.zh.md**

Replace `dawnforge-lab/spawnbot-v5` → `dawnforge-lab/spawnbot-v5`, `Spawnbot` → `Spawnbot`, `spawnbot` → `spawnbot` throughout.

- [ ] **Step 3: Update bug_report.md**

```markdown
- **Spawnbot Version:** → **Spawnbot Version:**
```

- [ ] **Step 4: Commit**

```bash
git add LICENSE CONTRIBUTING.md CONTRIBUTING.zh.md .github/
git commit -m "chore: rebrand LICENSE, CONTRIBUTING, and issue templates"
```

---

### Task 10: Fix sipeed references in Go test files

**Files:**
- Modify: `cmd/spawnbot/internal/skills/install.go:19` (help text example)
- Modify: `cmd/spawnbot/internal/skills/install_test.go`
- Modify: `pkg/skills/installer_test.go`

- [ ] **Step 1: Update install.go help text**

```go
// OLD:
spawnbot skills install dawnforge-lab/spawnbot-v5-skills/weather

// NEW:
spawnbot skills install dawnforge-lab/spawnbot-skills/weather
```

- [ ] **Step 2: Update install_test.go**

Replace `dawnforge-lab/spawnbot-v5-skills/weather` → `dawnforge-lab/spawnbot-skills/weather`

- [ ] **Step 3: Update installer_test.go**

Replace all `sipeed` owner references with `dawnforge-lab` in test fixtures, and `dawnforge-lab/spawnbot-v5` → `dawnforge-lab/spawnbot-v5`.

- [ ] **Step 4: Run tests**

```bash
CGO_ENABLED=0 go test -tags goolm,stdjson ./pkg/skills/... ./cmd/spawnbot/internal/skills/...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/spawnbot/internal/skills/ pkg/skills/
git commit -m "chore: replace sipeed references in skills install code and tests"
```

---

### Task 11: Update pico-echo-server example

**Files:**
- Modify: `examples/pico-echo-server/README.md`

- [ ] **Step 1: Replace spawnbot references**

Line 47: `Start spawnbot` → `Start spawnbot`

- [ ] **Step 2: Commit**

```bash
git add examples/
git commit -m "chore: rebrand pico-echo-server README"
```

---

### Task 12: Bulk rebrand documentation files (~100+ files)

**Files:**
- All files in `docs/` containing spawnbot/Spawnbot references
- `README.md` (root)
- `web/backend/.gitignore`

**Replacement rules applied across ALL doc files:**

| Pattern | Replacement |
|---------|-------------|
| `Spawnbot` | `Spawnbot` |
| `spawnbot` (binary/command) | `spawnbot` |
| `~/.spawnbot` | `~/.spawnbot` |
| `SPAWNBOT_HOME` | `SPAWNBOT_HOME` |
| `SPAWNBOT_CONFIG` | `SPAWNBOT_CONFIG` |
| `SPAWNBOT_CHANNELS_*` | `SPAWNBOT_CHANNELS_*` |
| `dawnforge-lab/spawnbot-v5` | `dawnforge-lab/spawnbot-v5` |
| `sipeed` (as org in URLs/refs) | `dawnforge-lab` where it refers to spawnbot repo |

- [ ] **Step 1: Use sed to bulk replace across all docs**

```bash
# Run from project root
find docs/ -name '*.md' -exec sed -i \
  -e 's|Spawnbot|Spawnbot|g' \
  -e 's|spawnbot|spawnbot|g' \
  -e 's|SPAWNBOT_HOME|SPAWNBOT_HOME|g' \
  -e 's|SPAWNBOT_CONFIG|SPAWNBOT_CONFIG|g' \
  -e 's|SPAWNBOT_CHANNELS|SPAWNBOT_CHANNELS|g' \
  -e 's|dawnforge-lab/spawnbot-v5|dawnforge-lab/spawnbot-v5|g' \
  {} +
```

Note: The `dawnforge-lab/spawnbot-v5` pattern handles the case where `dawnforge-lab/spawnbot-v5` was already partially renamed to `dawnforge-lab/spawnbot-v5` by the first rule.

- [ ] **Step 2: Fix README.md**

Update root README.md — replace remaining spawnbot/Spawnbot/sipeed references.

- [ ] **Step 3: Fix web/backend/.gitignore**

Replace `spawnbot-web` → `spawnbot-web`

- [ ] **Step 4: Spot-check a few files to verify correctness**

Read 3-4 files across different languages to verify replacements are clean and no broken URLs or garbled text.

- [ ] **Step 5: Commit**

```bash
git add docs/ README.md web/backend/.gitignore
git commit -m "chore: bulk rebrand all documentation from spawnbot to spawnbot"
```

---

### Task 13: Verify build and tests

**Files:** None (verification only)

- [ ] **Step 1: Run go vet**

```bash
make vet
```

- [ ] **Step 2: Run tests**

```bash
make test
```

- [ ] **Step 3: Build binary**

```bash
make build
```

- [ ] **Step 4: Verify banner**

```bash
./build/spawnbot version
```

- [ ] **Step 5: Grep for remaining spawnbot references**

```bash
grep -ri 'spawnbot' --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' --include='*.yaml' --include='*.json' --include='*.iss' --include='*.desktop' . | grep -v '.git/' | grep -v 'node_modules/'
```

Expected: Zero results (or only in git history / vendor).

- [ ] **Step 6: Grep for remaining sipeed references**

```bash
grep -ri 'sipeed' --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' --include='*.yaml' --include='*.json' . | grep -v '.git/' | grep -v 'node_modules/'
```

Expected: Zero results.
