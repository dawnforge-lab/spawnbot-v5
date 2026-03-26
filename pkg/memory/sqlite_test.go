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

func TestSQLiteStore_VectorSearch(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4) // 4-dim for testing
	require.NoError(t, err)
	defer store.Close()

	embedding := []float32{0.1, 0.2, 0.3, 0.4}
	err = store.StoreWithEmbedding(Chunk{Content: "test content", SourceFile: "test.md", Heading: "Test"}, embedding)
	require.NoError(t, err)

	query := []float32{0.1, 0.2, 0.3, 0.5}
	results, err := store.SearchVec(query, 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test content", results[0].Content)
}

func TestSQLiteStore_VectorSearchMultiple(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	// Store three items with different embeddings
	store.StoreWithEmbedding(Chunk{Content: "cats are furry", SourceFile: "a.md"}, []float32{0.9, 0.1, 0.0, 0.0})
	store.StoreWithEmbedding(Chunk{Content: "dogs are loyal", SourceFile: "b.md"}, []float32{0.8, 0.2, 0.0, 0.0})
	store.StoreWithEmbedding(Chunk{Content: "fish swim in water", SourceFile: "c.md"}, []float32{0.0, 0.0, 0.9, 0.1})

	// Query closer to cats/dogs
	results, err := store.SearchVec([]float32{0.85, 0.15, 0.0, 0.0}, 2)
	require.NoError(t, err)
	require.Len(t, results, 2)
	// Closest match should be first
	assert.Contains(t, results[0].Content, "cats")
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
