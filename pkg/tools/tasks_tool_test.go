package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tasks"
)

func newTestTasksTool(t *testing.T) *TasksTool {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "tasks.json")
	store := tasks.NewTaskStore(storePath)
	return NewTasksTool(store)
}

func TestTasksTool_Create(t *testing.T) {
	tool := newTestTasksTool(t)
	result := tool.Execute(context.Background(), map[string]any{
		"action":   "create",
		"title":    "Fix the bug",
		"priority": "high",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "Task created") {
		t.Errorf("expected 'Task created' in result, got: %s", result.ForLLM)
	}
	taskList := tool.store.List("")
	if len(taskList) != 1 {
		t.Errorf("expected 1 task in store, got %d", len(taskList))
	}
	if taskList[0].Title != "Fix the bug" {
		t.Errorf("expected title 'Fix the bug', got %q", taskList[0].Title)
	}
	if taskList[0].Priority != "high" {
		t.Errorf("expected priority 'high', got %q", taskList[0].Priority)
	}
}

func TestTasksTool_List(t *testing.T) {
	tool := newTestTasksTool(t)
	tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"title":  "Task One",
	})
	tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"title":  "Task Two",
	})
	result := tool.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if strings.Contains(result.ForLLM, "No tasks") {
		t.Error("expected non-empty task list")
	}
	if !strings.Contains(result.ForLLM, "2 task(s)") {
		t.Errorf("expected '2 task(s)' in result, got: %s", result.ForLLM)
	}
}

func TestTasksTool_Get(t *testing.T) {
	tool := newTestTasksTool(t)
	createResult := tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"title":  "Get me",
	})
	if createResult.IsError {
		t.Fatalf("create failed: %s", createResult.ForLLM)
	}
	taskList := tool.store.List("")
	if len(taskList) == 0 {
		t.Fatal("no tasks in store after create")
	}
	id := taskList[0].ID

	result := tool.Execute(context.Background(), map[string]any{
		"action": "get",
		"id":     id,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, id) {
		t.Errorf("expected task ID %q in result, got: %s", id, result.ForLLM)
	}
}

func TestTasksTool_Complete(t *testing.T) {
	tool := newTestTasksTool(t)
	tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"title":  "Complete me",
	})
	taskList := tool.store.List("")
	if len(taskList) == 0 {
		t.Fatal("no tasks in store after create")
	}
	id := taskList[0].ID

	result := tool.Execute(context.Background(), map[string]any{
		"action": "complete",
		"id":     id,
		"result": "done successfully",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "completed") {
		t.Errorf("expected 'completed' in result, got: %s", result.ForLLM)
	}

	task := tool.store.Get(id)
	if task == nil {
		t.Fatal("task not found after complete")
	}
	if task.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", task.Status)
	}
	if task.Result != "done successfully" {
		t.Errorf("expected result 'done successfully', got %q", task.Result)
	}
}

func TestTasksTool_Fail(t *testing.T) {
	tool := newTestTasksTool(t)
	tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"title":  "Fail me",
	})
	taskList := tool.store.List("")
	if len(taskList) == 0 {
		t.Fatal("no tasks in store after create")
	}
	id := taskList[0].ID

	result := tool.Execute(context.Background(), map[string]any{
		"action": "fail",
		"id":     id,
		"result": "something went wrong",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "failed") {
		t.Errorf("expected 'failed' in result, got: %s", result.ForLLM)
	}

	task := tool.store.Get(id)
	if task == nil {
		t.Fatal("task not found after fail")
	}
	if task.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", task.Status)
	}
	if task.Result != "something went wrong" {
		t.Errorf("expected result 'something went wrong', got %q", task.Result)
	}
}

func TestTasksTool_InvalidAction(t *testing.T) {
	tool := newTestTasksTool(t)
	result := tool.Execute(context.Background(), map[string]any{
		"action": "delete",
	})
	if !result.IsError {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(result.ForLLM, "unknown action") {
		t.Errorf("expected 'unknown action' in error, got: %s", result.ForLLM)
	}
}

func TestTasksTool_MissingTitle(t *testing.T) {
	tool := newTestTasksTool(t)
	result := tool.Execute(context.Background(), map[string]any{
		"action": "create",
	})
	if !result.IsError {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(result.ForLLM, "title is required") {
		t.Errorf("expected 'title is required' in error, got: %s", result.ForLLM)
	}
}

func TestTasksTool_NotFound(t *testing.T) {
	tool := newTestTasksTool(t)
	result := tool.Execute(context.Background(), map[string]any{
		"action": "get",
		"id":     "nonexistent",
	})
	if !result.IsError {
		t.Fatal("expected error for nonexistent task")
	}
	if !strings.Contains(result.ForLLM, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", result.ForLLM)
	}
}
