package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEndToEnd_FullLifecycle(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	task1, _ := store.Create("Research auth libraries", "", PriorityHigh)
	task2, _ := store.Create("Write unit tests", "", PriorityMedium)
	task3, _ := store.Create("Update docs", "README needs new section", PriorityLow)

	all := store.List("")
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(all))
	}

	store.Update(task1.ID, map[string]string{"status": StatusInProgress, "agent_type": "researcher"})
	store.Complete(task2.ID, "All 15 tests passing")
	store.Fail(task3.ID, "README is auto-generated, can't edit")

	got1 := store.Get(task1.ID)
	if got1.Status != StatusInProgress || got1.AgentType != "researcher" {
		t.Fatalf("task1: status=%s agent=%s", got1.Status, got1.AgentType)
	}
	got2 := store.Get(task2.ID)
	if got2.Status != StatusCompleted || got2.Result != "All 15 tests passing" {
		t.Fatalf("task2: status=%s result=%s", got2.Status, got2.Result)
	}
	got3 := store.Get(task3.ID)
	if got3.Status != StatusFailed {
		t.Fatalf("task3: status=%s", got3.Status)
	}
}

func TestEndToEnd_PersistenceAcrossReloads(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	store1 := NewTaskStore(fp)
	task, _ := store1.Create("Survive restart", "", PriorityHigh)
	store1.Update(task.ID, map[string]string{"status": StatusInProgress})

	store2 := NewTaskStore(fp)
	got := store2.Get(task.ID)
	if got == nil {
		t.Fatal("task not found after reload")
	}
	if got.Status != StatusInProgress {
		t.Fatalf("status not persisted, got %s", got.Status)
	}
}

func TestEndToEnd_TTLCleansCompletedOnly(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	store := NewTaskStore(fp)
	active, _ := store.Create("Still working", "", PriorityHigh)
	old, _ := store.Create("Old done", "", PriorityLow)
	store.Complete(old.ID, "done long ago")

	store.mu.Lock()
	store.tasks[old.ID].UpdatedAt = time.Now().Add(-8 * 24 * time.Hour).UnixMilli()
	store.mu.Unlock()
	store.Save()

	store2 := NewTaskStore(fp)
	if store2.Get(active.ID) == nil {
		t.Fatal("active task should survive")
	}
	if store2.Get(old.ID) != nil {
		t.Fatal("old completed task should be cleaned")
	}
}

func TestEndToEnd_SummaryFormats(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	store.Create("Task A", "", PriorityHigh)
	store.Create("Task B", "", PriorityMedium)

	s := store.Summary(10)
	if !strings.Contains(s, "Active tasks:") {
		t.Fatalf("expected full list: %s", s)
	}

	for i := 0; i < 10; i++ {
		store.Create("Bulk task", "", PriorityLow)
	}

	s = store.Summary(10)
	if !strings.Contains(s, "You have 12 tasks") {
		t.Fatalf("expected count format: %s", s)
	}
}

func TestEndToEnd_HeartbeatSummary(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	store.Create("Pending work", "", PriorityHigh)
	completed, _ := store.Create("Done work", "", PriorityMedium)
	store.Complete(completed.ID, "done")

	pending := store.PendingSummary()
	if !strings.Contains(pending, "Pending work") {
		t.Fatalf("should include pending: %s", pending)
	}
	if strings.Contains(pending, "Done work") {
		t.Fatalf("should not include completed: %s", pending)
	}
}

func TestEndToEnd_CorruptedFileRecovery(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	os.WriteFile(fp, []byte("{invalid"), 0644)

	store := NewTaskStore(fp)
	task, err := store.Create("New task after corruption", "", PriorityMedium)
	if err != nil {
		t.Fatalf("should create after corruption: %v", err)
	}

	s := store.Summary(10)
	if !strings.Contains(s, "WARNING") {
		t.Fatalf("expected warning: %s", s)
	}
	if store.Get(task.ID) == nil {
		t.Fatal("task should exist")
	}
}
