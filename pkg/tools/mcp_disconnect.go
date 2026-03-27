package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/mcp"
)

// DisconnectMCPTool disconnects an MCP server and unregisters its tools.
type DisconnectMCPTool struct {
	manager  *mcp.Manager
	registry *ToolRegistry
}

func NewDisconnectMCPTool(manager *mcp.Manager, registry *ToolRegistry) *DisconnectMCPTool {
	return &DisconnectMCPTool{
		manager:  manager,
		registry: registry,
	}
}

func (t *DisconnectMCPTool) Name() string {
	return "disconnect_mcp"
}

func (t *DisconnectMCPTool) Description() string {
	return "Disconnect an MCP server and remove its tools. Use this to clean up servers that are no longer needed."
}

func (t *DisconnectMCPTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Name of the MCP server to disconnect",
			},
		},
		"required": []string{"name"},
	}
}

func (t *DisconnectMCPTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return ErrorResult("name is required")
	}

	// Unregister all tools from this server
	prefix := "mcp_" + sanitizeIdentifierComponent(name) + "_"
	removed := t.registry.UnregisterByPrefix(prefix)

	// Disconnect the server
	if err := t.manager.DisconnectServer(name); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to disconnect MCP server '%s': %v", name, err)).WithError(err)
	}

	return &ToolResult{
		ForLLM:  fmt.Sprintf("Disconnected MCP server '%s', removed %d tools.", name, removed),
		ForUser: fmt.Sprintf("Disconnected '%s' (%d tools removed)", name, removed),
	}
}
