package council

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

// Store persists council sessions to disk.
// Each council is stored in its own directory: {dir}/{id}/
// with meta.json and transcript.jsonl files.
type Store struct {
	dir string
	mu  sync.RWMutex
}

// NewStore creates a new Store rooted at the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Create persists a new council session. It generates an ID like "council-{unixmilli}",
// creates the council directory, and writes meta.json. The generated ID is set on
// the meta struct and returned.
func (s *Store) Create(meta *CouncilMeta) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("council-%d", time.Now().UnixMilli())
	meta.ID = id

	councilDir := filepath.Join(s.dir, id)
	if err := os.MkdirAll(councilDir, 0o755); err != nil {
		logger.ErrorCF("council.store", "failed to create council dir", map[string]any{"id": id, "error": err.Error()})
		return "", fmt.Errorf("create council dir: %w", err)
	}

	if err := s.writeMeta(meta); err != nil {
		return "", err
	}

	return id, nil
}

// Load reads and returns the CouncilMeta for the given council ID.
func (s *Store) Load(id string) (*CouncilMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metaPath := filepath.Join(s.dir, id, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		logger.ErrorCF("council.store", "failed to read meta", map[string]any{"id": id, "error": err.Error()})
		return nil, fmt.Errorf("load council %s: %w", id, err)
	}

	var meta CouncilMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		logger.ErrorCF("council.store", "failed to parse meta", map[string]any{"id": id, "error": err.Error()})
		return nil, fmt.Errorf("parse council meta %s: %w", id, err)
	}

	return &meta, nil
}

// SaveMeta updates the meta.json for an existing council.
func (s *Store) SaveMeta(meta *CouncilMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeMeta(meta)
}

// writeMeta writes meta.json (must be called with lock held).
func (s *Store) writeMeta(meta *CouncilMeta) error {
	metaPath := filepath.Join(s.dir, meta.ID, "meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		logger.ErrorCF("council.store", "failed to marshal meta", map[string]any{"id": meta.ID, "error": err.Error()})
		return fmt.Errorf("marshal council meta: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		logger.ErrorCF("council.store", "failed to write meta", map[string]any{"id": meta.ID, "error": err.Error()})
		return fmt.Errorf("write council meta: %w", err)
	}

	return nil
}

// List returns all council metas sorted by UpdatedAt descending.
func (s *Store) List() ([]*CouncilMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		logger.ErrorCF("council.store", "failed to list councils", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("list councils: %w", err)
	}

	var metas []*CouncilMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(s.dir, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue // skip dirs without meta.json
		}
		var meta CouncilMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		metas = append(metas, &meta)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})

	return metas, nil
}

// Delete removes a council directory and all its contents.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	councilDir := filepath.Join(s.dir, id)
	if err := os.RemoveAll(councilDir); err != nil {
		logger.ErrorCF("council.store", "failed to delete council", map[string]any{"id": id, "error": err.Error()})
		return fmt.Errorf("delete council %s: %w", id, err)
	}

	return nil
}

// AppendMessage appends a transcript entry as a JSONL line to transcript.jsonl.
func (s *Store) AppendMessage(id string, entry TranscriptEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	transcriptPath := filepath.Join(s.dir, id, "transcript.jsonl")
	f, err := os.OpenFile(transcriptPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		logger.ErrorCF("council.store", "failed to open transcript", map[string]any{"id": id, "error": err.Error()})
		return fmt.Errorf("open transcript %s: %w", id, err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		logger.ErrorCF("council.store", "failed to marshal transcript entry", map[string]any{"id": id, "error": err.Error()})
		return fmt.Errorf("marshal transcript entry: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		logger.ErrorCF("council.store", "failed to write transcript entry", map[string]any{"id": id, "error": err.Error()})
		return fmt.Errorf("write transcript entry: %w", err)
	}

	return nil
}

// GetTranscript reads and returns all transcript entries for the given council.
func (s *Store) GetTranscript(id string) ([]TranscriptEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	transcriptPath := filepath.Join(s.dir, id, "transcript.jsonl")
	f, err := os.Open(transcriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		logger.ErrorCF("council.store", "failed to open transcript", map[string]any{"id": id, "error": err.Error()})
		return nil, fmt.Errorf("open transcript %s: %w", id, err)
	}
	defer f.Close()

	var entries []TranscriptEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry TranscriptEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			logger.ErrorCF("council.store", "failed to parse transcript line", map[string]any{"id": id, "error": err.Error()})
			return nil, fmt.Errorf("parse transcript line: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		logger.ErrorCF("council.store", "failed to scan transcript", map[string]any{"id": id, "error": err.Error()})
		return nil, fmt.Errorf("scan transcript %s: %w", id, err)
	}

	return entries, nil
}
