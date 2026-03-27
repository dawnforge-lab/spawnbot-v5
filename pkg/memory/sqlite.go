//go:build cgo

package memory

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/oklog/ulid/v2"
)

func init() {
	// Register sqlite-vec as an auto-extension so all new connections
	// get the vec0 virtual-table module.
	sqliteVecAuto()

	// Register under a distinct driver name so we don't conflict with
	// modernc.org/sqlite (registers as "sqlite") or mattn's default
	// registration (registers as "sqlite3").
	sql.Register("sqlite3_memory", &sqlite3.SQLiteDriver{})
}

// SQLiteStore is a chunk store backed by SQLite with FTS5 full-text search.
type SQLiteStore struct {
	db            *sql.DB
	dbPath        string
	vecDimensions int
}

// NewSQLiteStore creates or opens a SQLite database at dbDir/spawnbot.db.
// vecDimensions is stored for later use by vector search (Task 9).
func NewSQLiteStore(dbDir string, vecDimensions int) (*SQLiteStore, error) {
	dbPath := filepath.Join(dbDir, "spawnbot.db")

	// Open using our dedicated "sqlite3_memory" driver (mattn/go-sqlite3 CGO).
	db, err := sql.Open("sqlite3_memory", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Verify connectivity.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &SQLiteStore{
		db:            db,
		dbPath:        dbPath,
		vecDimensions: vecDimensions,
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// migrate creates the required tables and triggers if they don't exist.
func (s *SQLiteStore) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS memory_chunks (
    id           TEXT PRIMARY KEY,
    source_file  TEXT NOT NULL DEFAULT '',
    heading      TEXT NOT NULL DEFAULT '',
    content      TEXT NOT NULL,
    content_hash TEXT NOT NULL UNIQUE,
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
    content,
    content='memory_chunks',
    content_rowid='rowid'
);

-- Triggers to keep FTS index in sync with the chunks table.
CREATE TRIGGER IF NOT EXISTS memory_fts_insert AFTER INSERT ON memory_chunks BEGIN
    INSERT INTO memory_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS memory_fts_delete AFTER DELETE ON memory_chunks BEGIN
    INSERT INTO memory_fts(memory_fts, rowid, content) VALUES('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS memory_fts_update AFTER UPDATE ON memory_chunks BEGIN
    INSERT INTO memory_fts(memory_fts, rowid, content) VALUES('delete', old.rowid, old.content);
    INSERT INTO memory_fts(rowid, content) VALUES (new.rowid, new.content);
END;
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Create the vec0 virtual table for vector search.
	_, err = s.db.Exec(fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS memory_vec USING vec0(
		chunk_id TEXT PRIMARY KEY,
		embedding float[%d]
	)`, s.vecDimensions))
	return err
}

// Store inserts a chunk into the database. Deduplicates by content hash
// (INSERT OR IGNORE on the content_hash UNIQUE constraint).
func (s *SQLiteStore) Store(chunk Chunk) error {
	now := time.Now().UTC()

	// Generate ULID for the primary key.
	id, err := ulid.New(ulid.Timestamp(now), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ulid: %w", err)
	}

	// Compute SHA-256 hash of the content for dedup.
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(chunk.Content)))

	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO memory_chunks (id, source_file, heading, content, content_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.String(), chunk.SourceFile, chunk.Heading, chunk.Content, hash, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert chunk: %w", err)
	}

	return nil
}

// SearchFTS performs a full-text search using FTS5 MATCH and returns up to
// limit results ordered by relevance (BM25 rank).
func (s *SQLiteStore) SearchFTS(query string, limit int) ([]Chunk, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.source_file, c.heading, c.content, c.content_hash, c.created_at, c.updated_at
		FROM memory_fts f
		JOIN memory_chunks c ON c.rowid = f.rowid
		WHERE memory_fts MATCH ?
		ORDER BY rank
		LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()

	var results []Chunk
	for rows.Next() {
		var ch Chunk
		if err := rows.Scan(&ch.ID, &ch.SourceFile, &ch.Heading, &ch.Content, &ch.ContentHash, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		results = append(results, ch)
	}
	return results, rows.Err()
}

// StoreWithEmbedding inserts a chunk and its embedding vector atomically.
// The chunk is stored via Store (FTS5-indexed), then the embedding is
// inserted into the vec0 virtual table.
func (s *SQLiteStore) StoreWithEmbedding(chunk Chunk, embedding []float32) error {
	now := time.Now().UTC()

	// Generate ULID for the primary key.
	id, err := ulid.New(ulid.Timestamp(now), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ulid: %w", err)
	}

	// Compute SHA-256 hash of the content for dedup.
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(chunk.Content)))

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	idStr := id.String()

	// Insert the chunk (with dedup on content_hash).
	res, err := tx.Exec(`
		INSERT OR IGNORE INTO memory_chunks (id, source_file, heading, content, content_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		idStr, chunk.SourceFile, chunk.Heading, chunk.Content, hash, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert chunk: %w", err)
	}

	// If the chunk was a duplicate (OR IGNORE), look up its existing ID.
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		// Content already exists — look up the existing chunk's ID.
		err = tx.QueryRow(`SELECT id FROM memory_chunks WHERE content_hash = ?`, hash).Scan(&idStr)
		if err != nil {
			return fmt.Errorf("lookup existing chunk: %w", err)
		}
	}

	// Insert embedding into the vec0 table.
	serialized, err := serializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("serialize embedding: %w", err)
	}

	_, err = tx.Exec(`INSERT OR REPLACE INTO memory_vec (chunk_id, embedding) VALUES (?, ?)`,
		idStr, serialized,
	)
	if err != nil {
		return fmt.Errorf("insert embedding: %w", err)
	}

	return tx.Commit()
}

// SearchVec performs a vector similarity search and returns up to limit
// results ordered by distance (closest first).
func (s *SQLiteStore) SearchVec(queryEmbedding []float32, limit int) ([]ScoredChunk, error) {
	serialized, err := serializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("serialize query embedding: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT c.id, c.source_file, c.heading, c.content, c.content_hash, c.created_at, c.updated_at, v.distance
		FROM memory_vec v
		JOIN memory_chunks c ON c.id = v.chunk_id
		WHERE v.embedding MATCH ? AND k = ?
		ORDER BY v.distance`,
		serialized, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("vec query: %w", err)
	}
	defer rows.Close()

	var results []ScoredChunk
	for rows.Next() {
		var sc ScoredChunk
		if err := rows.Scan(&sc.ID, &sc.SourceFile, &sc.Heading, &sc.Content, &sc.ContentHash, &sc.CreatedAt, &sc.UpdatedAt, &sc.Score); err != nil {
			return nil, fmt.Errorf("scan scored chunk: %w", err)
		}
		results = append(results, sc)
	}
	return results, rows.Err()
}

// HasContentHash reports whether a chunk with the given SHA-256 content hash
// already exists in the store.
func (s *SQLiteStore) HasContentHash(hash string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_chunks WHERE content_hash = ?)`, hash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check content hash: %w", err)
	}
	return exists, nil
}

// RecallBySource returns up to limit chunks matching the given sourceFile and/or
// heading. Empty strings are treated as wildcards (not filtered).
func (s *SQLiteStore) RecallBySource(sourceFile, heading string, limit int) ([]Chunk, error) {
	query := `
		SELECT id, source_file, heading, content, content_hash, created_at, updated_at
		FROM memory_chunks
		WHERE 1=1`
	args := []any{}

	if sourceFile != "" {
		query += " AND source_file = ?"
		args = append(args, sourceFile)
	}
	if heading != "" {
		query += " AND heading = ?"
		args = append(args, heading)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("recall query: %w", err)
	}
	defer rows.Close()

	var results []Chunk
	for rows.Next() {
		var ch Chunk
		if err := rows.Scan(&ch.ID, &ch.SourceFile, &ch.Heading, &ch.Content, &ch.ContentHash, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		results = append(results, ch)
	}
	return results, rows.Err()
}

// DBPath returns the filesystem path to the SQLite database file.
func (s *SQLiteStore) DBPath() string {
	return s.dbPath
}

// RecentChunks returns up to limit chunks ordered by creation time (newest first).
func (s *SQLiteStore) RecentChunks(limit int) ([]Chunk, error) {
	rows, err := s.db.Query(`
		SELECT id, source_file, heading, content, content_hash, created_at, updated_at
		FROM memory_chunks
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent chunks query: %w", err)
	}
	defer rows.Close()

	var results []Chunk
	for rows.Next() {
		var ch Chunk
		if err := rows.Scan(&ch.ID, &ch.SourceFile, &ch.Heading, &ch.Content, &ch.ContentHash, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		results = append(results, ch)
	}
	return results, rows.Err()
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
