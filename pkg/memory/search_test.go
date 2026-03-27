//go:build cgo

package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbedder always returns a fixed embedding.
type mockEmbedder struct {
	dims int
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	emb := make([]float32, m.dims)
	for i := range emb {
		emb[i] = 0.5
	}
	return emb, nil
}

func (m *mockEmbedder) Dimensions() int { return m.dims }

// failingEmbedder always returns an error.
type failingEmbedder struct {
	dims int
}

func (m *failingEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, errors.New("embedding service unavailable")
}

func (m *failingEmbedder) Dimensions() int { return m.dims }

func setupTestStoreWithData(t *testing.T) (*SQLiteStore, *mockEmbedder) {
	t.Helper()
	dims := 4
	store, err := NewSQLiteStore(t.TempDir(), dims)
	require.NoError(t, err)

	embedder := &mockEmbedder{dims: dims}

	chunks := []struct {
		content string
		emb     []float32
	}{
		{"Go is a compiled programming language", []float32{0.9, 0.1, 0.0, 0.0}},
		{"Python is an interpreted language", []float32{0.8, 0.2, 0.0, 0.0}},
		{"Rust focuses on memory safety", []float32{0.7, 0.3, 0.0, 0.0}},
		{"SQL is used for databases", []float32{0.0, 0.0, 0.9, 0.1}},
		{"JavaScript runs in browsers", []float32{0.0, 0.0, 0.8, 0.2}},
	}

	for _, c := range chunks {
		err := store.StoreWithEmbedding(Chunk{Content: c.content, SourceFile: "test.md"}, c.emb)
		require.NoError(t, err)
	}

	return store, embedder
}

func TestHybridSearch_ReturnsResults(t *testing.T) {
	store, embedder := setupTestStoreWithData(t)
	defer store.Close()

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), "programming language", 5)
	require.NoError(t, err)
	assert.True(t, len(results) > 0, "expected at least one result")
}

func TestHybridSearch_ResultsAreSorted(t *testing.T) {
	store, embedder := setupTestStoreWithData(t)
	defer store.Close()

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), "compiled language", 5)
	require.NoError(t, err)
	require.True(t, len(results) >= 2, "expected at least 2 results")
	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score,
			"result %d (score=%.4f) should be >= result %d (score=%.4f)",
			i-1, results[i-1].Score, i, results[i].Score)
	}
}

func TestHybridSearch_EmptyQuery(t *testing.T) {
	store, embedder := setupTestStoreWithData(t)
	defer store.Close()

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), "", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestHybridSearch_LimitRespected(t *testing.T) {
	store, embedder := setupTestStoreWithData(t)
	defer store.Close()

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), "language", 2)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 2, "expected at most 2 results")
}

func TestHybridSearch_DeduplicatesResults(t *testing.T) {
	store, embedder := setupTestStoreWithData(t)
	defer store.Close()

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), "language", 10)
	require.NoError(t, err)

	// Check that no chunk ID appears more than once
	seen := make(map[string]bool)
	for _, r := range results {
		assert.False(t, seen[r.ID], "duplicate chunk ID: %s", r.ID)
		seen[r.ID] = true
	}
}

func TestHybridSearch_FallsBackToFTSOnEmbedError(t *testing.T) {
	store, _ := setupTestStoreWithData(t)
	defer store.Close()

	failing := &failingEmbedder{dims: 4}
	searcher := NewHybridSearcher(store, failing)
	results, err := searcher.Search(context.Background(), "programming language", 5)
	require.NoError(t, err)
	assert.True(t, len(results) > 0, "should fall back to FTS-only results")
}

func TestHybridSearch_TemporalDecayFavorsNewer(t *testing.T) {
	// Verify the temporal decay function directly
	now := time.Now()
	recent := temporalDecay(now, now)
	monthAgo := temporalDecay(now.Add(-30*24*time.Hour), now)
	yearAgo := temporalDecay(now.Add(-365*24*time.Hour), now)

	assert.InDelta(t, 1.0, recent, 0.01, "recent should have decay ~1.0")
	assert.InDelta(t, 0.5, monthAgo, 0.01, "30-day-old should have decay ~0.5")
	assert.True(t, yearAgo < monthAgo, "older should have lower decay")
}

func TestHybridSearch_CustomWeights(t *testing.T) {
	store, embedder := setupTestStoreWithData(t)
	defer store.Close()

	// FTS-heavy: should still return results
	ftsSearcher := NewHybridSearcher(store, embedder, WithWeights(0.9, 0.1))
	ftsResults, err := ftsSearcher.Search(context.Background(), "programming language", 5)
	require.NoError(t, err)
	assert.True(t, len(ftsResults) > 0)

	// Vec-heavy: should still return results
	vecSearcher := NewHybridSearcher(store, embedder, WithWeights(0.1, 0.9))
	vecResults, err := vecSearcher.Search(context.Background(), "programming language", 5)
	require.NoError(t, err)
	assert.True(t, len(vecResults) > 0)
}

func TestHybridSearch_CustomK(t *testing.T) {
	store, embedder := setupTestStoreWithData(t)
	defer store.Close()

	searcher := NewHybridSearcher(store, embedder, WithK(30))
	results, err := searcher.Search(context.Background(), "programming language", 5)
	require.NoError(t, err)
	assert.True(t, len(results) > 0)
}
