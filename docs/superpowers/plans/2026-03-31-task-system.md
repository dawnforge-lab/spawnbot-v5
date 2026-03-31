# Task System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add persistent task tracking so Spawnbot can create, track, and follow up on tasks across sessions and heartbeats.

**Architecture:** New `pkg/tasks/` package with Task struct, JSON-file-backed TaskStore, and summary generators. A single `tasks` tool (matching the cron tool pattern) provides CRUD. Task summaries are injected into the system prompt and heartbeat prompt.

**Tech Stack:** Go, JSON file persistence, `encoding/json`, `crypto/rand` for ID generation

---

## File Structure

```
pkg/tasks/                          # NEW PACKAGE
  task.go                           # Task struct, validation, status/priority constants
  task_test.go                      # Tests for validation
  store.go                          # TaskStore — CRUD, JSON persistence, TTL cleanup
  store_test.go                     # Tests for store operations
  summary.go                        # Summary + PendingSummary for prompt injection
  summary_test.go                   # Tests for summary formatting

pkg/tools/
  tasks_tool.go                     # NEW: tasks tool with action parameter
  tasks_tool_test.go                # Tests for tasks tool

pkg/agent/
  context.go                        # MODIFY: inject task summary into system prompt
  loop.go                           # MODIFY: init task store, register tasks tool

pkg/heartbeat/
  service.go                        # MODIFY: inject pending tasks into heartbeat prompt
```

---

### Task 1: Task Struct + Validation

**Files:**
- Create: `pkg/tasks/task.go`
- Create: `pkg/tasks/task_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/tasks/task_test.go`:

```go
package tasks

import (
	"testing"
)

func TestValidateTask_Valid(t *testing.T) {
	task := &Task{
		ID:       "a1b2c3d4",
		Title:    "Fix the auth bug",
		Status:   StatusPending,
		Priority: PriorityMedium,
	}
	if err := task.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateTask_MissingTitle(t *testing.T) {
	task := &Task{
		ID:     "a1b2c3d4",
		Status: StatusPending,
	}
	if err := task.Validate(); err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestValidateTask_InvalidStatus(t *testing.T) {
	task := &Task{
		ID:     "a1b2c3d4",
		Title:  "Test",
		Status: "done",
	}
	if err := task.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestValidateTask_InvalidPriority(t *testing.T) {
	task := &Task{
		ID:       "a1b2c3d4",
		Title:    "Test",
		Status:   StatusPending,
		Priority: "urgent",
	}
	if err := task.Validate(); err == nil {
		t.Fatal("expected error for invalid priority")
	}
}

func TestValidateTask_EmptyPriorityDefaultsToMedium(t *testing.T) {
	task := &Task{
		ID:     "a1b2c3d4",
		Title:  "Test",
		Status: StatusPending,
	}
	task.ApplyDefaults()
	if task.Priority != PriorityMedium {
		t.Fatalf("expected medium priority default, got %s", task.Priority)
	}
}

func TestIsTerminal(t *testing.T) {
	if !IsTerminal(StatusCompleted) {
		t.Fatal("completed should be terminal")
	}
	if !IsTerminal(StatusFailed) {
		t.Fatal("failed should be terminal")
	}
	if IsTerminal(StatusPending) {
		t.Fatal("pending should not be terminal")
	}
	if IsTerminal(StatusInProgress) {
		t.Fatal("in_progress should not be terminal")
	}
}

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	if len(id) != 8 {
		t.Fatalf("expected 8-char ID, got %d: %s", len(id), id)
	}
	id2 := GenerateID()
	if id == id2 {
		t.Fatal("IDs should be unique")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -count=1`
Expected: Compilation error — package doesn't exist

- [ ] **Step 3: Implement Task struct**

Create `pkg/tasks/task.go`:

```go
package tasks

import (
	"crypto/rand"
	"fmt"
	"time"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"

	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"

	ttlDays = 7
)

var validStatuses = map[string]bool{
	StatusPending: true, StatusInProgress: true,
	StatusCompleted: true, StatusFailed: true,
}

var validPriorities = map[string]bool{
	PriorityLow: true, PriorityMedium: true, PriorityHigh: true,
}

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	AgentType   string `json:"agent_type,omitempty"`
	Result      string `json:"result,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (t *Task) Validate() error {
	if t.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if t.Status != "" && !validStatuses[t.Status] {
		return fmt.Errorf("invalid status %q. Valid: pending, in_progress, completed, failed", t.Status)
	}
	if t.Priority != "" && !validPriorities[t.Priority] {
		return fmt.Errorf("invalid priority %q. Valid: low, medium, high", t.Priority)
	}
	return nil
}

func (t *Task) ApplyDefaults() {
	if t.Priority == "" {
		t.Priority = PriorityMedium
	}
	if t.Status == "" {
		t.Status = StatusPending
	}
	now := time.Now().UnixMilli()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	if t.UpdatedAt == 0 {
		t.UpdatedAt = now
	}
}

func IsTerminal(status string) bool {
	return status == StatusCompleted || status == StatusFailed
}

func IsExpired(task *Task) bool {
	if !IsTerminal(task.Status) {
		return false
	}
	cutoff := time.Now().Add(-ttlDays * 24 * time.Hour).UnixMilli()
	return task.UpdatedAt < cutoff
}

func GenerateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/tasks/task.go pkg/tasks/task_test.go
git commit -m "feat(tasks): add Task struct with validation, status constants, and ID generation"
```

---

### Task 2: TaskStore — CRUD + JSON Persistence

**Files:**
- Create: `pkg/tasks/store.go`
- Create: `pkg/tasks/store_test.go`

- [ ] **Step 1: Write failing tests**

Create `pkg/tasks/store_test.go`:

```go
package tasks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	task, err := store.Create("Fix auth bug", "Token expiry issue", PriorityHigh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if task.Status != StatusPending {
		t.Fatalf("expected pending, got %s", task.Status)
	}
	if task.Priority != PriorityHigh {
		t.Fatalf("expected high, got %s", task.Priority)
	}

	got := store.Get(task.ID)
	if got == nil {
		t.Fatal("expected to find task")
	}
	if got.Title != "Fix auth bug" {
		t.Fatalf("wrong title: %s", got.Title)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))
	if store.Get("nonexistent") != nil {
		t.Fatal("expected nil")
	}
}

func TestStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	store.Create("Task A", "", PriorityHigh)
	store.Create("Task B", "", PriorityLow)

	all := store.List("")
	if len(all) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(all))
	}
}

func TestStore_ListFilterByStatus(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	task, _ := store.Create("Task A", "", PriorityHigh)
	store.Create("Task B", "", PriorityLow)
	store.Complete(task.ID, "done")

	pending := store.List(StatusPending)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	completed := store.List(StatusCompleted)
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed, got %d", len(completed))
	}
}

func TestStore_Update(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	task, _ := store.Create("Task", "", PriorityMedium)
	err := store.Update(task.ID, map[string]string{
		"status":    StatusInProgress,
		"agent_type": "researcher",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := store.Get(task.ID)
	if got.Status != StatusInProgress {
		t.Fatalf("expected in_progress, got %s", got.Status)
	}
	if got.AgentType != "researcher" {
		t.Fatalf("expected researcher, got %s", got.AgentType)
	}
}

func TestStore_UpdateRejectsTerminal(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	task, _ := store.Create("Task", "", PriorityMedium)
	store.Complete(task.ID, "done")

	err := store.Update(task.ID, map[string]string{"status": StatusInProgress})
	if err == nil {
		t.Fatal("expected error updating completed task")
	}
}

func TestStore_Complete(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	task, _ := store.Create("Task", "", PriorityMedium)
	err := store.Complete(task.ID, "all done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := store.Get(task.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	if got.Result != "all done" {
		t.Fatalf("expected result, got %s", got.Result)
	}
}

func TestStore_Fail(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	task, _ := store.Create("Task", "", PriorityMedium)
	err := store.Fail(task.ID, "crashed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := store.Get(task.ID)
	if got.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", got.Status)
	}
}

func TestStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	store1 := NewTaskStore(fp)
	store1.Create("Persistent task", "", PriorityHigh)

	store2 := NewTaskStore(fp)
	all := store2.List("")
	if len(all) != 1 {
		t.Fatalf("expected 1 task after reload, got %d", len(all))
	}
	if all[0].Title != "Persistent task" {
		t.Fatalf("wrong title after reload: %s", all[0].Title)
	}
}

func TestStore_TTLCleanup(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	store := NewTaskStore(fp)
	task, _ := store.Create("Old task", "", PriorityMedium)
	store.Complete(task.ID, "done")

	// Manually backdate the task
	store.mu.Lock()
	store.tasks[task.ID].UpdatedAt = time.Now().Add(-8 * 24 * time.Hour).UnixMilli()
	store.mu.Unlock()
	store.Save()

	// Reload — should clean up expired task
	store2 := NewTaskStore(fp)
	if store2.Get(task.ID) != nil {
		t.Fatal("expected expired task to be cleaned up")
	}
}

func TestStore_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	os.WriteFile(fp, []byte("not json"), 0644)

	store := NewTaskStore(fp)
	if store.Warning() == "" {
		t.Fatal("expected warning for corrupted file")
	}

	// Should still work with empty store
	all := store.List("")
	if len(all) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(all))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -count=1`
Expected: Compilation error — NewTaskStore not defined

- [ ] **Step 3: Implement TaskStore**

Create `pkg/tasks/store.go`:

```go
package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

type TaskStore struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	filePath string
	warning  string
}

type taskFile struct {
	Tasks []*Task `json:"tasks"`
}

func NewTaskStore(filePath string) *TaskStore {
	s := &TaskStore{
		tasks:    make(map[string]*Task),
		filePath: filePath,
	}
	s.Load()
	return s
}

func (s *TaskStore) Warning() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.warning
}

func (s *TaskStore) Create(title, description, priority string) (*Task, error) {
	task := &Task{
		ID:          GenerateID(),
		Title:       title,
		Description: description,
		Priority:    priority,
	}
	task.ApplyDefaults()

	if err := task.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskStore) Get(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil
	}
	cp := *t
	return &cp
}

func (s *TaskStore) List(status string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if status == "" || t.Status == status {
			cp := *t
			result = append(result, &cp)
		}
	}
	return result
}

func (s *TaskStore) Update(id string, fields map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}

	if IsTerminal(task.Status) {
		return fmt.Errorf("cannot update %s task %q — create a new task instead", task.Status, id)
	}

	for key, val := range fields {
		switch key {
		case "status":
			if !validStatuses[val] {
				return fmt.Errorf("invalid status %q. Valid: pending, in_progress, completed, failed", val)
			}
			task.Status = val
		case "description":
			task.Description = val
		case "result":
			task.Result = val
		case "agent_type":
			task.AgentType = val
		case "priority":
			if !validPriorities[val] {
				return fmt.Errorf("invalid priority %q. Valid: low, medium, high", val)
			}
			task.Priority = val
		}
	}
	task.UpdatedAt = time.Now().UnixMilli()

	return s.saveLocked()
}

func (s *TaskStore) Complete(id, result string) error {
	return s.Update(id, map[string]string{
		"status": StatusCompleted,
		"result": result,
	})
}

func (s *TaskStore) Fail(id, result string) error {
	return s.Update(id, map[string]string{
		"status": StatusFailed,
		"result": result,
	})
}

func (s *TaskStore) Load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.warning = fmt.Sprintf("tasks.json failed to load: %s", err)
			logger.WarnCF("tasks", "Failed to load tasks.json", map[string]any{"error": err.Error()})
		}
		return
	}

	var tf taskFile
	if err := json.Unmarshal(data, &tf); err != nil {
		s.warning = fmt.Sprintf("tasks.json failed to load: %s", err)
		logger.WarnCF("tasks", "Failed to parse tasks.json", map[string]any{"error": err.Error()})
		return
	}

	cleaned := false
	for _, t := range tf.Tasks {
		if IsExpired(t) {
			cleaned = true
			continue
		}
		s.tasks[t.ID] = t
	}

	if cleaned {
		s.Save()
	}
}

func (s *TaskStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveLocked()
}

func (s *TaskStore) saveLocked() error {
	var tf taskFile
	for _, t := range s.tasks {
		tf.Tasks = append(tf.Tasks, t)
	}

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}

	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks: %w", err)
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		return fmt.Errorf("failed to rename tasks file: %w", err)
	}
	return nil
}
```

Note: The `Save()` method acquires a read lock because `saveLocked()` is also called from `Update()` which already holds the write lock. This avoids deadlocks. The `saveLocked()` helper does the actual work without locking.

- [ ] **Step 4: Run tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/tasks/store.go pkg/tasks/store_test.go
git commit -m "feat(tasks): add TaskStore with CRUD, JSON persistence, and TTL cleanup"
```

---

### Task 3: Summary Generation

**Files:**
- Create: `pkg/tasks/summary.go`
- Create: `pkg/tasks/summary_test.go`

- [ ] **Step 1: Write failing tests**

Create `pkg/tasks/summary_test.go`:

```go
package tasks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSummary_Empty(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	s := store.Summary(10)
	if s != "" {
		t.Fatalf("expected empty summary, got: %s", s)
	}
}

func TestSummary_FullList(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	store.Create("High task", "", PriorityHigh)
	store.Create("Low task", "", PriorityLow)

	s := store.Summary(10)
	if !strings.Contains(s, "Active tasks:") {
		t.Fatalf("expected 'Active tasks:' header, got: %s", s)
	}
	if !strings.Contains(s, "HIGH") {
		t.Fatalf("expected HIGH in summary, got: %s", s)
	}
	if !strings.Contains(s, "High task") {
		t.Fatalf("expected task title in summary, got: %s", s)
	}
}

func TestSummary_SwitchesToCount(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	for i := 0; i < 12; i++ {
		store.Create("Task", "", PriorityMedium)
	}

	s := store.Summary(10)
	if !strings.Contains(s, "You have 12 tasks") {
		t.Fatalf("expected count format for 12 tasks, got: %s", s)
	}
	if !strings.Contains(s, "Top 5") {
		t.Fatalf("expected 'Top 5' in summary, got: %s", s)
	}
}

func TestSummary_SortOrder(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	taskLow, _ := store.Create("Low pending", "", PriorityLow)
	taskHigh, _ := store.Create("High pending", "", PriorityHigh)
	taskInProg, _ := store.Create("In progress", "", PriorityMedium)
	store.Update(taskInProg.ID, map[string]string{"status": StatusInProgress})

	s := store.Summary(10)
	lines := strings.Split(s, "\n")

	// Find the task lines
	var taskLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "- [") {
			taskLines = append(taskLines, line)
		}
	}

	if len(taskLines) != 3 {
		t.Fatalf("expected 3 task lines, got %d: %v", len(taskLines), taskLines)
	}

	// in_progress should be first
	if !strings.Contains(taskLines[0], taskInProg.ID) {
		t.Fatalf("expected in_progress first, got: %s", taskLines[0])
	}
	// high priority pending before low
	if !strings.Contains(taskLines[1], taskHigh.ID) {
		t.Fatalf("expected high priority second, got: %s", taskLines[1])
	}
	_ = taskLow // used in creation
}

func TestPendingSummary_ExcludesTerminal(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	store.Create("Pending task", "", PriorityMedium)
	completed, _ := store.Create("Done task", "", PriorityMedium)
	store.Complete(completed.ID, "done")

	s := store.PendingSummary()
	if strings.Contains(s, "Done task") {
		t.Fatalf("pending summary should not contain completed tasks: %s", s)
	}
	if !strings.Contains(s, "Pending task") {
		t.Fatalf("pending summary should contain pending tasks: %s", s)
	}
}

func TestPendingSummary_Empty(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	s := store.PendingSummary()
	if s != "" {
		t.Fatalf("expected empty pending summary, got: %s", s)
	}
}

func TestSummary_WithWarning(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	// Write corrupted file
	writeFile(t, fp, "not json")

	store := NewTaskStore(fp)
	s := store.Summary(10)
	if !strings.Contains(s, "WARNING") {
		t.Fatalf("expected warning in summary: %s", s)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
```

Add `"os"` to imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -run "TestSummary|TestPending" -count=1`
Expected: Compilation error — Summary and PendingSummary not defined

- [ ] **Step 3: Implement summary generation**

Create `pkg/tasks/summary.go`:

```go
package tasks

import (
	"fmt"
	"sort"
	"strings"
)

var statusOrder = map[string]int{
	StatusInProgress: 0,
	StatusPending:    1,
	StatusFailed:     2,
	StatusCompleted:  3,
}

var priorityOrder = map[string]int{
	PriorityHigh:   0,
	PriorityMedium: 1,
	PriorityLow:    2,
}

func (s *TaskStore) Summary(maxFull int) string {
	s.mu.RLock()
	warning := s.warning
	tasks := s.sortedTasks()
	s.mu.RUnlock()

	var lines []string

	if warning != "" {
		lines = append(lines, fmt.Sprintf("WARNING: %s. The file is at %s — inspect and fix it.", warning, s.filePath))
	}

	if len(tasks) == 0 {
		if len(lines) == 0 {
			return ""
		}
		return strings.Join(lines, "\n")
	}

	if len(tasks) <= maxFull {
		lines = append(lines, "Active tasks:")
		for _, t := range tasks {
			lines = append(lines, formatTaskLine(t))
		}
	} else {
		counts := make(map[string]int)
		for _, t := range tasks {
			counts[t.Status]++
		}
		lines = append(lines, fmt.Sprintf("You have %d tasks (%s).",
			len(tasks), formatCounts(counts)))
		lines = append(lines, "")
		lines = append(lines, "Top 5 by priority:")
		limit := 5
		if len(tasks) < limit {
			limit = len(tasks)
		}
		for _, t := range tasks[:limit] {
			lines = append(lines, formatTaskLine(t))
		}
		lines = append(lines, "")
		lines = append(lines, "Use tasks list for full details.")
	}

	return strings.Join(lines, "\n")
}

func (s *TaskStore) PendingSummary() string {
	s.mu.RLock()
	tasks := s.sortedTasks()
	s.mu.RUnlock()

	var lines []string
	for _, t := range tasks {
		if IsTerminal(t.Status) {
			continue
		}
		lines = append(lines, formatTaskLine(t))
	}

	return strings.Join(lines, "\n")
}

func (s *TaskStore) sortedTasks() []*Task {
	all := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool {
		si, sj := statusOrder[all[i].Status], statusOrder[all[j].Status]
		if si != sj {
			return si < sj
		}
		pi, pj := priorityOrder[all[i].Priority], priorityOrder[all[j].Priority]
		return pi < pj
	})
	return all
}

func formatTaskLine(t *Task) string {
	return fmt.Sprintf("- [%s] %s %s (%s)", t.ID, strings.ToUpper(t.Priority), t.Title, t.Status)
}

func formatCounts(counts map[string]int) string {
	var parts []string
	for _, status := range []string{StatusInProgress, StatusPending, StatusFailed, StatusCompleted} {
		if c, ok := counts[status]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, status))
		}
	}
	return strings.Join(parts, ", ")
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/tasks/summary.go pkg/tasks/summary_test.go
git commit -m "feat(tasks): add Summary and PendingSummary for prompt injection"
```

---

### Task 4: The `tasks` Tool

**Files:**
- Create: `pkg/tools/tasks_tool.go`
- Create: `pkg/tools/tasks_tool_test.go`

- [ ] **Step 1: Write failing tests**

Create `pkg/tools/tasks_tool_test.go`:

```go
package tools

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tasks"
)

func TestTasksTool_Create(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	result := tool.Execute(context.Background(), map[string]any{
		"action":   "create",
		"title":    "Test task",
		"priority": "high",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	all := store.List("")
	if len(all) != 1 {
		t.Fatalf("expected 1 task, got %d", len(all))
	}
}

func TestTasksTool_List(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	store.Create("Task A", "", "medium")
	store.Create("Task B", "", "high")

	result := tool.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Fatal("expected non-empty list")
	}
}

func TestTasksTool_Get(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	task, _ := store.Create("My task", "details here", "medium")

	result := tool.Execute(context.Background(), map[string]any{
		"action": "get",
		"id":     task.ID,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
}

func TestTasksTool_Complete(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	task, _ := store.Create("My task", "", "medium")

	result := tool.Execute(context.Background(), map[string]any{
		"action": "complete",
		"id":     task.ID,
		"result": "all done",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	got := store.Get(task.ID)
	if got.Status != tasks.StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
}

func TestTasksTool_Fail(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	task, _ := store.Create("My task", "", "medium")

	result := tool.Execute(context.Background(), map[string]any{
		"action": "fail",
		"id":     task.ID,
		"result": "crashed",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	got := store.Get(task.ID)
	if got.Status != tasks.StatusFailed {
		t.Fatalf("expected failed, got %s", got.Status)
	}
}

func TestTasksTool_InvalidAction(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	result := tool.Execute(context.Background(), map[string]any{
		"action": "delete",
	})
	if !result.IsError {
		t.Fatal("expected error for invalid action")
	}
}

func TestTasksTool_MissingTitle(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	result := tool.Execute(context.Background(), map[string]any{
		"action": "create",
	})
	if !result.IsError {
		t.Fatal("expected error for missing title")
	}
}

func TestTasksTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewTaskStore(filepath.Join(dir, "tasks.json"))
	tool := NewTasksTool(store)

	result := tool.Execute(context.Background(), map[string]any{
		"action": "get",
		"id":     "nonexistent",
	})
	if !result.IsError {
		t.Fatal("expected error for not found")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tools/ -v -run "TestTasksTool" -count=1`
Expected: Compilation error — NewTasksTool not defined

- [ ] **Step 3: Implement the tasks tool**

Create `pkg/tools/tasks_tool.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tasks"
)

type TasksTool struct {
	store *tasks.TaskStore
}

func NewTasksTool(store *tasks.TaskStore) *TasksTool {
	return &TasksTool{store: store}
}

func (t *TasksTool) Name() string {
	return "tasks"
}

func (t *TasksTool) Description() string {
	return "Manage persistent tasks that survive across sessions. Actions: create, list, get, update, complete, fail."
}

func (t *TasksTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"create", "list", "get", "update", "complete", "fail"},
				"description": "Action to perform.",
			},
			"id": map[string]any{
				"type":        "string",
				"description": "Task ID (required for get, update, complete, fail).",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Task title (required for create).",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Detailed task description.",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "Task priority (default: medium).",
			},
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"pending", "in_progress", "completed", "failed"},
				"description": "Task status (for update action).",
			},
			"result": map[string]any{
				"type":        "string",
				"description": "Task result or outcome.",
			},
			"agent_type": map[string]any{
				"type":        "string",
				"description": "Which agent type worked on this task.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *TasksTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)

	switch action {
	case "create":
		return t.create(args)
	case "list":
		return t.list(args)
	case "get":
		return t.get(args)
	case "update":
		return t.update(args)
	case "complete":
		return t.complete(args)
	case "fail":
		return t.fail(args)
	default:
		return ErrorResult(fmt.Sprintf("unknown action %q. Valid: create, list, get, update, complete, fail", action))
	}
}

func (t *TasksTool) create(args map[string]any) *ToolResult {
	title, _ := args["title"].(string)
	if title == "" {
		return ErrorResult("title is required for create action")
	}
	description, _ := args["description"].(string)
	priority, _ := args["priority"].(string)

	task, err := t.store.Create(title, description, priority)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create task: %s", err))
	}

	data, _ := json.MarshalIndent(task, "", "  ")
	return NewToolResult(fmt.Sprintf("Task created:\n%s", string(data)))
}

func (t *TasksTool) list(args map[string]any) *ToolResult {
	status, _ := args["status"].(string)
	taskList := t.store.List(status)

	if len(taskList) == 0 {
		if status != "" {
			return NewToolResult(fmt.Sprintf("No tasks with status %q.", status))
		}
		return NewToolResult("No tasks.")
	}

	var lines []string
	for _, task := range taskList {
		line := fmt.Sprintf("[%s] %s %s (%s)", task.ID, task.Priority, task.Title, task.Status)
		if task.Result != "" {
			line += fmt.Sprintf(" — %s", task.Result)
		}
		lines = append(lines, line)
	}
	return NewToolResult(fmt.Sprintf("%d task(s):\n%s", len(taskList), joinLines(lines)))
}

func (t *TasksTool) get(args map[string]any) *ToolResult {
	id, _ := args["id"].(string)
	if id == "" {
		return ErrorResult("id is required for get action")
	}

	task := t.store.Get(id)
	if task == nil {
		return ErrorResult(fmt.Sprintf("task %q not found", id))
	}

	data, _ := json.MarshalIndent(task, "", "  ")
	return NewToolResult(string(data))
}

func (t *TasksTool) update(args map[string]any) *ToolResult {
	id, _ := args["id"].(string)
	if id == "" {
		return ErrorResult("id is required for update action")
	}

	fields := make(map[string]string)
	for _, key := range []string{"status", "description", "result", "agent_type", "priority"} {
		if val, ok := args[key].(string); ok && val != "" {
			fields[key] = val
		}
	}

	if len(fields) == 0 {
		return ErrorResult("no fields to update")
	}

	if err := t.store.Update(id, fields); err != nil {
		return ErrorResult(err.Error())
	}

	task := t.store.Get(id)
	data, _ := json.MarshalIndent(task, "", "  ")
	return NewToolResult(fmt.Sprintf("Task updated:\n%s", string(data)))
}

func (t *TasksTool) complete(args map[string]any) *ToolResult {
	id, _ := args["id"].(string)
	if id == "" {
		return ErrorResult("id is required for complete action")
	}
	result, _ := args["result"].(string)

	if err := t.store.Complete(id, result); err != nil {
		return ErrorResult(err.Error())
	}
	return NewToolResult(fmt.Sprintf("Task %q marked as completed.", id))
}

func (t *TasksTool) fail(args map[string]any) *ToolResult {
	id, _ := args["id"].(string)
	if id == "" {
		return ErrorResult("id is required for fail action")
	}
	result, _ := args["result"].(string)

	if err := t.store.Fail(id, result); err != nil {
		return ErrorResult(err.Error())
	}
	return NewToolResult(fmt.Sprintf("Task %q marked as failed.", id))
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tools/ -v -run "TestTasksTool" -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/tasks_tool.go pkg/tools/tasks_tool_test.go
git commit -m "feat(tasks): add tasks tool with create, list, get, update, complete, fail actions"
```

---

### Task 5: Integrate TaskStore into AgentInstance + System Prompt

**Files:**
- Modify: `pkg/agent/instance.go` — add TaskStore field
- Modify: `pkg/agent/context.go` — add taskStore field, setter, inject summary
- Modify: `pkg/agent/loop.go` — init task store, register tasks tool

- [ ] **Step 1: Read the files to find exact integration points**

Read:
- `pkg/agent/instance.go` — find AgentInstance struct, look for AgentRegistry field
- `pkg/agent/context.go` — find ContextBuilder struct, look for agentRegistry field and where agent summary is injected in BuildSystemPrompt()
- `pkg/agent/loop.go` — find where agent registry is initialized and where create_agent tool is registered

- [ ] **Step 2: Add TaskStore field to AgentInstance**

In `pkg/agent/instance.go`, add to AgentInstance struct (after AgentRegistry):

```go
TaskStore *tasks.TaskStore
```

Add import: `"github.com/dawnforge-lab/spawnbot-v5/pkg/tasks"`

- [ ] **Step 3: Add task store to ContextBuilder**

In `pkg/agent/context.go`, add to ContextBuilder struct (after agentRegistry):

```go
taskStore *tasks.TaskStore
```

Add setter:

```go
func (cb *ContextBuilder) SetTaskStore(s *tasks.TaskStore) {
	cb.taskStore = s
}
```

- [ ] **Step 4: Inject task summary into BuildSystemPrompt()**

In `pkg/agent/context.go`, in `BuildSystemPrompt()`, after the agent summary injection block, add:

```go
if cb.taskStore != nil {
	taskSummary := cb.taskStore.Summary(10)
	if taskSummary != "" {
		parts = append(parts, "# Tasks\n\n" + taskSummary)
	}
}
```

- [ ] **Step 5: Initialize task store and register tool in loop.go**

In `pkg/agent/loop.go`, in `registerSharedTools()`, after the agent registry initialization block (where `create_agent` tool is registered), add:

```go
// Initialize task store
taskStorePath := filepath.Join(agent.Workspace, "tasks.json")
taskStore := tasks.NewTaskStore(taskStorePath)
agent.TaskStore = taskStore
agent.ContextBuilder.SetTaskStore(taskStore)
agent.Tools.Register(tools.NewTasksTool(taskStore))
```

Add import: `"github.com/dawnforge-lab/spawnbot-v5/pkg/tasks"`

- [ ] **Step 6: Build and verify**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go build ./pkg/agent/`
Expected: Compiles without errors

- [ ] **Step 7: Commit**

```bash
git add pkg/agent/instance.go pkg/agent/context.go pkg/agent/loop.go
git commit -m "feat(tasks): integrate task store into AgentInstance and system prompt"
```

---

### Task 6: Heartbeat Integration

**Files:**
- Modify: `pkg/heartbeat/service.go` — inject pending tasks into heartbeat prompt

- [ ] **Step 1: Read heartbeat service to find exact integration point**

Read `pkg/heartbeat/service.go` — find `buildPrompt()` method and understand how the HeartbeatService accesses workspace data. Also check how the service is initialized to understand how to pass the TaskStore reference.

- [ ] **Step 2: Add TaskStore field to HeartbeatService**

In the HeartbeatService struct, add:

```go
taskStore *tasks.TaskStore
```

Add a setter or constructor parameter — follow whichever pattern the service uses for its other dependencies.

- [ ] **Step 3: Inject task context into buildPrompt()**

In `buildPrompt()`, after the content is read from HEARTBEAT.md and before the final `fmt.Sprintf`, append task context:

```go
taskContext := ""
if hs.taskStore != nil {
	pending := hs.taskStore.PendingSummary()
	if pending != "" {
		taskContext = "\n\n# Pending Tasks\n\n" + pending +
			"\n\nReview these tasks. Follow up on any that need action — " +
			"start working on pending tasks, check on in_progress tasks, " +
			"or mark tasks as completed/failed."
	}
}
```

Then include `taskContext` in the final prompt string.

- [ ] **Step 4: Wire TaskStore into HeartbeatService initialization**

In `pkg/agent/loop.go` (or wherever HeartbeatService is created), pass the task store to the heartbeat service. This depends on how the heartbeat is initialized — read the code first to find the creation point, then add the task store reference.

- [ ] **Step 5: Build and verify**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go build ./pkg/heartbeat/ ./pkg/agent/`
Expected: Compiles without errors

- [ ] **Step 6: Commit**

```bash
git add pkg/heartbeat/service.go pkg/agent/loop.go
git commit -m "feat(tasks): inject pending tasks into heartbeat prompt"
```

---

### Task 7: End-to-End Integration Tests

**Files:**
- Create: `pkg/tasks/integration_test.go`

- [ ] **Step 1: Write integration tests**

Create `pkg/tasks/integration_test.go`:

```go
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

	// Create tasks
	task1, _ := store.Create("Research auth libraries", "", PriorityHigh)
	task2, _ := store.Create("Write unit tests", "", PriorityMedium)
	task3, _ := store.Create("Update docs", "README needs new section", PriorityLow)

	// Verify list
	all := store.List("")
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(all))
	}

	// Update status
	store.Update(task1.ID, map[string]string{
		"status":     StatusInProgress,
		"agent_type": "researcher",
	})

	// Complete one
	store.Complete(task2.ID, "All 15 tests passing")

	// Fail one
	store.Fail(task3.ID, "README is auto-generated, can't edit")

	// Verify states
	got1 := store.Get(task1.ID)
	if got1.Status != StatusInProgress {
		t.Fatalf("expected in_progress, got %s", got1.Status)
	}
	if got1.AgentType != "researcher" {
		t.Fatalf("expected researcher, got %s", got1.AgentType)
	}

	got2 := store.Get(task2.ID)
	if got2.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got2.Status)
	}

	got3 := store.Get(task3.ID)
	if got3.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", got3.Status)
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

	// Active task (should survive)
	active, _ := store.Create("Still working", "", PriorityHigh)

	// Old completed task (should be cleaned)
	old, _ := store.Create("Old done", "", PriorityLow)
	store.Complete(old.ID, "done long ago")
	store.mu.Lock()
	store.tasks[old.ID].UpdatedAt = time.Now().Add(-8 * 24 * time.Hour).UnixMilli()
	store.mu.Unlock()
	store.Save()

	// Reload
	store2 := NewTaskStore(fp)
	if store2.Get(active.ID) == nil {
		t.Fatal("active task should survive TTL cleanup")
	}
	if store2.Get(old.ID) != nil {
		t.Fatal("old completed task should be cleaned up")
	}
}

func TestEndToEnd_SummaryFormats(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(filepath.Join(dir, "tasks.json"))

	// Few tasks — full list
	store.Create("Task A", "", PriorityHigh)
	store.Create("Task B", "", PriorityMedium)

	s := store.Summary(10)
	if !strings.Contains(s, "Active tasks:") {
		t.Fatalf("expected full list format: %s", s)
	}

	// Add more to exceed threshold
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
		t.Fatalf("pending summary should include pending task: %s", pending)
	}
	if strings.Contains(pending, "Done work") {
		t.Fatalf("pending summary should not include completed task: %s", pending)
	}
}

func TestEndToEnd_CorruptedFileRecovery(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	os.WriteFile(fp, []byte("{invalid"), 0644)

	store := NewTaskStore(fp)

	// Should work with empty store
	task, err := store.Create("New task after corruption", "", PriorityMedium)
	if err != nil {
		t.Fatalf("should be able to create after corruption: %v", err)
	}

	// Warning should be in summary
	s := store.Summary(10)
	if !strings.Contains(s, "WARNING") {
		t.Fatalf("expected warning in summary: %s", s)
	}

	// Task should be there
	if store.Get(task.ID) == nil {
		t.Fatal("task should exist after creation")
	}
}
```

- [ ] **Step 2: Run integration tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -run "TestEndToEnd" -count=1`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/tasks/integration_test.go
git commit -m "test(tasks): add end-to-end integration tests"
```

---

### Task 8: Full Build Verification

**Files:** None (verification only)

- [ ] **Step 1: Run all task tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tasks/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 2: Run tool tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/tools/ -v -run "TestTasksTool" -count=1`
Expected: All tests PASS

- [ ] **Step 3: Build full binary**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" CGO_ENABLED=1 go build -tags fts5 ./cmd/spawnbot/`
Expected: Compiles without errors

- [ ] **Step 4: Run existing tests for regressions**

Run: `cd /home/eugen-dev/Workflows/picoclaw && PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" go test ./pkg/agents/ -v -count=1`
Expected: All agent tests still PASS

- [ ] **Step 5: Manual verification**

```bash
PATH="/home/eugen-dev/go-sdk/go/bin:$PATH" CGO_ENABLED=1 make install
spawnbot agent -m "create a task to test the task system, then list all tasks"
```

Expected: Agent creates a task and lists it back.

- [ ] **Step 6: Commit if fixes needed**

```bash
git add -A
git commit -m "fix(tasks): address issues found during build verification"
```
