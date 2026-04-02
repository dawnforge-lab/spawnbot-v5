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
	storeDir := filepath.Join(t.TempDir(), "tool-results")
	store, err := NewResultStore(storeDir)
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
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 files, got %d", len(entries))
	}
}
