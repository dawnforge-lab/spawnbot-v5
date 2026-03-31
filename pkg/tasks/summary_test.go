package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) (*TaskStore, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	return NewTaskStore(path), path
}

func TestSummary_Empty(t *testing.T) {
	s, _ := newTestStore(t)
	got := s.Summary(10)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSummary_FullList(t *testing.T) {
	s, _ := newTestStore(t)
	s.Create("Task Alpha", "", PriorityHigh)
	s.Create("Task Beta", "", PriorityLow)

	got := s.Summary(10)
	if !strings.Contains(got, "Active tasks:") {
		t.Errorf("expected 'Active tasks:' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Task Alpha") {
		t.Errorf("expected 'Task Alpha' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Task Beta") {
		t.Errorf("expected 'Task Beta' in output, got:\n%s", got)
	}
}

func TestSummary_SwitchesToCount(t *testing.T) {
	s, _ := newTestStore(t)
	for i := 0; i < 12; i++ {
		s.Create("Task", "", PriorityMedium)
	}

	got := s.Summary(10)
	if !strings.Contains(got, "You have 12 tasks") {
		t.Errorf("expected 'You have 12 tasks' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Top 5") {
		t.Errorf("expected 'Top 5' in output, got:\n%s", got)
	}
}

func TestSummary_SortOrder(t *testing.T) {
	s, _ := newTestStore(t)

	lowPending, _ := s.Create("Low Pending", "", PriorityLow)
	highPending, _ := s.Create("High Pending", "", PriorityHigh)
	medInProgress, _ := s.Create("Med InProgress", "", PriorityMedium)
	s.Update(medInProgress.ID, map[string]string{"status": StatusInProgress})

	got := s.Summary(10)
	lines := strings.Split(got, "\n")

	// Find line indices for each task
	idxMed, idxHigh, idxLow := -1, -1, -1
	for i, line := range lines {
		if strings.Contains(line, "Med InProgress") {
			idxMed = i
		}
		if strings.Contains(line, "High Pending") {
			idxHigh = i
		}
		if strings.Contains(line, "Low Pending") {
			idxLow = i
		}
	}

	_ = lowPending
	_ = highPending

	if idxMed == -1 || idxHigh == -1 || idxLow == -1 {
		t.Fatalf("could not find all tasks in output:\n%s", got)
	}
	if idxMed >= idxHigh {
		t.Errorf("expected in_progress (idx %d) before high pending (idx %d)", idxMed, idxHigh)
	}
	if idxHigh >= idxLow {
		t.Errorf("expected high pending (idx %d) before low pending (idx %d)", idxHigh, idxLow)
	}
}

func TestPendingSummary_ExcludesTerminal(t *testing.T) {
	s, _ := newTestStore(t)
	pending, _ := s.Create("Pending Task", "", PriorityMedium)
	completed, _ := s.Create("Completed Task", "", PriorityMedium)
	s.Update(completed.ID, map[string]string{"status": StatusCompleted})

	_ = pending

	got := s.PendingSummary()
	if !strings.Contains(got, "Pending Task") {
		t.Errorf("expected 'Pending Task' in output, got:\n%s", got)
	}
	if strings.Contains(got, "Completed Task") {
		t.Errorf("expected 'Completed Task' to be excluded, got:\n%s", got)
	}
}

func TestPendingSummary_Empty(t *testing.T) {
	s, _ := newTestStore(t)
	got := s.PendingSummary()
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSummary_WithWarning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	// Write corrupted JSON
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewTaskStore(path)
	got := s.Summary(10)

	if !strings.Contains(got, "WARNING:") {
		t.Errorf("expected WARNING in summary, got:\n%s", got)
	}
	if !strings.Contains(got, path) {
		t.Errorf("expected file path in warning, got:\n%s", got)
	}
}
