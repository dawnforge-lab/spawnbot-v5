# Spawnbot v5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform spawnbot fork into Spawnbot v5 — rebranded, with semantic memory, autonomy, approval modes, onboarding, and enhanced web UI.

**Architecture:** Fork spawnbot (Go agent), full rename to spawnbot, add SQLite-backed semantic memory with FTS5 + sqlite-vec, idle/feed autonomy, YOLO/Approval tool modes, CLI + web onboarding, and enhanced chat frontend.

**Tech Stack:** Go 1.25+, mattn/go-sqlite3 (CGO), asg017/sqlite-vec, charmbracelet/huh, mmcdole/gofeed, TypeScript/Vite (frontend)

**Spec:** `docs/superpowers/specs/2026-03-26-spawnbot-v5-design.md`

---

## Phase 1: Full Rebrand (foundation — everything else depends on this)

### Task 1: Commit spawnbot source as base

**Files:**
- All spawnbot source files (currently untracked)

- [ ] **Step 1: Add the full spawnbot source**

```bash
git add -A
git commit -m "chore: add spawnbot v0.2.3 source as spawnbot-v5 base"
```

### Task 2: Rename Go module and all imports

**Files:**
- Modify: `go.mod:1` (module path)
- Modify: All 268 `.go` files with `dawnforge-lab/spawnbot-v5` imports

- [ ] **Step 1: Replace module path in go.mod**

Change line 1 of `go.mod`:
```
module github.com/dawnforge-lab/spawnbot-v5
```
to:
```
module github.com/dawnforge-lab/spawnbot-v5
```

- [ ] **Step 2: Replace all Go import paths**

```bash
find . -name '*.go' -exec sed -i 's|github.com/dawnforge-lab/spawnbot-v5|github.com/dawnforge-lab/spawnbot-v5|g' {} +
```

- [ ] **Step 3: Run go mod tidy to verify**

```bash
go mod tidy
```
Expected: no errors, all imports resolve.

- [ ] **Step 4: Verify build compiles**

```bash
go build ./...
```
Expected: PASS, no import errors.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: rename Go module dawnforge-lab/spawnbot-v5 → dawnforge-lab/spawnbot-v5"
```

### Task 3: Rename branding constants and env vars

**Files:**
- Modify: `pkg/env.go` (AppName, DefaultSpawnbotHome, Logo)
- Modify: `pkg/config/envkeys.go` (all `SPAWNBOT_*` → `SPAWNBOT_*`)
- Modify: `pkg/config/config.go` (any hardcoded "spawnbot" strings)
- Modify: `pkg/config/defaults.go` (default paths)
- Modify: `pkg/agent/context.go:95-116` (identity string "spawnbot")

- [ ] **Step 1: Update pkg/env.go**

```go
const (
	Logo    = "🤖"
	AppName = "Spawnbot"

	DefaultSpawnbotHome = ".spawnbot"
	WorkspaceName       = "workspace"
)
```

- [ ] **Step 2: Rename DefaultSpawnbotHome references across codebase**

```bash
grep -rn "DefaultSpawnbotHome" --include="*.go" -l
# Update each file to use DefaultSpawnbotHome
find . -name '*.go' -exec sed -i 's/DefaultSpawnbotHome/DefaultSpawnbotHome/g' {} +
```

- [ ] **Step 3: Update env var prefix in pkg/config/envkeys.go**

Replace all `SPAWNBOT_` with `SPAWNBOT_`:
```bash
sed -i 's/SPAWNBOT_/SPAWNBOT_/g' pkg/config/envkeys.go
```

- [ ] **Step 4: Update env var references across codebase**

```bash
grep -rn "SPAWNBOT_" --include="*.go" -l
# Update each occurrence
find . -name '*.go' -exec sed -i 's/SPAWNBOT_/SPAWNBOT_/g' {} +
```

- [ ] **Step 5: Update identity string in pkg/agent/context.go getIdentity()**

Replace "spawnbot" with "spawnbot" and "🦞" with "🤖" in the `getIdentity()` function (lines 89-116).

- [ ] **Step 6: Grep for any remaining "spawnbot" in Go files**

```bash
grep -rni "spawnbot" --include="*.go" | grep -v "_test.go" | grep -v "vendor/"
```
Fix any remaining references.

- [ ] **Step 7: Verify build and tests**

```bash
go build ./...
go test ./pkg/... -count=1 -short 2>&1 | tail -20
```

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor: rebrand Spawnbot → Spawnbot (constants, env vars, identity)"
```

### Task 4: Rename binary entry points

**Files:**
- Rename: `cmd/spawnbot/` → `cmd/spawnbot/`
- Rename: `cmd/spawnbot-launcher-tui/` → `cmd/spawnbot-launcher-tui/`
- Modify: `Makefile` (binary names, build targets)
- Modify: `web/backend/main.go` (subprocess binary name)

- [ ] **Step 1: Rename cmd directories**

```bash
mv cmd/spawnbot cmd/spawnbot
mv cmd/spawnbot-launcher-tui cmd/spawnbot-launcher-tui
```

- [ ] **Step 2: Update Makefile binary names**

Replace all `spawnbot` binary references with `spawnbot` in Makefile.

- [ ] **Step 3: Update web backend subprocess launch**

In `web/backend/main.go`, update the binary name used when spawning the core subprocess from "spawnbot" to "spawnbot".

- [ ] **Step 4: Update any other binary name references**

```bash
grep -rn '"spawnbot"' --include="*.go" | grep -v "_test.go"
```
Fix remaining hardcoded binary name strings.

- [ ] **Step 5: Verify build**

```bash
go build -o spawnbot ./cmd/spawnbot/
./spawnbot version
```
Expected: shows "Spawnbot" in output.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: rename binary spawnbot → spawnbot"
```

### Task 5: Rebrand frontend

**Files:**
- Modify: `web/frontend/` (~12 files with "spawnbot" references)
- Modify: `web/frontend/index.html` (title)
- Modify: `web/frontend/package.json` (name)

- [ ] **Step 1: Find all frontend spawnbot references**

```bash
grep -rni "spawnbot" web/frontend/ --include="*.ts" --include="*.tsx" --include="*.json" --include="*.html" --include="*.css"
```

- [ ] **Step 2: Replace all occurrences**

Replace "Spawnbot" / "spawnbot" with "Spawnbot" / "spawnbot" in all frontend files.

- [ ] **Step 3: Update page title in index.html**

- [ ] **Step 4: Build frontend to verify**

```bash
cd web/frontend && pnpm install && pnpm build
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: rebrand frontend Spawnbot → Spawnbot"
```

---

## Phase 2: Identity System

### Task 6: Simplify LoadBootstrapFiles to SOUL.md only

**Files:**
- Modify: `pkg/agent/context.go:446-477` (rewrite `LoadBootstrapFiles`)
- Modify: `pkg/agent/context.go:239-243` (update `sourcePaths`)
- Delete: `pkg/agent/definition.go` (255 lines)
- Delete: `pkg/agent/definition_test.go` (302 lines)

- [ ] **Step 1: Write test for SOUL.md-only loading**

Create `pkg/agent/context_soul_test.go`:
```go
func TestLoadBootstrapFiles_OnlySoulMd(t *testing.T) {
    workspace := t.TempDir()
    // Write SOUL.md
    os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("You are Spawnbot."), 0644)
    // Write AGENT.md (should be ignored)
    os.WriteFile(filepath.Join(workspace, "AGENT.md"), []byte("agent stuff"), 0644)
    // Write AGENTS.md (should be ignored)
    os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("agents stuff"), 0644)
    // Write IDENTITY.md (should be ignored)
    os.WriteFile(filepath.Join(workspace, "IDENTITY.md"), []byte("identity stuff"), 0644)

    cb := NewContextBuilder(workspace)
    content, err := cb.LoadBootstrapFiles()
    require.NoError(t, err)

    assert.Contains(t, content, "You are Spawnbot")
    assert.NotContains(t, content, "agent stuff")
    assert.NotContains(t, content, "agents stuff")
    assert.NotContains(t, content, "identity stuff")
}

func TestLoadBootstrapFiles_ErrorsWhenSoulMdMissing(t *testing.T) {
    workspace := t.TempDir()
    // No SOUL.md — should error, not silently return empty
    cb := NewContextBuilder(workspace)
    _, err := cb.LoadBootstrapFiles()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "SOUL.md")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./pkg/agent/ -run TestLoadBootstrapFiles_OnlySoulMd -v
```
Expected: FAIL (current code loads AGENT.md and IDENTITY.md).

- [ ] **Step 3: Rewrite LoadBootstrapFiles**

In `pkg/agent/context.go`, replace the entire `LoadBootstrapFiles()` method. Change signature to return `(string, error)` — missing SOUL.md is a fatal config error, not a silent fallback:
```go
func (cb *ContextBuilder) LoadBootstrapFiles() (string, error) {
    soulPath := filepath.Join(cb.workspace, "SOUL.md")
    data, err := os.ReadFile(soulPath)
    if err != nil {
        return "", fmt.Errorf("SOUL.md not found at %s — run 'spawnbot onboard' to create it: %w", soulPath, err)
    }
    return fmt.Sprintf("## SOUL.md\n\n%s\n\n", string(data)), nil
}
```

Update all callers of `LoadBootstrapFiles()` in `BuildSystemPrompt()` to handle the error (log and continue with empty bootstrap if error, since the agent may be starting before onboarding in some flows).

- [ ] **Step 4: Update sourcePaths to track SOUL.md directly**

Replace the `sourcePaths()` method to return `[]string{filepath.Join(cb.workspace, "SOUL.md"), ...}` instead of going through `AgentDefinition`.

- [ ] **Step 5: Delete definition.go and definition_test.go**

```bash
rm pkg/agent/definition.go pkg/agent/definition_test.go
```

- [ ] **Step 6: Fix compilation errors from deleted definition.go**

Any references to `AgentDefinition`, `LoadAgentDefinition`, `trackedPaths` in `context.go` or other files must be removed or replaced.

- [ ] **Step 7: Run tests**

```bash
go test ./pkg/agent/ -v -count=1
```
Expected: all pass, new test passes.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: simplify identity to SOUL.md only, remove AGENT.md/IDENTITY.md"
```

### Task 7: Create default workspace files template

**Files:**
- Create: `pkg/onboard/templates/SOUL.md`
- Create: `pkg/onboard/templates/USER.md`
- Create: `pkg/onboard/templates/GOALS.md`
- Create: `pkg/onboard/templates/PLAYBOOK.md`
- Create: `pkg/onboard/templates/HEARTBEAT.md`

- [ ] **Step 1: Create template directory and files**

`SOUL.md` template:
```markdown
# Spawnbot

You are Spawnbot, a personal AI agent for {{.UserName}}.

## Reference Files
These files contain important context. Read them with read_file when relevant:
- workspace/USER.md — what you know about the user (you can update this)
- workspace/GOALS.md — current objectives (you can update this)
- workspace/PLAYBOOK.md — how you operate
- workspace/HEARTBEAT.md — your autonomy triggers and proactive behaviors
```

`USER.md`, `GOALS.md`, `PLAYBOOK.md`, `HEARTBEAT.md` — minimal starter templates with placeholder sections.

- [ ] **Step 2: Commit**

```bash
git add -A
git commit -m "feat: add identity workspace file templates for onboarding"
```

---

## Phase 3: Semantic Memory System

### Task 8: SQLite store with FTS5

**Files:**
- Create: `pkg/memory/sqlite.go`
- Create: `pkg/memory/sqlite_test.go`
- Modify: `go.mod` (add `github.com/mattn/go-sqlite3`)

- [ ] **Step 1: Add mattn/go-sqlite3 dependency**

```bash
go get github.com/mattn/go-sqlite3
```

- [ ] **Step 2: Write test for SQLite store CRUD + FTS5 search**

Create `pkg/memory/sqlite_test.go`:
```go
func TestSQLiteStore_StoreAndSearchFTS(t *testing.T) {
    store, err := NewSQLiteStore(t.TempDir(), 768)
    require.NoError(t, err)
    defer store.Close()

    err = store.Store(Chunk{Content: "Go is a compiled language", SourceFile: "test.md", Heading: "Languages"})
    require.NoError(t, err)

    results, err := store.SearchFTS("compiled language", 10)
    require.NoError(t, err)
    require.Len(t, results, 1)
    assert.Contains(t, results[0].Content, "compiled")
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
CGO_ENABLED=1 go test ./pkg/memory/ -run TestSQLiteStore_StoreAndSearchFTS -v
```

- [ ] **Step 4: Implement SQLiteStore**

Create `pkg/memory/sqlite.go` with:
- `NewSQLiteStore(dbDir string, vecDimensions int)` — opens DB, creates tables (memory_chunks, memory_fts)
- `Store(chunk Chunk) error` — insert with ULID, SHA-256 dedup, FTS5 sync
- `SearchFTS(query string, limit int) ([]Chunk, error)` — FTS5 MATCH query
- `Close() error`

Register driver as `"sqlite3_memory"` to avoid conflict with modernc driver.

- [ ] **Step 5: Run test to verify it passes**

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: add SQLite memory store with FTS5 full-text search"
```

### Task 9: sqlite-vec integration for vector search

**Files:**
- Modify: `pkg/memory/sqlite.go` (add vec0 table, vector insert/search)
- Modify: `pkg/memory/sqlite_test.go`
- Modify: `go.mod` (add `github.com/asg017/sqlite-vec-go-bindings/cgo`)

- [ ] **Step 1: Add sqlite-vec dependency**

```bash
go get github.com/asg017/sqlite-vec-go-bindings/cgo
```

- [ ] **Step 2: Write test for vector store and search**

```go
func TestSQLiteStore_VectorSearch(t *testing.T) {
    store, err := NewSQLiteStore(t.TempDir(), 4) // 4-dim for testing
    require.NoError(t, err)
    defer store.Close()

    embedding := []float32{0.1, 0.2, 0.3, 0.4}
    err = store.StoreWithEmbedding(Chunk{Content: "test content", SourceFile: "test.md"}, embedding)
    require.NoError(t, err)

    query := []float32{0.1, 0.2, 0.3, 0.5}
    results, err := store.SearchVec(query, 10)
    require.NoError(t, err)
    require.Len(t, results, 1)
}
```

- [ ] **Step 3: Run test to verify it fails**

- [ ] **Step 4: Implement vector storage and search**

Add to `sqlite.go`:
- `sqlite_vec.Auto()` in init
- `memory_vec` table creation with configurable dimensions
- `StoreWithEmbedding(chunk Chunk, embedding []float32) error`
- `SearchVec(queryEmbedding []float32, limit int) ([]ScoredChunk, error)`

- [ ] **Step 5: Run test to verify it passes**

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: add sqlite-vec vector search to memory store"
```

### Task 10: Embeddings provider

**Files:**
- Create: `pkg/memory/embeddings.go`
- Create: `pkg/memory/embeddings_test.go`

- [ ] **Step 1: Write test for Gemini embeddings provider**

```go
func TestGeminiEmbeddings_Embed(t *testing.T) {
    if os.Getenv("GEMINI_API_KEY") == "" {
        t.Skip("GEMINI_API_KEY not set")
    }
    provider := NewGeminiEmbeddingProvider(os.Getenv("GEMINI_API_KEY"), "text-embedding-004", 768)
    result, err := provider.Embed(context.Background(), "Hello world")
    require.NoError(t, err)
    assert.Len(t, result, 768)
}
```

- [ ] **Step 2: Implement EmbeddingProvider interface**

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    Dimensions() int
}
```

Implement `GeminiEmbeddingProvider` and `OpenAIEmbeddingProvider` (both are simple HTTP POST calls).

- [ ] **Step 3: Implement factory**

```go
func NewEmbeddingProvider(cfg EmbeddingConfig) (EmbeddingProvider, error)
```

- [ ] **Step 4: Run tests**

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add embedding providers (Gemini, OpenAI) for memory system"
```

### Task 11: Hybrid search with Reciprocal Rank Fusion

**Files:**
- Create: `pkg/memory/search.go`
- Create: `pkg/memory/search_test.go`

- [ ] **Step 1: Write test for hybrid search**

```go
func TestHybridSearch(t *testing.T) {
    store := setupTestStoreWithData(t) // helper: creates store, inserts 10 chunks with embeddings
    searcher := NewHybridSearcher(store, mockEmbedder)

    results, err := searcher.Search(context.Background(), "Go programming", 5)
    require.NoError(t, err)
    assert.True(t, len(results) > 0)
    // Results should be ranked by combined FTS + vec score
    assert.True(t, results[0].Score >= results[len(results)-1].Score)
}
```

- [ ] **Step 2: Implement HybridSearcher**

`search.go`:
- `HybridSearcher` struct holding `*SQLiteStore` + `EmbeddingProvider`
- `Search(ctx, query, limit)` — runs FTS5 search + embeds query + vec search, merges via RRF with temporal decay
- RRF formula: `score = (w_fts / (k + fts_rank)) + (w_vec / (k + vec_rank)) * decay(age)`
- Default weights: 0.5/0.5, k=60

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: add hybrid search with Reciprocal Rank Fusion"
```

### Task 12: Markdown indexer

**Files:**
- Create: `pkg/memory/indexer.go`
- Create: `pkg/memory/indexer_test.go`

- [ ] **Step 1: Write test for markdown chunking**

```go
func TestIndexer_ChunkByHeadings(t *testing.T) {
    md := "# Title\nIntro\n## Section A\nContent A\n## Section B\nContent B"
    chunks := ChunkMarkdown(md, "test.md")
    assert.Len(t, chunks, 3) // title + section A + section B
    assert.Equal(t, "Section A", chunks[1].Heading)
}

func TestIndexer_SkipsUnchanged(t *testing.T) {
    store := setupTestStore(t)
    indexer := NewIndexer(store, nil) // nil embedder for this test
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "test.md"), []byte("## Heading\nContent"), 0644)

    changed1, _ := indexer.IndexDirectory(dir)
    assert.Equal(t, 1, changed1)

    changed2, _ := indexer.IndexDirectory(dir)
    assert.Equal(t, 0, changed2) // SHA-256 dedup, nothing changed
}
```

- [ ] **Step 2: Implement Indexer**

`indexer.go`:
- `ChunkMarkdown(content, sourceFile string) []Chunk` — split by `##` headings
- `Indexer` struct holding `*SQLiteStore` + `EmbeddingProvider`
- `IndexDirectory(dir string) (changed int, err error)` — walks `.md` files, chunks, SHA-256 skips unchanged, stores + embeds new chunks

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: add markdown indexer with heading chunking and SHA-256 dedup"
```

### Task 13: Memory agent tools

**Files:**
- Create: `pkg/memory/tools.go`
- Create: `pkg/memory/tools_test.go`

- [ ] **Step 1: Write tests for memory_store, memory_search, memory_recall tools**

Test each tool's `Execute()` method with mock store.

- [ ] **Step 2: Implement tools**

Three tools implementing spawnbot's `tools.Tool` interface:
- `MemoryStoreTool` — stores a new memory chunk (agent calls this when something is worth remembering)
- `MemorySearchTool` — runs hybrid search, returns ranked results
- `MemoryRecallTool` — retrieves by source file or heading

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: add memory_store, memory_search, memory_recall agent tools"
```

### Task 14: Wire memory into agent

**Files:**
- Modify: `pkg/agent/context.go` (`GetMemoryContext`, `sourcePaths`)
- Modify: `pkg/agent/instance.go` (register memory tools)
- Modify: `pkg/config/config.go` (add embeddings config)
- Modify: `pkg/config/defaults.go`

- [ ] **Step 1: Add embeddings config to config struct**

In `pkg/config/config.go`, add:
```go
type EmbeddingsConfig struct {
    Provider   string `json:"provider" env:"SPAWNBOT_EMBEDDINGS_PROVIDER"`
    Model      string `json:"model" env:"SPAWNBOT_EMBEDDINGS_MODEL"`
    APIKeyEnv  string `json:"api_key_env"`
    Dimensions int    `json:"dimensions" env:"SPAWNBOT_EMBEDDINGS_DIMENSIONS"`
}
```

- [ ] **Step 2: Update GetMemoryContext to use SQLite store**

Replace the flat MEMORY.md read with a query to the SQLite store for recent memories (top 10 by recency).

- [ ] **Step 3: Update sourcePaths to track SQLite DB file**

Replace `memory/MEMORY.md` path with `memory/spawnbot.db` in `sourcePaths()`.

- [ ] **Step 4: Register memory tools in AgentInstance**

In `pkg/agent/instance.go`, add memory tools to the tool registry during agent construction.

- [ ] **Step 5: Build and test**

```bash
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go test ./pkg/agent/ -v -count=1 -short
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: wire semantic memory into agent context and tool registry"
```

---

## Phase 4: Autonomy

### Task 15: Idle trigger monitor

**Files:**
- Create: `pkg/autonomy/idle.go`
- Create: `pkg/autonomy/idle_test.go`

- [ ] **Step 1: Write test**

```go
func TestIdleMonitor_FiresAfterThreshold(t *testing.T) {
    fired := make(chan string, 1)
    monitor := NewIdleMonitor(100*time.Millisecond, func(channel string) { fired <- channel })
    monitor.RecordActivity("telegram")
    // Wait for threshold
    select {
    case ch := <-fired:
        assert.Equal(t, "telegram", ch)
    case <-time.After(500 * time.Millisecond):
        t.Fatal("idle trigger did not fire")
    }
}
```

- [ ] **Step 2: Implement IdleMonitor**

`idle.go`:
- `IdleMonitor` with per-channel last-activity timestamps
- `RecordActivity(channel string)` — resets timer
- Background goroutine checks every minute, fires callback when threshold exceeded
- `Start(ctx context.Context)` / `Stop()`

- [ ] **Step 3: Run tests, commit**

```bash
git add -A
git commit -m "feat: add idle trigger monitor for proactive agent wake-up"
```

### Task 16: Feed poller

**Files:**
- Create: `pkg/autonomy/poller.go`
- Create: `pkg/autonomy/poller_test.go`
- Create: `pkg/autonomy/config.go`
- Modify: `go.mod` (add `github.com/mmcdole/gofeed`)

- [ ] **Step 1: Add gofeed dependency**

```bash
go get github.com/mmcdole/gofeed
```

- [ ] **Step 2: Write test**

```go
func TestFeedPoller_DetectsNewItems(t *testing.T) {
    // Use httptest server serving a test RSS feed
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(testRSSFeed))
    }))
    defer server.Close()

    var received []FeedItem
    poller := NewFeedPoller([]FeedConfig{{URL: server.URL, CheckIntervalMinutes: 1}},
        func(items []FeedItem, cfg FeedConfig) { received = append(received, items...) })

    poller.PollOnce(context.Background())
    assert.True(t, len(received) > 0)

    // Second poll — no new items
    received = nil
    poller.PollOnce(context.Background())
    assert.Len(t, received, 0)
}
```

- [ ] **Step 3: Implement FeedPoller**

`poller.go`:
- `FeedPoller` struct with feed configs and seen-item tracking (by GUID/link hash)
- `PollOnce(ctx)` — checks all feeds, calls callback only for new items
- `Start(ctx)` — background goroutine polling at each feed's interval
- `Stop()`

`config.go`:
- `AutonomyConfig`, `FeedConfig`, `IdleTriggerConfig` structs

- [ ] **Step 4: Run tests, commit**

```bash
git add -A
git commit -m "feat: add feed poller for RSS/Atom monitoring"
```

### Task 17: Wire autonomy into gateway

**Files:**
- Modify: `pkg/gateway/gateway.go` (start idle monitor + feed poller)
- Modify: `pkg/config/config.go` (add autonomy config section)
- Modify: `pkg/config/defaults.go`

- [ ] **Step 1: Write integration test for autonomy wiring**

Create `pkg/gateway/autonomy_test.go`:
```go
func TestGateway_AutonomyWiring(t *testing.T) {
    cfg := config.DefaultConfig()
    cfg.Autonomy.IdleTrigger.Enabled = true
    cfg.Autonomy.IdleTrigger.ThresholdHours = 1

    // Verify config parses without error
    require.True(t, cfg.Autonomy.IdleTrigger.Enabled)

    // Verify IdleMonitor can be constructed from config
    monitor := autonomy.NewIdleMonitor(
        time.Duration(cfg.Autonomy.IdleTrigger.ThresholdHours)*time.Hour,
        func(channel string) {},
    )
    require.NotNil(t, monitor)
}
```

- [ ] **Step 2: Add autonomy config to config struct**

- [ ] **Step 3: Wire into gateway startup**

In the gateway startup sequence, initialize `IdleMonitor` and `FeedPoller` from config. Hook `IdleMonitor.RecordActivity` into the message bus (every inbound user message resets the timer). Feed poller callback sends an internal message to the agent via the message bus.

- [ ] **Step 4: Run tests**

```bash
CGO_ENABLED=1 go test ./pkg/gateway/ -run TestGateway_AutonomyWiring -v
CGO_ENABLED=1 go build ./cmd/spawnbot/
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: wire idle triggers and feed poller into gateway"
```

---

## Phase 5: Approval Modes

### Task 18: YOLO/Approval ToolApprover hook

**Files:**
- Create: `pkg/agent/approval.go`
- Create: `pkg/agent/approval_test.go`
- Modify: `pkg/config/config.go` (add approval_mode, approval_timeout_seconds)

- [ ] **Step 1: Write tests**

```go
func TestApprovalHook_YOLOMode(t *testing.T) {
    hook := NewApprovalHook("yolo", 300)
    approved, _, _ := hook.ApproveTool(context.Background(), "exec", map[string]any{"command": "ls"})
    assert.True(t, approved)
}

func TestApprovalHook_ApprovalMode_DangerousToolBlocked(t *testing.T) {
    // Dangerous tool with no responder — should timeout and deny
    hook := NewApprovalHook("approval", 1) // 1 second timeout for test
    approved, _, reason := hook.ApproveTool(context.Background(), "exec", map[string]any{"command": "rm -rf /"})
    assert.False(t, approved)
    assert.Contains(t, reason, "timed out")
}

func TestApprovalHook_ApprovalMode_SafeToolAllowed(t *testing.T) {
    // Safe tools (read_file, list_dir, web_search) should auto-approve even in approval mode
    hook := NewApprovalHook("approval", 300)
    approved, _, _ := hook.ApproveTool(context.Background(), "read_file", map[string]any{"path": "/tmp/test"})
    assert.True(t, approved)
}

func TestApprovalHook_ApprovalMode_DangerousToolWithCallback(t *testing.T) {
    // Test that approval callback is invoked for dangerous tools
    hook := NewApprovalHook("approval", 300)
    hook.SetApprovalCallback(func(toolName string, args map[string]any) bool {
        return true // simulate user approving
    })
    approved, _, _ := hook.ApproveTool(context.Background(), "exec", map[string]any{"command": "ls"})
    assert.True(t, approved)
}
```

- [ ] **Step 2: Implement ApprovalHook**

`approval.go`:
- Implements `ToolApprover` interface
- YOLO mode: always returns approved
- Approval mode: checks if tool is in dangerous list (exec, write_file, edit_file), sends approval request via callback, blocks until response or timeout
- Configurable timeout (default 300s)

- [ ] **Step 3: Wire into agent hooks**

Register the approval hook in `AgentInstance` construction based on config.

- [ ] **Step 4: Run tests, commit**

```bash
git add -A
git commit -m "feat: add YOLO/Approval tool approval modes"
```

---

## Phase 6: Onboarding

### Task 19: CLI onboarding wizard

**Files:**
- Modify: `cmd/spawnbot/internal/onboard/command.go`
- Modify: `cmd/spawnbot/internal/onboard/helpers.go`
- Modify: `go.mod` (add `github.com/charmbracelet/huh`)

- [ ] **Step 1: Add charmbracelet/huh dependency**

```bash
go get github.com/charmbracelet/huh
```

- [ ] **Step 2: Rewrite onboard command**

Replace existing `onboard` command with interactive wizard:
1. Provider selection (OpenRouter / Anthropic / OpenAI / Custom)
2. API key input (masked)
3. API key validation (test call)
4. User name
5. Approval mode (YOLO / Approval)
6. Telegram bot token (optional)
7. Embedding provider + API key

- [ ] **Step 3: Write workspace files from templates**

After wizard completes, write SOUL.md (with user name interpolated), USER.md, GOALS.md, PLAYBOOK.md, HEARTBEAT.md to workspace.

- [ ] **Step 4: Test manually**

```bash
CGO_ENABLED=1 go build -o spawnbot ./cmd/spawnbot/
./spawnbot onboard
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add interactive CLI onboarding wizard"
```

### Task 20: Web onboarding wizard

**Files:**
- Modify: `web/backend/` (add onboarding API routes)
- Create: `web/frontend/src/components/onboarding/` (wizard pages)
- Modify: `web/frontend/src/routes/` (add onboarding route)

- [ ] **Step 1: Write backend API tests**

Create `web/backend/api/onboarding_test.go`:
```go
func TestOnboardingStatus_NotCompleted(t *testing.T) {
    tmpDir := t.TempDir()
    handler := NewOnboardingHandler(tmpDir)
    req := httptest.NewRequest("GET", "/api/onboarding/status", nil)
    w := httptest.NewRecorder()
    handler.Status(w, req)
    assert.Equal(t, 200, w.Code)
    assert.Contains(t, w.Body.String(), `"completed":false`)
}

func TestOnboardingComplete_WritesConfig(t *testing.T) {
    tmpDir := t.TempDir()
    handler := NewOnboardingHandler(tmpDir)
    body := `{"provider":"openrouter","api_key":"test","user_name":"Alice","approval_mode":"yolo"}`
    req := httptest.NewRequest("POST", "/api/onboarding/complete", strings.NewReader(body))
    w := httptest.NewRecorder()
    handler.Complete(w, req)
    assert.Equal(t, 200, w.Code)
    // Verify SOUL.md was written
    _, err := os.Stat(filepath.Join(tmpDir, "workspace", "SOUL.md"))
    assert.NoError(t, err)
}
```

- [ ] **Step 2: Implement backend API routes**

- `GET /api/onboarding/status` — returns `{completed: bool}`
- `POST /api/onboarding/validate-key` — tests provider + API key
- `POST /api/onboarding/complete` — writes config + workspace files

- [ ] **Step 3: Run backend tests**

```bash
CGO_ENABLED=1 go test ./web/backend/api/ -run TestOnboarding -v
```

- [ ] **Step 4: Create frontend wizard component**

Multi-step form with the same fields as CLI wizard. Detect `onboarding/status` on app load — if not completed, redirect to wizard.

- [ ] **Step 5: Build and verify frontend**

```bash
cd web/frontend && pnpm build
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: add web onboarding wizard"
```

---

## Phase 7: Enhanced Web UI Chat

### Task 21: Enhanced chat page

**Files:**
- Modify: `web/frontend/src/components/chat/chat-page.tsx`
- Create: `web/frontend/src/components/chat/markdown-renderer.tsx`
- Create: `web/frontend/src/components/chat/tool-call-card.tsx`
- Modify: `web/frontend/src/components/chat/session-history-menu.tsx`

- [ ] **Step 1: Add markdown rendering library**

```bash
cd web/frontend && pnpm add react-markdown remark-gfm rehype-highlight
```

- [ ] **Step 2: Create MarkdownRenderer component**

Renders agent responses with code syntax highlighting, lists, headers, tables.

- [ ] **Step 3: Create ToolCallCard component**

Collapsible card showing tool name, status indicator (pending/running/done/error), expandable input/output.

- [ ] **Step 4: Enhance session history sidebar**

Update existing `session-history-menu.tsx` to show session list with timestamps, click to load.

- [ ] **Step 5: Add streaming status indicators**

Show "thinking...", "executing tool...", "sub-agent spawned" states in the chat UI.

- [ ] **Step 6: Build and test**

```bash
cd web/frontend && pnpm build
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: enhanced chat page with markdown, tool calls, session sidebar"
```

---

## Phase 8: Integration & Polish

### Task 22: End-to-end integration test

**Files:**
- Create: `tests/integration/e2e_test.go`

- [ ] **Step 1: Write E2E test**

Create `tests/integration/e2e_test.go`:
```go
func TestE2E_FullFlow(t *testing.T) {
    if os.Getenv("SPAWNBOT_E2E") == "" {
        t.Skip("Set SPAWNBOT_E2E=1 to run integration tests")
    }

    // Setup: temp workspace with config
    workspace := t.TempDir()
    configDir := t.TempDir()

    // 1. Write onboarding files (simulates onboard completion)
    writeTestConfig(t, configDir)
    writeTestSOUL(t, workspace)
    writeTestUSER(t, workspace)

    // 2. Initialize memory store
    store, err := memory.NewSQLiteStore(filepath.Join(workspace, "memory"), 4)
    require.NoError(t, err)
    defer store.Close()

    // 3. Store a memory
    err = store.Store(memory.Chunk{
        Content:    "The user prefers Go over Python",
        SourceFile: "USER.md",
        Heading:    "Preferences",
    })
    require.NoError(t, err)

    // 4. Search memory via FTS
    results, err := store.SearchFTS("Go Python", 5)
    require.NoError(t, err)
    require.Len(t, results, 1)
    assert.Contains(t, results[0].Content, "Go over Python")

    // 5. Verify SOUL.md loads
    cb := agent.NewContextBuilder(workspace)
    content, err := cb.LoadBootstrapFiles()
    require.NoError(t, err)
    assert.Contains(t, content, "Spawnbot")
}
```

- [ ] **Step 2: Run and fix any issues**

```bash
CGO_ENABLED=1 go test ./tests/integration/ -v -count=1
```

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test: add end-to-end integration test"
```

### Task 23: Update README and documentation

**Files:**
- Modify: `README.md`
- Delete or update any spawnbot-specific docs

- [ ] **Step 1: Rewrite README for Spawnbot v5**

Cover: what it is, quickstart (install, onboard, run), features, configuration, building from source.

- [ ] **Step 2: Commit**

```bash
git add -A
git commit -m "docs: rewrite README for Spawnbot v5"
```

### Task 24: CI and release setup

**Files:**
- Create: `.github/workflows/build.yml`
- Create: `.github/workflows/release.yml`
- Modify: `.goreleaser.yml` (if exists, update binary names)

- [ ] **Step 1: Create build workflow**

Go build + test on push/PR. Use `CGO_ENABLED=1` for memory system.

- [ ] **Step 2: Create release workflow**

Tag-triggered, builds for linux-amd64, linux-arm64, darwin-arm64. Includes frontend build.

- [ ] **Step 3: Commit and push**

```bash
git add -A
git commit -m "ci: add build and release workflows"
git push origin main
```
