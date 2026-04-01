package resultstore

import (
	"fmt"
	"os"
	"path/filepath"
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
	return truncated
}

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
