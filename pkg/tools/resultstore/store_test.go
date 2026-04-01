package resultstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePreview_TruncatesAtNewline(t *testing.T) {
	line := strings.Repeat("a", 1000)
	content := line + "\n" + line + "\n" + line + "\n"

	preview := generatePreview(content, 2000)

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
	content := strings.Repeat("x", 3000)
	preview := generatePreview(content, 2000)
	if len(preview) != 2000 {
		t.Errorf("no-newline content should hard truncate: got %d bytes, want 2000", len(preview))
	}
}

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

	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("should be able to read persisted file: %v", err)
	}
	if string(data) != content {
		t.Error("persisted content should match original")
	}

	if len(result.Preview) > 2000 {
		t.Errorf("preview too long: %d bytes", len(result.Preview))
	}
	if result.Preview == "" {
		t.Error("preview should not be empty")
	}

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
