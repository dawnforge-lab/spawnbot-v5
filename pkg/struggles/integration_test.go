package struggles

import (
	"path/filepath"
	"testing"
)

func TestCollector_MultipleSignals_Integration(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	c := NewCollector(logPath)

	// Tool error
	c.HandleToolEnd("exec", true, "not found", "s1")

	// User correction
	c.HandleTurnStart("no that's wrong", "s1")

	// Repeated tool — simulate 4 calls then end turn
	c.HandleTurnStart("do it", "s1")
	for i := 0; i < 4; i++ {
		c.HandleToolEnd("exec", false, "", "s1")
	}
	c.HandleTurnEnd("s1")

	signals, err := ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d", len(signals))
	}

	types := map[string]bool{}
	for _, s := range signals {
		types[s.Type] = true
	}
	if !types[TypeToolError] {
		t.Error("missing tool_error signal")
	}
	if !types[TypeUserCorrection] {
		t.Error("missing user_correction signal")
	}
	if !types[TypeRepeatedTool] {
		t.Error("missing repeated_tool signal")
	}

	// Truncate and verify
	if err := TruncateLog(logPath); err != nil {
		t.Fatalf("TruncateLog: %v", err)
	}
	signals, _ = ReadLog(logPath)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals after truncate, got %d", len(signals))
	}
}
