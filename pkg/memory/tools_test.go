//go:build cgo

package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStoreTool_Execute(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	tool := NewMemoryStoreTool(store, nil)
	result := tool.Execute(context.Background(), map[string]any{
		"content": "The user prefers dark mode",
		"heading": "Preferences",
	})
	assert.Nil(t, result.Err)
	assert.Contains(t, result.ForLLM, "stored")

	// Verify it was actually stored
	results, err := store.SearchFTS("dark mode", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMemorySearchTool_Execute(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	store.Store(Chunk{Content: "Go is a compiled language", SourceFile: "manual", Heading: "Languages"})

	tool := NewMemorySearchTool(store, nil) // nil embedder = FTS only
	result := tool.Execute(context.Background(), map[string]any{
		"query": "compiled language",
	})
	assert.Nil(t, result.Err)
	assert.Contains(t, result.ForLLM, "compiled")
}

func TestMemoryRecallTool_Execute(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	store.Store(Chunk{Content: "Important fact", SourceFile: "notes.md", Heading: "Facts"})
	store.Store(Chunk{Content: "Another fact", SourceFile: "notes.md", Heading: "More Facts"})
	store.Store(Chunk{Content: "Unrelated", SourceFile: "other.md", Heading: "Other"})

	tool := NewMemoryRecallTool(store)
	result := tool.Execute(context.Background(), map[string]any{
		"source_file": "notes.md",
	})
	assert.Nil(t, result.Err)
	assert.Contains(t, result.ForLLM, "Important fact")
	assert.Contains(t, result.ForLLM, "Another fact")
	assert.NotContains(t, result.ForLLM, "Unrelated")
}
