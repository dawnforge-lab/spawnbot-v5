package agent

import (
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
		DefaultMaxChars:    100,
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
	largeContent := strings.Repeat("x", 200)
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

	result1 := tools.NewToolResult(strings.Repeat("a", 90))
	budgetUsed := 0
	err := maybePersistResult(store, cfg, "test_tool", "call_1", result1, &budgetUsed)
	if err != nil {
		t.Fatalf("call 1 error: %v", err)
	}
	if strings.Contains(result1.ForLLM, "<persisted-tool-result") {
		t.Error("call 1 should pass through")
	}

	budgetUsed = 450

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

	result1 := tools.NewToolResult("hello")
	maybePersistResult(store, cfg, "test_tool", "call_1", result1, &budgetUsed)
	if budgetUsed != 5 {
		t.Errorf("budget after small result = %d, want 5", budgetUsed)
	}

	result2 := tools.NewToolResult(strings.Repeat("x", 200))
	maybePersistResult(store, cfg, "test_tool", "call_2", result2, &budgetUsed)
	if budgetUsed == 205 {
		t.Error("budget should track preview size for persisted results, not original")
	}
}
