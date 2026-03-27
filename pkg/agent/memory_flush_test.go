package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractKeyFacts_NothingNew(t *testing.T) {
	// extractKeyFacts returns "" when LLM returns NOTHING_NEW.
	// We can't easily mock the LLM here, so test the validation logic
	// by checking the prompt construction and result filtering.

	result := filterExtractedFacts("NOTHING_NEW")
	if result != "" {
		t.Errorf("expected empty for NOTHING_NEW, got %q", result)
	}
}

func TestExtractKeyFacts_NoBullets(t *testing.T) {
	result := filterExtractedFacts("Some random text without bullets")
	if result != "" {
		t.Errorf("expected empty for non-bullet output, got %q", result)
	}
}

func TestExtractKeyFacts_ValidBullets(t *testing.T) {
	input := "- User prefers dark mode\n- API key stored in vault"
	result := filterExtractedFacts(input)
	if result == "" {
		t.Error("expected non-empty for valid bullet output")
	}
	if !strings.Contains(result, "Memory Flush") {
		t.Error("expected Memory Flush header in output")
	}
	if !strings.Contains(result, "dark mode") {
		t.Error("expected fact content preserved")
	}
}

func TestMemoryStore_AppendToday(t *testing.T) {
	dir := t.TempDir()
	ms := NewMemoryStore(dir)

	if err := ms.AppendToday("- Fact 1"); err != nil {
		t.Fatalf("AppendToday failed: %v", err)
	}

	content := ms.ReadToday()
	if !strings.Contains(content, "Fact 1") {
		t.Errorf("expected 'Fact 1' in daily notes, got %q", content)
	}

	// Append again — should not duplicate.
	if err := ms.AppendToday("- Fact 2"); err != nil {
		t.Fatalf("second AppendToday failed: %v", err)
	}

	content = ms.ReadToday()
	if !strings.Contains(content, "Fact 2") {
		t.Errorf("expected 'Fact 2' in daily notes, got %q", content)
	}
	if strings.Count(content, "Fact 1") != 1 {
		t.Errorf("expected exactly one occurrence of 'Fact 1'")
	}
}

func TestMemoryStore_DailyNotesDir(t *testing.T) {
	dir := t.TempDir()
	ms := NewMemoryStore(dir)

	if err := ms.AppendToday("test content"); err != nil {
		t.Fatalf("AppendToday failed: %v", err)
	}

	// Verify the month subdirectory was created.
	entries, err := os.ReadDir(filepath.Join(dir, "memory"))
	if err != nil {
		t.Fatalf("failed to read memory dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) == 6 { // YYYYMM
			found = true
		}
	}
	if !found {
		t.Error("expected YYYYMM subdirectory in memory dir")
	}
}

func TestPeriodicFlushThreshold(t *testing.T) {
	// Test the checkpoint logic used in maybePeriodicFlush.
	tests := []struct {
		count    int
		shouldFl bool
	}{
		{14, false},
		{15, true},
		{16, false},
		{29, false},
		{30, true},
		{45, true},
	}

	for _, tt := range tests {
		prevCheckpoint := (tt.count - 1) / PeriodicFlushInterval * PeriodicFlushInterval
		currCheckpoint := tt.count / PeriodicFlushInterval * PeriodicFlushInterval
		triggered := currCheckpoint > prevCheckpoint && currCheckpoint > 0
		if triggered != tt.shouldFl {
			t.Errorf("count=%d: expected flush=%v, got %v", tt.count, tt.shouldFl, triggered)
		}
	}
}

// filterExtractedFacts replicates the validation logic from extractKeyFacts
// for unit testing without an LLM dependency.
func filterExtractedFacts(result string) string {
	result = strings.TrimSpace(result)

	if result == "NOTHING_NEW" {
		return ""
	}

	if !strings.Contains(result, "- ") {
		return ""
	}

	return "\n## Memory Flush\n\n" + result
}
