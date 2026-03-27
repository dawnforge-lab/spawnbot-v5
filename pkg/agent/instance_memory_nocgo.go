//go:build !cgo

package agent

import (
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// registerSemanticMemory is a no-op when CGO is disabled.
// Semantic memory requires CGO for the SQLite + sqlite-vec backend.
func registerSemanticMemory(_ *config.Config, _ string, _ *tools.ToolRegistry) {}
