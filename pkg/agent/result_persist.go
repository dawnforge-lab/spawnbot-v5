package agent

import (
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools/resultstore"
)

// maybePersistResult checks whether a tool result exceeds the per-tool size
// threshold or the per-turn aggregate budget. If either triggers, the full
// result is persisted to disk and result.ForLLM is replaced with a preview.
// turnBudgetUsed is updated to reflect the size added this call.
func maybePersistResult(
	store *resultstore.ResultStore,
	cfg config.ToolResultPersistenceConfig,
	toolName string,
	toolCallID string,
	result *tools.ToolResult,
	turnBudgetUsed *int,
) error {
	content := result.ForLLM

	// Never persist errors or empty results
	if result.IsError || content == "" {
		*turnBudgetUsed += len(content)
		return nil
	}

	// Determine threshold for this tool
	threshold := cfg.DefaultMaxChars
	if override, ok := cfg.ToolOverrides[toolName]; ok {
		threshold = override
	}

	// Check individual size and aggregate budget
	overIndividual := len(content) > threshold
	overBudget := *turnBudgetUsed+len(content) > cfg.PerTurnBudgetChars

	if overIndividual || overBudget {
		persisted, err := store.Persist(toolCallID, content, cfg.PreviewSizeBytes)
		if err != nil {
			return err
		}
		result.ForLLM = persisted.FormatPreviewMessage()
	}

	*turnBudgetUsed += len(result.ForLLM)
	return nil
}
