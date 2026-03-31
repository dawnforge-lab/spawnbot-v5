package tools

import (
	"context"
	"testing"
)

func TestCreateHookTool_Name(t *testing.T) {
	tool := NewCreateHookTool("", nil, nil)
	if tool.Name() != "create_hook" {
		t.Errorf("expected name %q, got %q", "create_hook", tool.Name())
	}
}

func TestCreateHookTool_Parameters(t *testing.T) {
	tool := NewCreateHookTool("", nil, nil)
	params := tool.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	for _, required := range []string{"name", "script", "mode", "events"} {
		if _, ok := props[required]; !ok {
			t.Errorf("missing %q parameter", required)
		}
	}
}

func TestCreateHookTool_MissingName(t *testing.T) {
	tool := NewCreateHookTool("", nil, nil)
	result := tool.Execute(context.Background(), map[string]any{
		"script": "#!/bin/bash\nexit 0",
		"mode":   "observe",
		"events": []any{"tool_exec_end"},
	})
	if !result.IsError {
		t.Error("expected error for missing name")
	}
}

func TestCreateHookTool_InvalidMode(t *testing.T) {
	dir := t.TempDir()
	tool := NewCreateHookTool(dir, nil, nil)
	result := tool.Execute(context.Background(), map[string]any{
		"name":   "test-hook",
		"script": "#!/bin/bash\nexit 0",
		"mode":   "invalid",
		"events": []any{"tool_exec_end"},
	})
	if !result.IsError {
		t.Error("expected error for invalid mode")
	}
}

func TestCreateHookTool_CreatesScript(t *testing.T) {
	dir := t.TempDir()
	tool := NewCreateHookTool(dir, nil, nil)
	result := tool.Execute(context.Background(), map[string]any{
		"name":   "test-hook",
		"script": "#!/bin/bash\nexit 0",
		"mode":   "observe",
		"events": []any{"tool_exec_end"},
	})
	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}
}
