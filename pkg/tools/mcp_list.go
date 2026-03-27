package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/mcp"
)

// ListMCPTool lists all connected MCP servers and their tools.
type ListMCPTool struct {
	manager *mcp.Manager
}

func NewListMCPTool(manager *mcp.Manager) *ListMCPTool {
	return &ListMCPTool{manager: manager}
}

func (t *ListMCPTool) Name() string {
	return "list_mcp"
}

func (t *ListMCPTool) Description() string {
	return "List all connected MCP servers and their available tools."
}

func (t *ListMCPTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *ListMCPTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	servers := t.manager.GetServers()

	if len(servers) == 0 {
		return &ToolResult{
			ForLLM:  "No MCP servers are currently connected.",
			ForUser: "No MCP servers connected",
		}
	}

	// Sort server names for deterministic output
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	var lines []string
	totalTools := 0
	for _, name := range names {
		conn := servers[name]
		toolNames := make([]string, 0, len(conn.Tools))
		for _, tool := range conn.Tools {
			toolNames = append(toolNames, tool.Name)
		}
		totalTools += len(toolNames)

		if len(toolNames) > 0 {
			lines = append(lines, fmt.Sprintf("- %s (%d tools): %s",
				name, len(toolNames), strings.Join(toolNames, ", ")))
		} else {
			lines = append(lines, fmt.Sprintf("- %s (0 tools)", name))
		}
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("Connected MCP servers (%d servers, %d tools):\n%s",
			len(servers), totalTools, strings.Join(lines, "\n")),
		ForUser: fmt.Sprintf("%d MCP servers, %d tools", len(servers), totalTools),
	}
}
