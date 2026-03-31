package tasks

import (
	"testing"
)

func TestValidateTask_Valid(t *testing.T) {
	task := &Task{
		Title:    "My Task",
		Status:   StatusPending,
		Priority: PriorityHigh,
	}
	if err := task.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateTask_MissingTitle(t *testing.T) {
	task := &Task{}
	err := task.Validate()
	if err == nil {
		t.Fatal("expected error for missing title, got nil")
	}
	if err.Error() != "task title is required" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateTask_InvalidStatus(t *testing.T) {
	task := &Task{
		Title:  "My Task",
		Status: "bogus",
	}
	err := task.Validate()
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
}

func TestValidateTask_InvalidPriority(t *testing.T) {
	task := &Task{
		Title:    "My Task",
		Priority: "urgent",
	}
	err := task.Validate()
	if err == nil {
		t.Fatal("expected error for invalid priority, got nil")
	}
}

func TestValidateTask_EmptyPriorityDefaultsToMedium(t *testing.T) {
	task := &Task{Title: "My Task"}
	task.ApplyDefaults()
	if task.Priority != PriorityMedium {
		t.Fatalf("expected priority %q, got %q", PriorityMedium, task.Priority)
	}
	if task.Status != StatusPending {
		t.Fatalf("expected status %q, got %q", StatusPending, task.Status)
	}
}

func TestIsTerminal(t *testing.T) {
	cases := []struct {
		status   string
		terminal bool
	}{
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusPending, false},
		{StatusInProgress, false},
	}
	for _, c := range cases {
		got := IsTerminal(c.status)
		if got != c.terminal {
			t.Errorf("IsTerminal(%q) = %v, want %v", c.status, got, c.terminal)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	if len(id) != 8 {
		t.Fatalf("expected ID length 8, got %d: %q", len(id), id)
	}

	id2 := GenerateID()
	if id == id2 {
		t.Fatal("expected unique IDs, got identical values")
	}
}
