package struggles

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSignal_JSON(t *testing.T) {
	s := Signal{
		Timestamp: time.Date(2026, 3, 31, 3, 0, 0, 0, time.UTC),
		Type:      TypeToolError,
		Tool:      "exec",
		Error:     "command not found: jq",
		Session:   "telegram_123",
		Context:   "user asked to parse JSON",
	}

	if s.Type != TypeToolError {
		t.Errorf("expected type %q, got %q", TypeToolError, s.Type)
	}
	if s.Tool != "exec" {
		t.Errorf("expected tool %q, got %q", "exec", s.Tool)
	}
}

func TestSignalTypes(t *testing.T) {
	types := []string{TypeToolError, TypeUserCorrection, TypeRepeatedTool}
	for _, typ := range types {
		if typ == "" {
			t.Error("signal type constant is empty")
		}
	}
}

func TestCollector_OnToolResult_Error(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.OnToolResult("exec", map[string]any{"command": "jq .foo"}, true, "command not found: jq", "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Type != TypeToolError {
		t.Errorf("expected type %q, got %q", TypeToolError, signals[0].Type)
	}
	if signals[0].Tool != "exec" {
		t.Errorf("expected tool %q, got %q", "exec", signals[0].Tool)
	}
}

func TestCollector_OnToolResult_Success(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.OnToolResult("read_file", map[string]any{}, false, "", "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for success, got %d", len(signals))
	}
}

func TestCollector_OnUserMessage_Correction(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.OnUserMessage("no that's wrong, I said JSON not YAML", "I'll convert it to YAML", "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Type != TypeUserCorrection {
		t.Errorf("expected type %q, got %q", TypeUserCorrection, signals[0].Type)
	}
}

func TestCollector_OnUserMessage_Normal(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.OnUserMessage("Can you read the config file?", "Sure", "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for normal message, got %d", len(signals))
	}
}

func TestCollector_OnTurnEnd_RepeatedTool(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	counts := map[string]int{"exec": 5, "read_file": 1}
	c.OnTurnEnd(counts, "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal for repeated tool, got %d", len(signals))
	}
	if signals[0].Type != TypeRepeatedTool {
		t.Errorf("expected type %q, got %q", TypeRepeatedTool, signals[0].Type)
	}
	if signals[0].Tool != "exec" {
		t.Errorf("expected tool %q, got %q", "exec", signals[0].Tool)
	}
	if signals[0].Count != 5 {
		t.Errorf("expected count 5, got %d", signals[0].Count)
	}
}

func TestCollector_OnTurnEnd_NoneRepeated(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	counts := map[string]int{"exec": 2, "read_file": 1}
	c.OnTurnEnd(counts, "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals when no tool called 3+ times, got %d", len(signals))
	}
}
