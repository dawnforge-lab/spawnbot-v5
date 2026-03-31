package struggles

import (
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
