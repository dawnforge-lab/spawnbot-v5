package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStore_StoreAndSearchFTS(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 768)
	require.NoError(t, err)
	defer store.Close()

	err = store.Store(Chunk{Content: "Go is a compiled language", SourceFile: "test.md", Heading: "Languages"})
	require.NoError(t, err)

	results, err := store.SearchFTS("compiled language", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "compiled")
}

func TestSQLiteStore_Dedup(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 768)
	require.NoError(t, err)
	defer store.Close()

	chunk := Chunk{Content: "same content", SourceFile: "test.md", Heading: "Test"}
	err = store.Store(chunk)
	require.NoError(t, err)

	// Store same content again — should not create duplicate
	err = store.Store(chunk)
	require.NoError(t, err)

	results, err := store.SearchFTS("same content", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1) // Only one result, not two
}

func TestSQLiteStore_StoreAndClose(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir, 768)
	require.NoError(t, err)

	err = store.Store(Chunk{Content: "persistent data", SourceFile: "test.md"})
	require.NoError(t, err)
	store.Close()

	// Reopen and verify data persisted
	store2, err := NewSQLiteStore(dir, 768)
	require.NoError(t, err)
	defer store2.Close()

	results, err := store2.SearchFTS("persistent", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}
