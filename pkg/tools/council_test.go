package tools

import (
	"context"
	"testing"
)

// mockCouncilLister implements CouncilLister for tests.
type mockCouncilLister struct {
	metas []*CouncilMetaSummary
	err   error
}

func (m *mockCouncilLister) List() ([]*CouncilMetaSummary, error) {
	return m.metas, m.err
}

func newTestCouncilTool() *CouncilTool {
	ct := &CouncilTool{}
	ct.SetLister(&mockCouncilLister{})
	return ct
}

func TestCouncilTool_Name(t *testing.T) {
	ct := newTestCouncilTool()
	if ct.Name() != "council" {
		t.Fatalf("expected name %q, got %q", "council", ct.Name())
	}
}

func TestCouncilTool_ListEmpty(t *testing.T) {
	ct := newTestCouncilTool()
	result := ct.Execute(context.Background(), map[string]any{"action": "list"})
	if result.IsError {
		t.Fatalf("expected no error, got: %s", result.ForLLM)
	}
	if result.ForUser != "No councils found." {
		t.Fatalf("expected 'No councils found.', got: %s", result.ForUser)
	}
}

func TestCouncilTool_StartMissingTitle(t *testing.T) {
	ct := newTestCouncilTool()
	// No engine set, so we get engine-not-configured error first
	result := ct.Execute(context.Background(), map[string]any{"action": "start"})
	if !result.IsError {
		t.Fatal("expected error for start without engine")
	}
	if result.ForLLM != "council engine not configured" {
		t.Fatalf("expected engine not configured error, got: %s", result.ForLLM)
	}
}

func TestCouncilTool_ResumeMissingID(t *testing.T) {
	ct := newTestCouncilTool()
	// No engine set, so we get engine-not-configured error first
	result := ct.Execute(context.Background(), map[string]any{"action": "resume"})
	if !result.IsError {
		t.Fatal("expected error for resume without engine")
	}
	if result.ForLLM != "council engine not configured" {
		t.Fatalf("expected engine not configured error, got: %s", result.ForLLM)
	}
}
