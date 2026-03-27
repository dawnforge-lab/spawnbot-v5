package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/mcp"
)

// ConnectMCPTool connects to an MCP server at runtime and registers its tools.
type ConnectMCPTool struct {
	manager  *mcp.Manager
	registry *ToolRegistry
}

func NewConnectMCPTool(manager *mcp.Manager, registry *ToolRegistry) *ConnectMCPTool {
	return &ConnectMCPTool{
		manager:  manager,
		registry: registry,
	}
}

func (t *ConnectMCPTool) Name() string {
	return "connect_mcp"
}

func (t *ConnectMCPTool) Description() string {
	return "Connect to an MCP server at runtime and register its tools. Use this to add new tool capabilities from MCP servers (stdio command or SSE/HTTP URL)."
}

func (t *ConnectMCPTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Unique name for this server connection",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "Command to run for stdio transport (e.g., 'python3', 'node')",
			},
			"args": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Command arguments for stdio transport",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "URL for SSE/HTTP transport",
			},
			"env": map[string]any{
				"type":        "object",
				"description": "Environment variables for the server process",
			},
		},
		"required": []string{"name"},
	}
}

func (t *ConnectMCPTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return ErrorResult("name is required")
	}

	// Check if already connected
	if _, ok := t.manager.GetServer(name); ok {
		return ErrorResult(fmt.Sprintf("MCP server '%s' is already connected", name))
	}

	// Build server config from args
	serverCfg := config.MCPServerConfig{
		Enabled: true,
	}

	if cmd, ok := args["command"].(string); ok {
		serverCfg.Command = cmd
	}
	if argsList, ok := args["args"].([]any); ok {
		for _, a := range argsList {
			if s, ok := a.(string); ok {
				serverCfg.Args = append(serverCfg.Args, s)
			}
		}
	}
	if url, ok := args["url"].(string); ok {
		serverCfg.URL = url
	}
	if envMap, ok := args["env"].(map[string]any); ok {
		serverCfg.Env = make(map[string]string, len(envMap))
		for k, v := range envMap {
			if s, ok := v.(string); ok {
				serverCfg.Env[k] = s
			}
		}
	}

	// Validate: need either command or URL
	if serverCfg.Command == "" && serverCfg.URL == "" {
		return ErrorResult("either 'command' or 'url' must be provided")
	}

	// Connect
	if err := t.manager.ConnectServer(ctx, name, serverCfg); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to connect MCP server '%s': %v", name, err)).WithError(err)
	}

	// Get the newly connected server's tools
	conn, ok := t.manager.GetServer(name)
	if !ok {
		return ErrorResult(fmt.Sprintf("Server '%s' connected but not found in manager", name))
	}

	// Register tools in this agent's registry
	var registered []string
	for _, tool := range conn.Tools {
		mcpTool := NewMCPTool(t.manager, name, tool)
		t.registry.Register(mcpTool)
		registered = append(registered, mcpTool.Name())
	}

	if len(registered) == 0 {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Connected MCP server '%s' but it provides no tools.", name),
			ForUser: fmt.Sprintf("Connected MCP server '%s' (0 tools)", name),
		}
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("Connected MCP server '%s' with %d tools:\n%s",
			name, len(registered), strings.Join(registered, "\n")),
		ForUser: fmt.Sprintf("Connected MCP server '%s' (%d tools)", name, len(registered)),
	}
}
