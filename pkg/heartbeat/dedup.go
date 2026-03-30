package heartbeat

import (
	"crypto/sha256"
	"sync"
	"time"
)

// Dedup tracks recently sent heartbeat messages to avoid sending identical
// alerts within a configurable TTL window.
type Dedup struct {
	ttl  time.Duration
	mu   sync.Mutex
	seen map[[32]byte]time.Time
}

// NewDedup creates a deduplicator with the given TTL.
func NewDedup(ttl time.Duration) *Dedup {
	return &Dedup{
		ttl:  ttl,
		seen: make(map[[32]byte]time.Time),
	}
}

// IsDuplicate returns true if the same content was seen within the TTL window.
// Empty strings are never considered duplicates.
func (d *Dedup) IsDuplicate(content string) bool {
	if content == "" {
		return false
	}

	hash := sha256.Sum256([]byte(content))
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Prune expired entries opportunistically
	for k, v := range d.seen {
		if now.Sub(v) > d.ttl {
			delete(d.seen, k)
		}
	}

	if sentAt, ok := d.seen[hash]; ok {
		if now.Sub(sentAt) <= d.ttl {
			return true
		}
	}

	d.seen[hash] = now
	return false
}
