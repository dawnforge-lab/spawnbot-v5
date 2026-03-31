package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/skills"
)

// UseSkillTool activates a skill with arguments, handling inline/fork/spawn execution.
type UseSkillTool struct {
	loader      *skills.SkillsLoader
	spawner     SubTurnSpawner
	workspace   string
	agentsDir   string
	model       string
	maxTokens   int
	temperature float64
}

// NewUseSkillTool creates a new use_skill tool.
func NewUseSkillTool(loader *skills.SkillsLoader, spawner SubTurnSpawner, workspace, agentsDir, model string, maxTokens int, temperature float64) *UseSkillTool {
	return &UseSkillTool{
		loader:      loader,
		spawner:     spawner,
		workspace:   workspace,
		agentsDir:   agentsDir,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
	}
}

func (t *UseSkillTool) Name() string { return "use_skill" }

func (t *UseSkillTool) Description() string {
	return "Activate a skill with optional arguments. Skills run inline (default), as a sync subagent (fork), or async subagent (spawn) depending on the skill's configuration."
}

func (t *UseSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{
				"type":        "string",
				"description": "Name of the skill to activate",
			},
			"arguments": map[string]any{
				"type":        "string",
				"description": "Arguments to pass to the skill (space-separated)",
			},
		},
		"required": []string{"skill"},
	}
}

func (t *UseSkillTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	skillName, _ := args["skill"].(string)
	if skillName == "" {
		return ErrorResult("skill name is required")
	}

	argsStr, _ := args["arguments"].(string)
	var argsList []string
	if argsStr != "" {
		argsList = strings.Fields(argsStr)
	}

	if t.loader == nil {
		return ErrorResult(fmt.Sprintf("skill %q not found", skillName))
	}

	// Verify skill exists
	content, ok := t.loader.LoadSkill(skillName)
	if !ok || content == "" {
		return ErrorResult(fmt.Sprintf("skill %q not found or empty", skillName))
	}

	// Get metadata and skill directory
	meta := t.loader.GetSkillMetadata(skillName)
	skillDir := t.loader.GetSkillDir(skillName)

	// Substitute arguments
	content = skills.SubstituteArgs(content, argsList, skillDir, t.workspace)

	// Execute based on context
	switch meta.Context {
	case "fork":
		return t.executeFork(ctx, content, meta, skillName)
	case "spawn":
		return t.executeSpawn(ctx, content, meta, skillName)
	default:
		// Inline: return content for injection into conversation
		return &ToolResult{
			ForLLM:  content,
			ForUser: fmt.Sprintf("Skill %q activated", skillName),
			Silent:  true,
		}
	}
}

func (t *UseSkillTool) executeFork(ctx context.Context, content string, meta skills.SkillMetadata, skillName string) *ToolResult {
	if t.spawner == nil {
		return ErrorResult("subagent spawner not configured, cannot fork skill")
	}

	cfg := SubTurnConfig{
		Model:        t.model,
		MaxTokens:    t.maxTokens,
		Temperature:  t.temperature,
		SystemPrompt: content,
		Async:        false,
	}
	if meta.AgentType != "" {
		cfg.AgentType = meta.AgentType
	}

	result, err := t.spawner.SpawnSubTurn(ctx, cfg)
	if err != nil {
		return ErrorResult(fmt.Sprintf("fork skill %q failed: %v", skillName, err))
	}
	return result
}

func (t *UseSkillTool) executeSpawn(ctx context.Context, content string, meta skills.SkillMetadata, skillName string) *ToolResult {
	if t.spawner == nil {
		return ErrorResult("subagent spawner not configured, cannot spawn skill")
	}

	cfg := SubTurnConfig{
		Model:        t.model,
		MaxTokens:    t.maxTokens,
		Temperature:  t.temperature,
		SystemPrompt: content,
		Async:        true,
	}
	if meta.AgentType != "" {
		cfg.AgentType = meta.AgentType
	}

	result, err := t.spawner.SpawnSubTurn(ctx, cfg)
	if err != nil {
		return ErrorResult(fmt.Sprintf("spawn skill %q failed: %v", skillName, err))
	}
	return result
}
