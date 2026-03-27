//go:build cgo

package memory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ChunkMarkdown splits markdown content into chunks at heading boundaries.
// Every line starting with one or more '#' characters begins a new chunk.
// Content before the first heading is emitted as a chunk with an empty heading.
// Empty chunks (no content after trimming) are discarded.
func ChunkMarkdown(content, sourceFile string) []Chunk {
	if content == "" {
		return nil
	}

	var chunks []Chunk
	var currentHeading string
	var currentLines []string
	inChunk := false

	flush := func() {
		text := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if text != "" {
			chunks = append(chunks, Chunk{
				SourceFile: sourceFile,
				Heading:    currentHeading,
				Content:    text,
			})
		}
	}

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "#") {
			if inChunk {
				flush()
			}
			// Strip leading '#' characters and one optional space to get the heading text.
			heading := strings.TrimLeft(line, "#")
			heading = strings.TrimPrefix(heading, " ")
			currentHeading = heading
			currentLines = nil
			inChunk = true
		} else {
			currentLines = append(currentLines, line)
			inChunk = true
		}
	}

	if inChunk {
		flush()
	}

	return chunks
}

// Indexer walks markdown directories, chunks files, and stores new/changed
// chunks in the SQLiteStore. It uses SHA-256 content hashes to skip unchanged
// chunks so re-indexing the same directory is a no-op when nothing changed.
type Indexer struct {
	store   *SQLiteStore
	embedder EmbeddingProvider // may be nil
}

// NewIndexer creates an Indexer backed by store. embedder may be nil, in
// which case chunks are stored via FTS5 only (no vector embedding).
func NewIndexer(store *SQLiteStore, embedder EmbeddingProvider) *Indexer {
	return &Indexer{store: store, embedder: embedder}
}

// IndexDirectory recursively walks dir for .md files, chunks each file, and
// stores any chunk whose content hash is not yet present. It returns the
// number of chunks that were newly stored.
func (ix *Indexer) IndexDirectory(dir string) (int, error) {
	var changed int

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		chunks := ChunkMarkdown(string(data), path)
		for _, chunk := range chunks {
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(chunk.Content)))

			exists, err := ix.store.HasContentHash(hash)
			if err != nil {
				return fmt.Errorf("check hash for %s: %w", path, err)
			}
			if exists {
				continue
			}

			if ix.embedder != nil {
				embedding, err := ix.embedder.Embed(context.Background(), chunk.Content)
				if err != nil {
					return fmt.Errorf("embed chunk from %s: %w", path, err)
				}
				if err := ix.store.StoreWithEmbedding(chunk, embedding); err != nil {
					return fmt.Errorf("store chunk with embedding from %s: %w", path, err)
				}
			} else {
				if err := ix.store.Store(chunk); err != nil {
					return fmt.Errorf("store chunk from %s: %w", path, err)
				}
			}
			changed++
		}
		return nil
	})

	return changed, err
}
