// Spawnbot - Personal AI assistant
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package agent

import (
	"context"
	"fmt"
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

				serverCfg := al.cfg.Tools.MCP.Servers[serverName]
				registerAsHidden := serverIsDeferred(al.cfg.Tools.MCP.Discovery.Enabled, serverCfg)

				for _, tool := range conn.Tools {
					for _, agentID := range agentIDs {
						agent, ok := al.registry.GetAgent(agentID)
						if !ok {
							continue
						}

						mcpTool := tools.NewMCPTool(mcpManager, serverName, tool)
						if registerAsHidden {
							agent.Tools.RegisterHidden(mcpTool)
						} else {
							agent.Tools.Register(mcpTool)
						}

						totalRegistrations++
						logger.DebugCF("agent", "Registered MCP tool",
							map[string]any{
								"agent_id": agentID,
								"server":   serverName,
								"tool":     tool.Name,
								"name":     mcpTool.Name(),
								"deferred": registerAsHidden,
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
			agent.Tools.Register(tools.NewConnectMCPTool(mcpManager, agent.Tools))
			agent.Tools.Register(tools.NewDisconnectMCPTool(mcpManager, agent.Tools))
			agent.Tools.Register(tools.NewListMCPTool(mcpManager))
		}

		// Initializes Discovery Tools only if enabled by configuration
		if al.cfg.Tools.MCP.Discovery.Enabled {
			useBM25 := al.cfg.Tools.MCP.Discovery.UseBM25
			useRegex := al.cfg.Tools.MCP.Discovery.UseRegex

			if !useBM25 && !useRegex {
				al.mcp.setInitErr(fmt.Errorf(
					"tool discovery is enabled but neither 'use_bm25' nor 'use_regex' is set to true in the configuration",
				))
				if closeErr := mcpManager.Close(); closeErr != nil {
					logger.ErrorCF("agent", "Failed to close MCP manager",
						map[string]any{"error": closeErr.Error()})
				}
				return
			}

			ttl := al.cfg.Tools.MCP.Discovery.TTL
			if ttl <= 0 {
				ttl = 5
			}

			maxSearchResults := al.cfg.Tools.MCP.Discovery.MaxSearchResults
			if maxSearchResults <= 0 {
				maxSearchResults = 5
			}

			logger.InfoCF("agent", "Initializing tool discovery", map[string]any{
				"bm25": useBM25, "regex": useRegex, "ttl": ttl, "max_results": maxSearchResults,
			})

			for _, agentID := range agentIDs {
				agent, ok := al.registry.GetAgent(agentID)
				if !ok {
					continue
				}

				if useRegex {
					agent.Tools.Register(tools.NewRegexSearchTool(agent.Tools, ttl, maxSearchResults))
				}
				if useBM25 {
					agent.Tools.Register(tools.NewBM25SearchTool(agent.Tools, ttl, maxSearchResults))
				}
			}
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
