package struggles

import (
	"path/filepath"
	"testing"
)

func TestCollector_ToolError(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleToolEnd("exec", true, "command not found: jq", "telegram_123")

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

func TestCollector_ToolSuccess_NoSignal(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleToolEnd("read_file", false, "", "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for success, got %d", len(signals))
	}
}

func TestCollector_UserCorrection(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleTurnStart("no that's wrong, I said JSON not YAML", "telegram_123")

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

func TestCollector_NormalMessage_NoSignal(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleTurnStart("Can you read the config file?", "telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for normal message, got %d", len(signals))
	}
}

func TestCollector_RepeatedTool(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleTurnStart("do something", "telegram_123")
	for i := 0; i < 5; i++ {
		c.HandleToolEnd("exec", false, "", "telegram_123")
	}
	c.HandleToolEnd("read_file", false, "", "telegram_123")
	c.HandleTurnEnd("telegram_123")

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

func TestCollector_RepeatedTool_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleTurnStart("do something", "telegram_123")
	c.HandleToolEnd("exec", false, "", "telegram_123")
	c.HandleToolEnd("exec", false, "", "telegram_123")
	c.HandleTurnEnd("telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals when below threshold, got %d", len(signals))
	}
}

func TestCollector_TurnReset(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleTurnStart("turn 1", "telegram_123")
	c.HandleToolEnd("exec", false, "", "telegram_123")
	c.HandleToolEnd("exec", false, "", "telegram_123")
	c.HandleTurnEnd("telegram_123")

	c.HandleTurnStart("turn 2", "telegram_123")
	c.HandleToolEnd("exec", false, "", "telegram_123")
	c.HandleToolEnd("exec", false, "", "telegram_123")
	c.HandleTurnEnd("telegram_123")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals (counters should reset between turns), got %d", len(signals))
	}
}

func TestCollector_Integration(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	c.HandleToolEnd("exec", true, "command not found", "session1")
	c.HandleTurnStart("no, wrong", "session1")
	c.HandleTurnStart("do it", "session1")
	for i := 0; i < 4; i++ {
		c.HandleToolEnd("write_file", false, "", "session1")
	}
	c.HandleTurnEnd("session1")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 3 {
		t.Fatalf("expected 3 signals (error + correction + repeated), got %d", len(signals))
	}
	if signals[0].Type != TypeToolError {
		t.Errorf("signal 0: expected %q, got %q", TypeToolError, signals[0].Type)
	}
	if signals[1].Type != TypeUserCorrection {
		t.Errorf("signal 1: expected %q, got %q", TypeUserCorrection, signals[1].Type)
	}
	if signals[2].Type != TypeRepeatedTool {
		t.Errorf("signal 2: expected %q, got %q", TypeRepeatedTool, signals[2].Type)
	}
}
