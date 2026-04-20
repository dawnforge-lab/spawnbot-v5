package tools

import (
	"context"
	"strings"
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

func TestFormatResult_WithTasks(t *testing.T) {
	tool := NewCouncilTool(nil)
	result := &CouncilRunResult{
		ID:        "abc",
		Title:     "Planning",
		Rounds:    2,
		Synthesis: "The team agreed on approach B.",
		Tasks: []CouncilTask{
			{Agent: "researcher", Task: "validate the hypothesis", Priority: "high"},
			{Agent: "main", Task: "write the plan"},
		},
		Status: "closed",
	}

	tr := tool.formatResult(result)

	if !strings.Contains(tr.ForUser, "1. [researcher] validate the hypothesis  (high)") {
		t.Errorf("ForUser missing task 1: %s", tr.ForUser)
	}
	if !strings.Contains(tr.ForUser, "2. [main] write the plan  (normal)") {
		t.Errorf("ForUser missing task 2 with default priority: %s", tr.ForUser)
	}
	if !strings.Contains(tr.ForLLM, `"tasks"`) {
		t.Errorf("ForLLM missing tasks field: %s", tr.ForLLM)
	}
}

func TestFormatResult_NoTasks(t *testing.T) {
	tool := NewCouncilTool(nil)
	result := &CouncilRunResult{
		ID:        "xyz",
		Title:     "Discussion",
		Rounds:    1,
		Synthesis: "No clear actions emerged.",
		Status:    "closed",
	}

	tr := tool.formatResult(result)

	if strings.Contains(tr.ForUser, "Tasks:") {
		t.Errorf("ForUser should not contain Tasks section when there are none: %s", tr.ForUser)
	}
}
