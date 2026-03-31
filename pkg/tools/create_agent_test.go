package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
)

func TestCreateAgentTool_Success(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()

	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name":          "sql-expert",
		"description":   "Specializes in database queries",
		"system_prompt": "You are a SQL expert agent.",
		"tools_deny":    []any{"message", "send_file"},
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	agentFile := filepath.Join(dir, "sql-expert", "AGENT.md")
	if _, err := os.Stat(agentFile); os.IsNotExist(err) {
		t.Fatal("AGENT.md was not created")
	}

	if registry.Get("sql-expert") == nil {
		t.Fatal("agent not registered in registry")
	}
}

func TestCreateAgentTool_RejectBuiltinOverwrite(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()
	registry.Register(&agents.AgentDefinition{
		Name: "researcher", Description: "builtin", SystemPrompt: "p",
		Source: "builtin", MaxIterations: 20, Timeout: 5 * time.Minute,
	})

	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name":          "researcher",
		"description":   "custom researcher",
		"system_prompt": "override",
	})

	if !result.IsError {
		t.Fatal("expected error when overwriting builtin")
	}
}

func TestCreateAgentTool_InvalidName(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()
	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name":          "bad name!",
		"description":   "invalid",
		"system_prompt": "prompt",
	})

	if !result.IsError {
		t.Fatal("expected error for invalid name")
	}
}

func TestCreateAgentTool_MissingRequired(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()
	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name": "test",
	})

	if !result.IsError {
		t.Fatal("expected error for missing description")
	}
}
