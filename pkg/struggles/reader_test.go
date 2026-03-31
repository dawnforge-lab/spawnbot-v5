package struggles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLog_Empty(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog on nonexistent file: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals, got %d", len(signals))
	}
}

func TestReadLog_WithSignals(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.OnToolResult("exec", map[string]any{"command": "jq"}, true, "not found", "s1")
	c.OnToolResult("exec", map[string]any{"command": "jq"}, true, "not found", "s2")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(signals))
	}
}

func TestReadLogCapped(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")

	// Write more than maxLogBytes of data
	line := `{"ts":"2026-03-31T03:00:00Z","type":"tool_error","tool":"exec","error":"fail","session":"s1"}` + "\n"
	data := strings.Repeat(line, 2000) // ~190KB > 100KB cap
	os.WriteFile(logPath, []byte(data), 0o644)

	signals, err := ReadLogCapped(logPath, 100*1024)
	if err != nil {
		t.Fatalf("ReadLogCapped: %v", err)
	}
	// Should have fewer than 2000 signals due to cap
	if len(signals) >= 2000 {
		t.Errorf("expected capped signals, got %d", len(signals))
	}
	if len(signals) == 0 {
		t.Error("expected some signals, got 0")
	}
}

func TestTruncateLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.OnToolResult("exec", map[string]any{}, true, "fail", "s1")

	signals, _ := ReadLog(logPath)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal before truncate, got %d", len(signals))
	}

	err := TruncateLog(logPath)
	if err != nil {
		t.Fatalf("TruncateLog: %v", err)
	}

	signals, _ = ReadLog(logPath)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals after truncate, got %d", len(signals))
	}
}

func TestReadLogContent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.OnToolResult("exec", map[string]any{}, true, "not found", "s1")

	content, err := ReadLogContent(logPath, 100*1024)
	if err != nil {
		t.Fatalf("ReadLogContent: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty content")
	}
	if !strings.Contains(content, "tool_error") {
		t.Error("expected content to contain 'tool_error'")
	}
}

func TestReadLogContent_NonExistent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nonexistent.jsonl")

	content, err := ReadLogContent(logPath, 100*1024)
	if err != nil {
		t.Fatalf("ReadLogContent on nonexistent: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content for nonexistent file, got %q", content)
	}
}
