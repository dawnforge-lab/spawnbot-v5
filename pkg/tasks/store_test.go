package tasks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "tasks.json")
}

func TestStore_CreateAndGet(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	task, err := s.Create("Test task", "desc", "high")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if task.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, task.Status)
	}
	if task.Priority != "high" {
		t.Errorf("expected priority %q, got %q", "high", task.Priority)
	}
	got := s.Get(task.ID)
	if got == nil {
		t.Fatal("Get returned nil for existing task")
	}
	if got.ID != task.ID {
		t.Errorf("expected ID %q, got %q", task.ID, got.ID)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	got := s.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestStore_List(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	_, err := s.Create("Task 1", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	_, err = s.Create("Task 2", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	all := s.List("")
	if len(all) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(all))
	}
}

func TestStore_ListFilterByStatus(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	t1, err := s.Create("Task 1", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	_, err = s.Create("Task 2", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := s.Complete(t1.ID, "done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	pending := s.List(StatusPending)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(pending))
	}
	completed := s.List(StatusCompleted)
	if len(completed) != 1 {
		t.Errorf("expected 1 completed task, got %d", len(completed))
	}
}

func TestStore_Update(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	task, err := s.Create("Task", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = s.Update(task.ID, map[string]string{
		"status":     StatusInProgress,
		"agent_type": "coder",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got := s.Get(task.ID)
	if got.Status != StatusInProgress {
		t.Errorf("expected status %q, got %q", StatusInProgress, got.Status)
	}
	if got.AgentType != "coder" {
		t.Errorf("expected agent_type %q, got %q", "coder", got.AgentType)
	}
}

func TestStore_UpdateRejectsTerminal(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	task, err := s.Create("Task", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := s.Complete(task.ID, "done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	err = s.Update(task.ID, map[string]string{"status": StatusPending})
	if err == nil {
		t.Fatal("expected error updating terminal task, got nil")
	}
}

func TestStore_Complete(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	task, err := s.Create("Task", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := s.Complete(task.ID, "all done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	got := s.Get(task.ID)
	if got.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, got.Status)
	}
	if got.Result != "all done" {
		t.Errorf("expected result %q, got %q", "all done", got.Result)
	}
}

func TestStore_Fail(t *testing.T) {
	s := NewTaskStore(tempFile(t))
	task, err := s.Create("Task", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := s.Fail(task.ID, "something went wrong"); err != nil {
		t.Fatalf("Fail failed: %v", err)
	}
	got := s.Get(task.ID)
	if got.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, got.Status)
	}
	if got.Result != "something went wrong" {
		t.Errorf("expected result %q, got %q", "something went wrong", got.Result)
	}
}

func TestStore_Persistence(t *testing.T) {
	path := tempFile(t)
	s1 := NewTaskStore(path)
	task, err := s1.Create("Persistent task", "desc", "low")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	s2 := NewTaskStore(path)
	got := s2.Get(task.ID)
	if got == nil {
		t.Fatal("task not found in second store loaded from same file")
	}
	if got.Title != "Persistent task" {
		t.Errorf("expected title %q, got %q", "Persistent task", got.Title)
	}
}

func TestStore_TTLCleanup(t *testing.T) {
	path := tempFile(t)
	s1 := NewTaskStore(path)
	task, err := s1.Create("Old task", "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := s1.Complete(task.ID, "done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Backdate UpdatedAt to 8 days ago so it is past the 7-day TTL.
	s1.mu.Lock()
	s1.tasks[task.ID].UpdatedAt = time.Now().Add(-8 * 24 * time.Hour).UnixMilli()
	s1.mu.Unlock()
	if err := s1.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	s2 := NewTaskStore(path)
	got := s2.Get(task.ID)
	if got != nil {
		t.Errorf("expected expired task to be cleaned up on load, but it still exists: %+v", got)
	}
}

func TestStore_CorruptedFile(t *testing.T) {
	path := tempFile(t)
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	s := NewTaskStore(path)
	if s.Warning() == "" {
		t.Error("expected non-empty Warning() for corrupted file")
	}
	all := s.List("")
	if len(all) != 0 {
		t.Errorf("expected empty list for corrupted file, got %d tasks", len(all))
	}
}
