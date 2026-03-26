package memory

import "time"

// Chunk represents a piece of indexed content from a markdown file.
type Chunk struct {
	ID          string
	SourceFile  string
	Heading     string
	Content     string
	ContentHash string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ScoredChunk is a Chunk with a relevance score from search.
type ScoredChunk struct {
	Chunk
	Score float64
}
