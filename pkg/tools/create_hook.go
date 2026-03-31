package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// HookMounter is the interface for mounting hooks at runtime.
type HookMounter interface {
	MountCommandHook(name, scriptPath, mode string, events, tools []string, timeoutMS int) error
}

// ConfigHookUpdater updates the config with a new command hook entry.
type ConfigHookUpdater interface {
	AddCommandHook(name string, script, mode string, events, tools []string, timeoutMS int) error
}

// CreateHookTool allows the agent to create shell script hooks at runtime.
type CreateHookTool struct {
	workspace     string
	mounter       HookMounter
	configUpdater ConfigHookUpdater
}

// NewCreateHookTool creates a new create_hook tool.
func NewCreateHookTool(workspace string, mounter HookMounter, configUpdater ConfigHookUpdater) *CreateHookTool {
	return &CreateHookTool{
		workspace:     workspace,
		mounter:       mounter,
		configUpdater: configUpdater,
	}
}

func (t *CreateHookTool) Name() string { return "create_hook" }

func (t *CreateHookTool) Description() string {
	return "Create a shell script hook that runs on agent lifecycle events. Hooks can observe events (fire-and-forget) or intercept tool calls (can block/modify). The hook is immediately active."
}

func (t *CreateHookTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Hook name (alphanumeric + hyphens)",
			},
			"script": map[string]any{
				"type":        "string",
				"description": "Bash script content. Receives JSON on stdin. Exit 0=continue, 1=deny (intercept mode). Stdout JSON can modify tool arguments.",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"observe", "intercept"},
				"description": "observe=fire-and-forget, intercept=can block/modify tool calls",
			},
			"events": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Event kinds to listen to (e.g. tool_exec_start, tool_exec_end, turn_start, turn_end)",
			},
			"tools": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tool names to filter (empty=all tools). Only applies to tool events.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Timeout in milliseconds (default 5000 for observe, 10000 for intercept)",
			},
		},
		"required": []string{"name", "script", "mode", "events"},
	}
}

func (t *CreateHookTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	script, _ := args["script"].(string)
	mode, _ := args["mode"].(string)
	timeoutMS, _ := args["timeout_ms"].(float64)

	if name == "" {
		return ErrorResult("hook name is required")
	}
	if script == "" {
		return ErrorResult("script content is required")
	}
	if mode != "observe" && mode != "intercept" {
		return ErrorResult(fmt.Sprintf("mode must be 'observe' or 'intercept', got %q", mode))
	}

	events := extractStringArray(args, "events")
	if len(events) == 0 {
		return ErrorResult("at least one event kind is required")
	}
	tools := extractStringArray(args, "tools")

	// Write script file
	hooksDir := filepath.Join(t.workspace, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create hooks directory: %v", err))
	}

	scriptPath := filepath.Join(hooksDir, name+".sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to write hook script: %v", err))
	}

	// Update config if updater is available
	if t.configUpdater != nil {
		if err := t.configUpdater.AddCommandHook(name, "hooks/"+name+".sh", mode, events, tools, int(timeoutMS)); err != nil {
			return ErrorResult(fmt.Sprintf("failed to update config: %v", err))
		}
	}

	// Hot-mount if mounter is available
	if t.mounter != nil {
		if err := t.mounter.MountCommandHook(name, scriptPath, mode, events, tools, int(timeoutMS)); err != nil {
			return ErrorResult(fmt.Sprintf("hook script created but failed to mount: %v", err))
		}
	}

	return NewToolResult(fmt.Sprintf("Hook %q created at %s and mounted. Mode: %s, events: %v", name, scriptPath, mode, events))
}

func extractStringArray(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
