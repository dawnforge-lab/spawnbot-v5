//go:build cgo

package agent

import (
	"os"
	"path/filepath"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/memory"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// registerSemanticMemory initializes the SQLite-backed semantic memory store
// and registers memory_store, memory_search, memory_recall tools.
func registerSemanticMemory(cfg *config.Config, workspace string, registry *tools.ToolRegistry) {
	if cfg.Embeddings.Provider == "" {
		return
	}

	memoryDir := filepath.Join(workspace, "memory")
	os.MkdirAll(memoryDir, 0o755)

	sqliteStore, err := memory.NewSQLiteStore(memoryDir, cfg.Embeddings.Dimensions)
	if err != nil {
		logger.ErrorCF("agent", "Failed to initialize semantic memory store; continuing without semantic memory",
			map[string]any{"error": err.Error()})
		return
	}

	// Create embedding provider (may be nil on error — tools degrade to FTS-only).
	var embedder memory.EmbeddingProvider
	embCfg := memory.EmbeddingConfig{
		Provider:   cfg.Embeddings.Provider,
		Model:      cfg.Embeddings.Model,
		APIKey:     cfg.Embeddings.APIKey,
		BaseURL:    cfg.Embeddings.BaseURL,
		Dimensions: cfg.Embeddings.Dimensions,
	}
	embedder, err = memory.NewEmbeddingProvider(embCfg)
	if err != nil {
		logger.WarnCF("agent", "Failed to create embedding provider; memory tools will use FTS-only",
			map[string]any{"error": err.Error()})
	}

	registry.Register(memory.NewMemoryStoreTool(sqliteStore, embedder))
	registry.Register(memory.NewMemorySearchTool(sqliteStore, embedder))
	registry.Register(memory.NewMemoryRecallTool(sqliteStore))
}
