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

func (t *TasksTool) Name() string { return "tasks" }

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
		return t.getTask(args)
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

func (t *TasksTool) getTask(args map[string]any) *ToolResult {
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
