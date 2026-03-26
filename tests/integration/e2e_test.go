package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agent"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_FullFlow(t *testing.T) {
	// 1. Setup: temp workspace mimicking post-onboarding state
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "SOUL.md"),
		[]byte("# Spawnbot\nYou are Spawnbot, a personal AI agent for Alice."), 0644)
	os.WriteFile(filepath.Join(workspace, "USER.md"),
		[]byte("# User\nName: Alice\nPrefers Go over Python"), 0644)
	os.WriteFile(filepath.Join(workspace, "GOALS.md"),
		[]byte("# Goals\n## Active\n- Learn about the codebase"), 0644)

	// 2. Verify SOUL.md loads into system prompt
	cb := agent.NewContextBuilder(workspace)
	content, err := cb.LoadBootstrapFiles()
	require.NoError(t, err)
	assert.Contains(t, content, "Spawnbot")
	assert.Contains(t, content, "Alice")

	// 3. Initialize memory store
	memDir := filepath.Join(workspace, "memory")
	os.MkdirAll(memDir, 0755)
	store, err := memory.NewSQLiteStore(memDir, 4)
	require.NoError(t, err)
	defer store.Close()

	// 4. Store memories
	err = store.Store(memory.Chunk{
		Content:    "The user prefers Go over Python for backend development",
		SourceFile: "USER.md",
		Heading:    "Preferences",
	})
	require.NoError(t, err)

	err = store.Store(memory.Chunk{
		Content:    "The project uses SQLite with FTS5 for full-text search",
		SourceFile: "notes.md",
		Heading:    "Architecture",
	})
	require.NoError(t, err)

	// 5. Search via FTS
	results, err := store.SearchFTS("Go Python", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "Go over Python")

	// 6. Search via FTS — different query
	results2, err := store.SearchFTS("SQLite FTS5", 5)
	require.NoError(t, err)
	require.Len(t, results2, 1)
	assert.Contains(t, results2[0].Content, "full-text search")

	// 7. Index markdown files
	indexer := memory.NewIndexer(store, nil)
	os.WriteFile(filepath.Join(memDir, "test-notes.md"),
		[]byte("## Meeting Notes\nDiscussed deployment strategy\n## Action Items\nSet up CI pipeline"), 0644)

	changed, err := indexer.IndexDirectory(memDir)
	require.NoError(t, err)
	assert.Equal(t, 2, changed) // Two headings = two chunks

	// 8. Verify indexed content is searchable
	results3, err := store.SearchFTS("deployment", 5)
	require.NoError(t, err)
	assert.True(t, len(results3) > 0)

	// 9. Recall by source
	recalled, err := store.RecallBySource("USER.md", "", 10)
	require.NoError(t, err)
	assert.Len(t, recalled, 1)

	// 10. Memory tools work
	storeTool := memory.NewMemoryStoreTool(store, nil)
	result := storeTool.Execute(context.Background(), map[string]any{
		"content": "Integration test passed successfully",
		"heading": "Test Results",
	})
	assert.Nil(t, result.Err)

	searchTool := memory.NewMemorySearchTool(store, nil)
	searchResult := searchTool.Execute(context.Background(), map[string]any{
		"query": "integration test",
	})
	assert.Nil(t, searchResult.Err)
	assert.Contains(t, searchResult.ForLLM, "Integration test")
}
