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
	return truncated
}
