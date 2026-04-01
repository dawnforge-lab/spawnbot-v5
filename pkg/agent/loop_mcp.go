// Spawnbot - Personal AI assistant
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package agent

import (
	"context"
	"strings"
	"sync"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/mcp"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

type mcpRuntime struct {
	initOnce sync.Once
	mu       sync.Mutex
	manager  *mcp.Manager
	initErr  error
}

func (r *mcpRuntime) setManager(manager *mcp.Manager) {
	r.mu.Lock()
	r.manager = manager
	r.initErr = nil
	r.mu.Unlock()
}

func (r *mcpRuntime) setInitErr(err error) {
	r.mu.Lock()
	r.initErr = err
	r.mu.Unlock()
}

func (r *mcpRuntime) getInitErr() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.initErr
}

func (r *mcpRuntime) takeManager() *mcp.Manager {
	r.mu.Lock()
	defer r.mu.Unlock()
	manager := r.manager
	r.manager = nil
	return manager
}

func (r *mcpRuntime) hasManager() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.manager != nil
}

// ensureMCPInitialized loads MCP servers/tools once so both Run() and direct
// agent mode share the same initialization path.
// When MCP is enabled, the manager and management tools (connect_mcp,
// disconnect_mcp, list_mcp) are always created — even with zero initial
// servers — so the agent can connect to MCP servers at runtime.
func (al *AgentLoop) ensureMCPInitialized(ctx context.Context) error {
	if !al.cfg.Tools.IsToolEnabled("mcp") {
		return nil
	}

	al.mcp.initOnce.Do(func() {
		mcpManager := mcp.NewManager()

		agentIDs := al.registry.ListAgentIDs()

		// Connect initial servers if any are configured
		hasInitialServers := false
		for _, serverCfg := range al.cfg.Tools.MCP.Servers {
			if serverCfg.Enabled {
				hasInitialServers = true
				break
			}
		}

		if hasInitialServers {
			defaultAgent := al.registry.GetDefaultAgent()
			workspacePath := al.cfg.WorkspacePath()
			if defaultAgent != nil && defaultAgent.Workspace != "" {
				workspacePath = defaultAgent.Workspace
			}

			if err := mcpManager.LoadFromMCPConfig(ctx, al.cfg.Tools.MCP, workspacePath); err != nil {
				logger.WarnCF("agent", "Failed to load initial MCP servers",
					map[string]any{"error": err.Error()})
				// Continue — management tools are still useful for runtime connections
			}

			// Register tools from initial servers
			servers := mcpManager.GetServers()
			uniqueTools := 0
			totalRegistrations := 0

			for serverName, conn := range servers {
				uniqueTools += len(conn.Tools)

				for _, tool := range conn.Tools {
					for _, agentID := range agentIDs {
						agent, ok := al.registry.GetAgent(agentID)
						if !ok {
							continue
						}

						mcpTool := tools.NewMCPTool(mcpManager, serverName, tool)
						hint := firstSentence(tool.Description, 80)
						agent.Tools.RegisterHiddenWithHint(mcpTool, hint)

						totalRegistrations++
						logger.DebugCF("agent", "Registered MCP tool",
							map[string]any{
								"agent_id": agentID,
								"server":   serverName,
								"tool":     tool.Name,
								"name":     mcpTool.Name(),
								"deferred": true,
							})
					}
				}
			}
			logger.InfoCF("agent", "MCP tools registered",
				map[string]any{
					"server_count":        len(servers),
					"unique_tools":        uniqueTools,
					"total_registrations": totalRegistrations,
					"agent_count":         len(agentIDs),
				})
		}

		// Always register runtime MCP management tools so the agent can
		// connect to new servers at runtime (e.g., after creating an MCP
		// server via the skill-creator).
		for _, agentID := range agentIDs {
			agent, ok := al.registry.GetAgent(agentID)
			if !ok {
				continue
			}
			agent.Tools.RegisterHidden(tools.NewConnectMCPTool(mcpManager, agent.Tools))
			agent.Tools.RegisterHidden(tools.NewDisconnectMCPTool(mcpManager, agent.Tools))
			agent.Tools.RegisterHidden(tools.NewListMCPTool(mcpManager))
		}

		al.mcp.setManager(mcpManager)
	})

	return al.mcp.getInitErr()
}

// serverIsDeferred reports whether an MCP server's tools should be registered
// as hidden (deferred/discovery mode).
//
// The per-server Deferred field takes precedence over the global discoveryEnabled
// default. When Deferred is nil, discoveryEnabled is used as the fallback.
func serverIsDeferred(discoveryEnabled bool, serverCfg config.MCPServerConfig) bool {
	if !discoveryEnabled {
		return false
	}
	if serverCfg.Deferred != nil {
		return *serverCfg.Deferred
	}
	return true
}

// firstSentence returns the first sentence of s (up to maxLen characters).
// If no sentence-ending punctuation is found within maxLen, the string is
// truncated at maxLen.
func firstSentence(s string, maxLen int) string {
	if i := strings.IndexAny(s, ".!?"); i >= 0 && i < maxLen {
		return s[:i+1]
	}
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
