# Tool Result Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist large tool results to disk and send a ~2KB preview to the LLM, preventing context window overflow.

**Architecture:** Intercept tool results after execution but before adding to message history. Check size against per-tool thresholds and per-turn aggregate budget. If over, write full content to session-scoped disk file and replace with a preview + file path reference. Model can `read_file` the full result when needed.

**Tech Stack:** Go, standard library (`os`, `path/filepath`, `strings`, `fmt`)

---

### File Structure

| File | Responsibility |
|------|----------------|
| `pkg/tools/resultstore/store.go` (create) | ResultStore: persist content to disk, generate preview |
| `pkg/tools/resultstore/store_test.go` (create) | ResultStore unit tests |
| `pkg/agent/result_persist.go` (create) | Size gate function: threshold check + ResultStore wiring |
| `pkg/agent/result_persist_test.go` (create) | Size gate unit tests |
| `pkg/config/config.go` (modify) | Add ToolResultPersistenceConfig type and field |
| `pkg/config/defaults.go` (modify) | Add default values |
| `pkg/agent/loop.go` (modify) | Wire maybePersistResult into tool execution loop |

---

### Task 1: ResultStore — Preview Generation

**Files:**
- Create: `pkg/tools/resultstore/store.go`
- Test: `pkg/tools/resultstore/store_test.go`

- [ ] **Step 1: Write the failing test for generatePreview**

```go
// pkg/tools/resultstore/store_test.go
package resultstore

import (
	"strings"
	"testing"
)

func TestGeneratePreview_TruncatesAtNewline(t *testing.T) {
	// 3 lines, each 1000 chars + newline = 3003 bytes total
	line := strings.Repeat("a", 1000)
	content := line + "\n" + line + "\n" + line + "\n"

	preview := generatePreview(content, 2000)

	// Should include first line (1001 bytes with newline) and second line (1001 bytes)
	// Total 2002 bytes > 2000, so should cut at end of first line
	if len(preview) > 2000 {
		t.Errorf("preview too long: got %d bytes, want <= 2000", len(preview))
	}
	if !strings.HasSuffix(preview, "\n") {
		t.Error("preview should end at a newline boundary")
	}
}

func TestGeneratePreview_ShortContent(t *testing.T) {
	content := "short content"
	preview := generatePreview(content, 2000)
	if preview != content {
		t.Errorf("short content should pass through unchanged: got %q", preview)
	}
}

func TestGeneratePreview_EmptyContent(t *testing.T) {
	preview := generatePreview("", 2000)
	if preview != "" {
		t.Errorf("empty content should return empty: got %q", preview)
	}
}

func TestGeneratePreview_NoNewlines(t *testing.T) {
	// Content with no newlines longer than maxBytes — hard truncate
	content := strings.Repeat("x", 3000)
	preview := generatePreview(content, 2000)
	if len(preview) != 2000 {
		t.Errorf("no-newline content should hard truncate: got %d bytes, want 2000", len(preview))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -run TestGeneratePreview -v`
Expected: FAIL — package/function does not exist

- [ ] **Step 3: Implement generatePreview**

```go
// pkg/tools/resultstore/store.go
package resultstore

import (
	"strings"
)

// generatePreview returns the first maxBytes of content, cutting at the last
// newline boundary before the limit. If there are no newlines, hard truncates.
// Returns content unchanged if it fits within maxBytes.
func generatePreview(content string, maxBytes int) string {
	if len(content) <= maxBytes {
		return content
	}

	truncated := content[:maxBytes]
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > 0 {
		return truncated[:lastNewline+1]
	}
	// No newline found — hard truncate
	return truncated
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -run TestGeneratePreview -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/resultstore/store.go pkg/tools/resultstore/store_test.go
git commit -m "feat(resultstore): add preview generation with newline-boundary truncation"
```

---

### Task 2: ResultStore — Persist to Disk

**Files:**
- Modify: `pkg/tools/resultstore/store.go`
- Modify: `pkg/tools/resultstore/store_test.go`

- [ ] **Step 1: Write the failing tests for ResultStore and Persist**

```go
// Append to pkg/tools/resultstore/store_test.go

func TestNewResultStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "tool-results")

	store, err := NewResultStore(storeDir)
	if err != nil {
		t.Fatalf("NewResultStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("store should not be nil")
	}

	info, err := os.Stat(storeDir)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("should be a directory")
	}
}

func TestPersist_WritesFileAndGeneratesPreview(t *testing.T) {
	store, err := NewResultStore(filepath.Join(t.TempDir(), "tool-results"))
	if err != nil {
		t.Fatalf("NewResultStore failed: %v", err)
	}

	content := strings.Repeat("line of content\n", 5000) // ~80KB

	result, err := store.Persist("tool_use_abc123", content, 2000)
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Check file was written
	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("should be able to read persisted file: %v", err)
	}
	if string(data) != content {
		t.Error("persisted content should match original")
	}

	// Check preview
	if len(result.Preview) > 2000 {
		t.Errorf("preview too long: %d bytes", len(result.Preview))
	}
	if result.Preview == "" {
		t.Error("preview should not be empty")
	}

	// Check metadata
	if result.OrigSize != len(content) {
		t.Errorf("OrigSize = %d, want %d", result.OrigSize, len(content))
	}
}

func TestPersist_FileNameFromToolUseID(t *testing.T) {
	store, err := NewResultStore(filepath.Join(t.TempDir(), "tool-results"))
	if err != nil {
		t.Fatalf("NewResultStore failed: %v", err)
	}

	result, err := store.Persist("toolu_01ABC", "some content", 2000)
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	if !strings.HasSuffix(result.FilePath, "toolu_01ABC.txt") {
		t.Errorf("file should be named after toolUseID: got %s", result.FilePath)
	}
}
```

Add these imports to the test file's import block:

```go
"os"
"path/filepath"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -run "TestNewResultStore|TestPersist" -v`
Expected: FAIL — NewResultStore and Persist not defined

- [ ] **Step 3: Implement ResultStore and Persist**

Add to `pkg/tools/resultstore/store.go`:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResultStore persists large tool results to disk and generates previews.
type ResultStore struct {
	baseDir string
}

// PersistedResult contains the file path, preview, and original size of a persisted tool result.
type PersistedResult struct {
	FilePath string
	Preview  string
	OrigSize int
}

// NewResultStore creates a ResultStore rooted at baseDir, creating the directory if needed.
func NewResultStore(baseDir string) (*ResultStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("resultstore: create directory: %w", err)
	}
	return &ResultStore{baseDir: baseDir}, nil
}

// Persist writes the full content to disk as {toolUseID}.txt and returns a
// PersistedResult with the file path, a preview truncated to previewMaxBytes,
// and the original content size.
func (rs *ResultStore) Persist(toolUseID, content string, previewMaxBytes int) (*PersistedResult, error) {
	filePath := filepath.Join(rs.baseDir, toolUseID+".txt")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("resultstore: write file: %w", err)
	}
	return &PersistedResult{
		FilePath: filePath,
		Preview:  generatePreview(content, previewMaxBytes),
		OrigSize: len(content),
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -v`
Expected: PASS (7 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/resultstore/store.go pkg/tools/resultstore/store_test.go
git commit -m "feat(resultstore): add ResultStore with disk persistence and preview"
```

---

### Task 3: ResultStore — Format Preview Message

**Files:**
- Modify: `pkg/tools/resultstore/store.go`
- Modify: `pkg/tools/resultstore/store_test.go`

- [ ] **Step 1: Write the failing test for FormatPreviewMessage**

```go
// Append to pkg/tools/resultstore/store_test.go

func TestFormatPreviewMessage(t *testing.T) {
	pr := &PersistedResult{
		FilePath: "/home/user/.spawnbot/workspace/sessions/abc/tool-results/toolu_123.txt",
		Preview:  "first line\nsecond line\n",
		OrigSize: 125430,
	}

	msg := pr.FormatPreviewMessage()

	if !strings.Contains(msg, `path="/home/user/.spawnbot/workspace/sessions/abc/tool-results/toolu_123.txt"`) {
		t.Error("message should contain file path")
	}
	if !strings.Contains(msg, `original_size="125430"`) {
		t.Error("message should contain original size")
	}
	if !strings.Contains(msg, "first line\nsecond line\n") {
		t.Error("message should contain preview content")
	}
	if !strings.Contains(msg, "read_file") {
		t.Error("message should instruct model to use read_file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -run TestFormatPreviewMessage -v`
Expected: FAIL — FormatPreviewMessage not defined

- [ ] **Step 3: Implement FormatPreviewMessage**

Add to `pkg/tools/resultstore/store.go`:

```go
// FormatPreviewMessage returns the XML-wrapped preview message that replaces
// the original tool result content in the LLM conversation.
func (pr *PersistedResult) FormatPreviewMessage() string {
	return fmt.Sprintf(
		"<persisted-tool-result path=%q original_size=%q>\n%s</persisted-tool-result>\n"+
			"This tool result was too large to include in full (%d bytes). "+
			"A preview is shown above. Use read_file to access the complete output at the path above.",
		pr.FilePath,
		fmt.Sprintf("%d", pr.OrigSize),
		pr.Preview,
		pr.OrigSize,
	)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -v`
Expected: PASS (8 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/resultstore/store.go pkg/tools/resultstore/store_test.go
git commit -m "feat(resultstore): add FormatPreviewMessage for LLM-facing preview"
```

---

### Task 4: Configuration

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `pkg/config/defaults.go`

- [ ] **Step 1: Add ToolResultPersistenceConfig type to config.go**

Add this type definition near the other tool config types (after `ReadFileToolConfig` around line 1167):

```go
// ToolResultPersistenceConfig controls disk persistence of large tool results.
type ToolResultPersistenceConfig struct {
	Enabled            bool           `json:"enabled"`
	DefaultMaxChars    int            `json:"default_max_chars"`
	PerTurnBudgetChars int            `json:"per_turn_budget_chars"`
	PreviewSizeBytes   int            `json:"preview_size_bytes"`
	ToolOverrides      map[string]int `json:"tool_overrides,omitempty"`
}
```

- [ ] **Step 2: Add field to ToolsConfig struct**

Add this field to the `ToolsConfig` struct (around line 1200, after the `WriteFile` field):

```go
	ResultPersistence ToolResultPersistenceConfig `json:"result_persistence"`
```

- [ ] **Step 3: Add defaults in defaults.go**

In the `DefaultConfig()` function's `Tools: ToolsConfig{` block, add after `WriteFile`:

```go
			ResultPersistence: ToolResultPersistenceConfig{
				Enabled:            true,
				DefaultMaxChars:    50000,
				PerTurnBudgetChars: 200000,
				PreviewSizeBytes:   2000,
			},
```

- [ ] **Step 4: Verify it compiles**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go build ./pkg/config/`
Expected: Success (exit 0)

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go pkg/config/defaults.go
git commit -m "feat(config): add ToolResultPersistenceConfig with defaults"
```

---

### Task 5: Size Gate Function

**Files:**
- Create: `pkg/agent/result_persist.go`
- Create: `pkg/agent/result_persist_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/agent/result_persist_test.go
package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools/resultstore"
)

func newTestStore(t *testing.T) *resultstore.ResultStore {
	t.Helper()
	store, err := resultstore.NewResultStore(filepath.Join(t.TempDir(), "tool-results"))
	if err != nil {
		t.Fatalf("NewResultStore: %v", err)
	}
	return store
}

func defaultPersistCfg() config.ToolResultPersistenceConfig {
	return config.ToolResultPersistenceConfig{
		Enabled:            true,
		DefaultMaxChars:    100,  // low threshold for testing
		PerTurnBudgetChars: 500,
		PreviewSizeBytes:   50,
	}
}

func TestMaybePersistResult_SmallResultPassesThrough(t *testing.T) {
	store := newTestStore(t)
	cfg := defaultPersistCfg()
	result := tools.NewToolResult("small result")
	budgetUsed := 0

	err := maybePersistResult(store, cfg, "test_tool", "call_1", result, &budgetUsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ForLLM != "small result" {
		t.Errorf("small result should pass through unchanged: got %q", result.ForLLM)
	}
	if budgetUsed != len("small result") {
		t.Errorf("budgetUsed = %d, want %d", budgetUsed, len("small result"))
	}
}

func TestMaybePersistResult_LargeResultPersisted(t *testing.T) {
	store := newTestStore(t)
	cfg := defaultPersistCfg()
	largeContent := strings.Repeat("x", 200) // over 100-char threshold
	result := tools.NewToolResult(largeContent)
	budgetUsed := 0

	err := maybePersistResult(store, cfg, "test_tool", "call_1", result, &budgetUsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.ForLLM, "<persisted-tool-result") {
		t.Error("large result should be replaced with preview message")
	}
	if !strings.Contains(result.ForLLM, "call_1.txt") {
		t.Error("preview should reference the persisted file")
	}
}

func TestMaybePersistResult_ErrorResultSkipped(t *testing.T) {
	store := newTestStore(t)
	cfg := defaultPersistCfg()
	largeError := strings.Repeat("e", 200)
	result := tools.ErrorResult(largeError)
	budgetUsed := 0

	err := maybePersistResult(store, cfg, "test_tool", "call_1", result, &budgetUsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ForLLM != largeError {
		t.Error("error results should never be persisted")
	}
}

func TestMaybePersistResult_EmptyResultSkipped(t *testing.T) {
	store := newTestStore(t)
	cfg := defaultPersistCfg()
	result := tools.NewToolResult("")
	budgetUsed := 0

	err := maybePersistResult(store, cfg, "test_tool", "call_1", result, &budgetUsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ForLLM != "" {
		t.Error("empty result should pass through unchanged")
	}
}

func TestMaybePersistResult_PerToolOverride(t *testing.T) {
	store := newTestStore(t)
	cfg := defaultPersistCfg()
	cfg.ToolOverrides = map[string]int{"big_tool": 300}

	// 200 chars: over default (100) but under override (300)
	content := strings.Repeat("x", 200)
	result := tools.NewToolResult(content)
	budgetUsed := 0

	err := maybePersistResult(store, cfg, "big_tool", "call_1", result, &budgetUsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ForLLM != content {
		t.Error("result under per-tool override threshold should pass through")
	}
}

func TestMaybePersistResult_AggregateBudgetTriggers(t *testing.T) {
	store := newTestStore(t)
	cfg := defaultPersistCfg() // budget = 500

	// First call: 90 chars (under 100 threshold), budget = 90
	result1 := tools.NewToolResult(strings.Repeat("a", 90))
	budgetUsed := 0
	err := maybePersistResult(store, cfg, "test_tool", "call_1", result1, &budgetUsed)
	if err != nil {
		t.Fatalf("call 1 error: %v", err)
	}
	if strings.Contains(result1.ForLLM, "<persisted-tool-result") {
		t.Error("call 1 should pass through")
	}

	// Simulate several more calls pushing budget to 450
	budgetUsed = 450

	// Next call: 90 chars (under individual threshold) but 450+90=540 > 500 budget
	result2 := tools.NewToolResult(strings.Repeat("b", 90))
	err = maybePersistResult(store, cfg, "test_tool", "call_2", result2, &budgetUsed)
	if err != nil {
		t.Fatalf("call 2 error: %v", err)
	}
	if !strings.Contains(result2.ForLLM, "<persisted-tool-result") {
		t.Error("call exceeding aggregate budget should be persisted")
	}
}

func TestMaybePersistResult_BudgetTracksCorrectly(t *testing.T) {
	store := newTestStore(t)
	cfg := defaultPersistCfg()
	budgetUsed := 0

	// Small result: budget should increase by content length
	result1 := tools.NewToolResult("hello")
	maybePersistResult(store, cfg, "test_tool", "call_1", result1, &budgetUsed)
	if budgetUsed != 5 {
		t.Errorf("budget after small result = %d, want 5", budgetUsed)
	}

	// Large result: budget should increase by preview length, not original
	result2 := tools.NewToolResult(strings.Repeat("x", 200))
	maybePersistResult(store, cfg, "test_tool", "call_2", result2, &budgetUsed)
	// Budget should be 5 + len(preview message), not 5 + 200
	if budgetUsed == 205 {
		t.Error("budget should track preview size for persisted results, not original")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agent/ -run TestMaybePersistResult -v -count=1`
Expected: FAIL — maybePersistResult not defined

- [ ] **Step 3: Implement maybePersistResult**

```go
// pkg/agent/result_persist.go
package agent

import (
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools/resultstore"
)

// maybePersistResult checks whether a tool result exceeds the per-tool size
// threshold or the per-turn aggregate budget. If either triggers, the full
// result is persisted to disk and result.ForLLM is replaced with a preview.
// turnBudgetUsed is updated to reflect the size added this call.
func maybePersistResult(
	store *resultstore.ResultStore,
	cfg config.ToolResultPersistenceConfig,
	toolName string,
	toolCallID string,
	result *tools.ToolResult,
	turnBudgetUsed *int,
) error {
	content := result.ForLLM

	// Never persist errors or empty results
	if result.IsError || content == "" {
		*turnBudgetUsed += len(content)
		return nil
	}

	// Determine threshold for this tool
	threshold := cfg.DefaultMaxChars
	if override, ok := cfg.ToolOverrides[toolName]; ok {
		threshold = override
	}

	// Check individual size and aggregate budget
	overIndividual := len(content) > threshold
	overBudget := *turnBudgetUsed+len(content) > cfg.PerTurnBudgetChars

	if overIndividual || overBudget {
		persisted, err := store.Persist(toolCallID, content, cfg.PreviewSizeBytes)
		if err != nil {
			return err
		}
		result.ForLLM = persisted.FormatPreviewMessage()
	}

	*turnBudgetUsed += len(result.ForLLM)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agent/ -run TestMaybePersistResult -v -count=1`
Expected: PASS (7 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/result_persist.go pkg/agent/result_persist_test.go
git commit -m "feat(agent): add maybePersistResult size gate for large tool results"
```

---

### Task 6: Wire into Agent Loop

**Files:**
- Modify: `pkg/agent/loop.go`

- [ ] **Step 1: Add import for resultstore**

Add to the import block in `pkg/agent/loop.go`:

```go
"github.com/dawnforge-lab/spawnbot-v5/pkg/tools/resultstore"
```

- [ ] **Step 2: Initialize ResultStore and budget counter before tool loop**

In `loop.go`, find the tool execution loop. Before the loop that iterates over normalized tool calls (around line 2295 where `for _, tc := range normalizedCalls`), add:

```go
		// Tool result persistence: lazy-init store and budget counter
		var resultStore *resultstore.ResultStore
		turnBudgetUsed := 0
		persistCfg := al.cfg.Tools.ResultPersistence
		if persistCfg.Enabled {
			storeDir := filepath.Join(al.cfg.Agents.Defaults.Workspace, "sessions", ts.sessionKey, "tool-results")
			var storeErr error
			resultStore, storeErr = resultstore.NewResultStore(storeDir)
			if storeErr != nil {
				logger.WarnCF("agent", "Failed to create result store, persistence disabled for this turn",
					map[string]any{"error": storeErr.Error()})
			}
		}
```

- [ ] **Step 3: Call maybePersistResult after tool execution**

Find the line `contentForLLM := toolResult.ContentForLLM()` (around line 2578). Insert the persistence check **before** this line:

```go
			// Persist large results to disk if enabled
			if resultStore != nil {
				if err := maybePersistResult(resultStore, persistCfg, toolName, toolCallID, toolResult, &turnBudgetUsed); err != nil {
					toolResult = tools.ErrorResult(fmt.Sprintf("Failed to persist large tool result: %v", err))
				}
			}
```

- [ ] **Step 4: Verify it compiles**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go build ./pkg/agent/`
Expected: Success (exit 0)

- [ ] **Step 5: Run existing tests to check nothing is broken**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agent/ -count=1 -timeout 120s 2>&1 | tail -5`
Expected: PASS (or only pre-existing failures from embed.go)

- [ ] **Step 6: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): wire tool result persistence into tool execution loop"
```

---

### Task 7: Integration Test

**Files:**
- Create: `pkg/tools/resultstore/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
// pkg/tools/resultstore/integration_test.go
package resultstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_PersistAndReadBack(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "sessions", "test_session", "tool-results")
	store, err := NewResultStore(baseDir)
	if err != nil {
		t.Fatalf("NewResultStore: %v", err)
	}

	// Simulate a large tool result (60KB)
	largeContent := strings.Repeat("This is line number N of the tool output.\n", 1500)

	// Persist it
	result, err := store.Persist("toolu_integration_1", largeContent, 2000)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// Verify preview is small
	if len(result.Preview) > 2000 {
		t.Errorf("preview should be <= 2000 bytes, got %d", len(result.Preview))
	}

	// Verify preview message has expected structure
	msg := result.FormatPreviewMessage()
	if !strings.Contains(msg, "<persisted-tool-result") {
		t.Error("preview message should have XML wrapper")
	}
	if !strings.Contains(msg, "read_file") {
		t.Error("preview message should mention read_file")
	}

	// Verify full content can be read back from disk (simulates read_file)
	readBack, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("reading back persisted file: %v", err)
	}
	if string(readBack) != largeContent {
		t.Error("read-back content should match original")
	}

	// Verify file path is within the expected directory structure
	if !strings.Contains(result.FilePath, "sessions/test_session/tool-results") {
		t.Errorf("file path should be session-scoped: got %s", result.FilePath)
	}
}

func TestIntegration_MultiplePersists(t *testing.T) {
	store, err := NewResultStore(filepath.Join(t.TempDir(), "tool-results"))
	if err != nil {
		t.Fatalf("NewResultStore: %v", err)
	}

	// Persist multiple results
	for i := 0; i < 5; i++ {
		id := strings.Replace("toolu_XXXXX", "XXXXX", strings.Repeat("a", i+1), 1)
		content := strings.Repeat("data\n", 1000*(i+1))
		result, err := store.Persist(id, content, 2000)
		if err != nil {
			t.Fatalf("Persist %d: %v", i, err)
		}
		if result.OrigSize != len(content) {
			t.Errorf("Persist %d: OrigSize = %d, want %d", i, result.OrigSize, len(content))
		}
	}

	// Verify all files exist
	entries, err := os.ReadDir(filepath.Join(t.TempDir(), "tool-results"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 files, got %d", len(entries))
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -run TestIntegration -v`
Expected: PASS (2 tests)

- [ ] **Step 3: Run all resultstore tests together**

Run: `export PATH="/home/eugen-dev/.spawnbot/go/bin:$PATH" && cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/resultstore/ -v`
Expected: PASS (10 tests)

- [ ] **Step 4: Commit**

```bash
git add pkg/tools/resultstore/integration_test.go
git commit -m "test(resultstore): add integration tests for persist-and-read-back flow"
```

---

### Self-Review Checklist

**Spec coverage:**
- [x] ResultStore with disk persistence and preview — Tasks 1-3
- [x] Size gate with per-tool thresholds and aggregate budget — Task 5
- [x] Configuration with defaults — Task 4
- [x] Integration into loop.go — Task 6
- [x] Error handling: disk write fails → error to agent (Task 5 gate returns error, Task 6 converts to ErrorResult), store init fails → log warning + skip (Task 6), error results skipped (Task 5)
- [x] Testing: unit tests (Tasks 1-3, 5), integration test (Task 7)

**Placeholder scan:** No TBD/TODO/placeholders found.

**Type consistency:**
- `ResultStore` / `NewResultStore` — consistent across Tasks 1-3, 5-7
- `PersistedResult` / `FormatPreviewMessage` — consistent across Tasks 2-3, 5
- `maybePersistResult` — consistent signature across Tasks 5-6
- `ToolResultPersistenceConfig` — consistent across Tasks 4-6
- `Persist(toolUseID, content string, previewMaxBytes int)` — consistent across Tasks 2-3, 5, 7
