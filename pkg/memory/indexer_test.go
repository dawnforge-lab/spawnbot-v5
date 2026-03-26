package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkMarkdown_ByHeadings(t *testing.T) {
	md := "# Title\nIntro text\n## Section A\nContent A\n## Section B\nContent B"
	chunks := ChunkMarkdown(md, "test.md")
	require.Len(t, chunks, 3)
	assert.Equal(t, "Title", chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "Intro text")
	assert.Equal(t, "Section A", chunks[1].Heading)
	assert.Contains(t, chunks[1].Content, "Content A")
	assert.Equal(t, "Section B", chunks[2].Heading)
}

func TestChunkMarkdown_NoHeadings(t *testing.T) {
	md := "Just plain text\nwith multiple lines"
	chunks := ChunkMarkdown(md, "test.md")
	require.Len(t, chunks, 1)
	assert.Empty(t, chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "Just plain text")
}

func TestChunkMarkdown_EmptyContent(t *testing.T) {
	chunks := ChunkMarkdown("", "test.md")
	assert.Empty(t, chunks)
}

func TestIndexer_IndexDirectory(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	indexer := NewIndexer(store, nil) // nil embedder
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("## Heading\nContent here"), 0644)

	changed, err := indexer.IndexDirectory(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, changed)

	// Verify stored
	results, err := store.SearchFTS("Content", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestIndexer_SkipsUnchanged(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	indexer := NewIndexer(store, nil)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("## Heading\nContent"), 0644)

	changed1, _ := indexer.IndexDirectory(dir)
	assert.Equal(t, 1, changed1)

	// Index again — nothing changed
	changed2, _ := indexer.IndexDirectory(dir)
	assert.Equal(t, 0, changed2)
}

func TestIndexer_DetectsChanges(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	indexer := NewIndexer(store, nil)
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	os.WriteFile(filePath, []byte("## Heading\nOriginal"), 0644)

	indexer.IndexDirectory(dir)

	// Modify file
	os.WriteFile(filePath, []byte("## Heading\nUpdated content"), 0644)

	changed, _ := indexer.IndexDirectory(dir)
	assert.Equal(t, 1, changed)
}

func TestIndexer_WalksSubdirectories(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir(), 4)
	require.NoError(t, err)
	defer store.Close()

	indexer := NewIndexer(store, nil)
	dir := t.TempDir()
	subdir := filepath.Join(dir, "202603")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(dir, "root.md"), []byte("## Root\nRoot content"), 0644)
	os.WriteFile(filepath.Join(subdir, "sub.md"), []byte("## Sub\nSub content"), 0644)

	changed, err := indexer.IndexDirectory(dir)
	require.NoError(t, err)
	assert.Equal(t, 2, changed)
}
