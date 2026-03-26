package memory

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"
)

// HybridSearcher combines FTS5 keyword search and vector similarity search
// using Reciprocal Rank Fusion (RRF) with temporal decay.
type HybridSearcher struct {
	store     *SQLiteStore
	embedder  EmbeddingProvider
	weightFTS float64 // weight for FTS results in RRF
	weightVec float64 // weight for vector results in RRF
	k         float64 // RRF constant (default 60)
}

// Option configures a HybridSearcher.
type Option func(*HybridSearcher)

// WithWeights sets the FTS and vector weights for RRF scoring.
// Both weights should be positive; they do not need to sum to 1.
func WithWeights(fts, vec float64) Option {
	return func(h *HybridSearcher) {
		h.weightFTS = fts
		h.weightVec = vec
	}
}

// WithK sets the RRF constant k. Higher values reduce the influence of
// high-ranking documents. The standard default is 60.
func WithK(k float64) Option {
	return func(h *HybridSearcher) {
		h.k = k
	}
}

// NewHybridSearcher creates a new hybrid searcher that fuses FTS5 and
// vector similarity results using Reciprocal Rank Fusion.
func NewHybridSearcher(store *SQLiteStore, embedder EmbeddingProvider, opts ...Option) *HybridSearcher {
	h := &HybridSearcher{
		store:     store,
		embedder:  embedder,
		weightFTS: 0.5,
		weightVec: 0.5,
		k:         60,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Search performs a hybrid search combining FTS5 keyword matching and
// vector similarity, fused with RRF and weighted by temporal decay.
//
// If the embedding provider fails, it falls back to FTS-only results.
func (h *HybridSearcher) Search(ctx context.Context, query string, limit int) ([]ScoredChunk, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	// Over-fetch from each source to improve fusion quality.
	fetchLimit := limit * 2

	// --- FTS5 keyword search ---
	ftsResults, ftsErr := h.store.SearchFTS(query, fetchLimit)
	if ftsErr != nil {
		// FTS query syntax errors are non-fatal; log and continue
		// with vector-only results if available.
		log.Printf("hybrid search: FTS query failed: %v", ftsErr)
		ftsResults = nil
	}

	// --- Vector similarity search ---
	var vecResults []ScoredChunk
	embedding, embedErr := h.embedder.Embed(ctx, query)
	if embedErr != nil {
		// Embedding failed — fall back to FTS-only.
		log.Printf("hybrid search: embedding failed, falling back to FTS-only: %v", embedErr)
	} else {
		var vecErr error
		vecResults, vecErr = h.store.SearchVec(embedding, fetchLimit)
		if vecErr != nil {
			log.Printf("hybrid search: vec query failed: %v", vecErr)
			vecResults = nil
		}
	}

	// If both sources failed, we have nothing.
	if len(ftsResults) == 0 && len(vecResults) == 0 {
		return nil, nil
	}

	now := time.Now()

	// Build a map of chunk ID -> fused result.
	type fusedEntry struct {
		chunk   Chunk
		ftsRank int // 1-based rank in FTS results; 0 = absent
		vecRank int // 1-based rank in vec results; 0 = absent
	}
	entries := make(map[string]*fusedEntry)
	missingRank := fetchLimit + 1 // rank assigned to absent results

	// Record FTS ranks.
	for i, ch := range ftsResults {
		entries[ch.ID] = &fusedEntry{
			chunk:   ch,
			ftsRank: i + 1,
		}
	}

	// Record vec ranks.
	for i, sc := range vecResults {
		if e, ok := entries[sc.ID]; ok {
			e.vecRank = i + 1
		} else {
			entries[sc.ID] = &fusedEntry{
				chunk:   sc.Chunk,
				vecRank: i + 1,
			}
		}
	}

	// Compute RRF score with temporal decay for each chunk.
	scored := make([]ScoredChunk, 0, len(entries))
	for _, e := range entries {
		ftsR := e.ftsRank
		if ftsR == 0 {
			ftsR = missingRank
		}
		vecR := e.vecRank
		if vecR == 0 {
			vecR = missingRank
		}

		rrfScore := (h.weightFTS / (h.k + float64(ftsR))) + (h.weightVec / (h.k + float64(vecR)))
		decay := temporalDecay(e.chunk.UpdatedAt, now)
		finalScore := rrfScore * decay

		scored = append(scored, ScoredChunk{
			Chunk: e.chunk,
			Score: finalScore,
		})
	}

	// Sort by score descending.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Trim to requested limit.
	if len(scored) > limit {
		scored = scored[:limit]
	}

	return scored, nil
}

// temporalDecay computes a decay factor for a chunk based on its age.
// Returns a value in (0, 1] where newer items get a value closer to 1.
//
// Formula: 1.0 / (1.0 + ageDays/30.0)
//
//   - Now:        1.0
//   - 30 days:   ~0.5
//   - 365 days:  ~0.076
func temporalDecay(updatedAt time.Time, now time.Time) float64 {
	ageDays := now.Sub(updatedAt).Hours() / 24.0
	if ageDays < 0 {
		ageDays = 0
	}
	return 1.0 / (1.0 + ageDays/30.0)
}
