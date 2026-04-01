package resultstore

import (
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
