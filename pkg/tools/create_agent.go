package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
)

type CreateAgentTool struct {
	agentsDir string
	registry  *agents.Registry
}

func NewCreateAgentTool(agentsDir string, registry *agents.Registry) *CreateAgentTool {
	return &CreateAgentTool{
		agentsDir: agentsDir,
		registry:  registry,
	}
}

func (t *CreateAgentTool) Name() string {
	return "create_agent"
}

func (t *CreateAgentTool) Description() string {
	return "Create a new agent type definition. The agent becomes immediately available for use with spawn/subagent tools."
}

func (t *CreateAgentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Unique agent name (alphanumeric + hyphens, max 64 chars)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "What this agent does (shown in agent summary)",
			},
			"system_prompt": map[string]any{
				"type":        "string",
				"description": "The agent's system prompt (markdown)",
			},
			"tools_allow": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tools to allow (empty = all tools)",
			},
			"tools_deny": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tools to deny",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Model override (empty = inherit parent model)",
			},
			"max_iterations": map[string]any{
				"type":        "integer",
				"description": "Maximum iterations (default 20)",
			},
			"timeout": map[string]any{
				"type":        "string",
				"description": "Timeout duration (default 5m)",
			},
		},
		"required": []string{"name", "description", "system_prompt"},
	}
}

func (t *CreateAgentTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	systemPrompt, _ := args["system_prompt"].(string)
	model, _ := args["model"].(string)
	maxIters, _ := args["max_iterations"].(float64)
	timeoutStr, _ := args["timeout"].(string)

	if t.registry.IsBuiltin(name) {
		return ErrorResult(fmt.Sprintf("cannot overwrite builtin agent %q — use a different name", name))
	}

	var fm strings.Builder
	fm.WriteString("---\n")
	fm.WriteString(fmt.Sprintf("name: %s\n", name))
	fm.WriteString(fmt.Sprintf("description: %q\n", description))
	if model != "" {
		fm.WriteString(fmt.Sprintf("model: %s\n", model))
	}

	if toolsAllow, ok := args["tools_allow"]; ok {
		if arr, ok := toolsAllow.([]any); ok && len(arr) > 0 {
			fm.WriteString("tools_allow:\n")
			for _, v := range arr {
				fm.WriteString(fmt.Sprintf("  - %s\n", v))
			}
		}
	}

	if toolsDeny, ok := args["tools_deny"]; ok {
		if arr, ok := toolsDeny.([]any); ok && len(arr) > 0 {
			fm.WriteString("tools_deny:\n")
			for _, v := range arr {
				fm.WriteString(fmt.Sprintf("  - %s\n", v))
			}
		}
	}

	if maxIters > 0 {
		fm.WriteString(fmt.Sprintf("max_iterations: %d\n", int(maxIters)))
	}
	if timeoutStr != "" {
		fm.WriteString(fmt.Sprintf("timeout: %s\n", timeoutStr))
	}
	fm.WriteString("---\n\n")
	fm.WriteString(systemPrompt)

	content := fm.String()

	def, err := agents.ParseAgentMD(content, "workspace", filepath.Join(t.agentsDir, name))
	if err != nil {
		return ErrorResult(fmt.Sprintf("generated AGENT.md failed validation: %s", err))
	}

	agentDir := filepath.Join(t.agentsDir, name)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create agent directory: %s", err))
	}

	agentFile := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
		return ErrorResult(fmt.Sprintf("failed to write AGENT.md: %s", err))
	}

	t.registry.Register(def)

	return NewToolResult(fmt.Sprintf("Agent %q created and registered. Available immediately for spawn/subagent with agent_type=%q.", name, name))
}
