package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

// persistedWaiter mirrors eventWaiter for on-disk serialization. Times are
// stored as Unix-ms so the schema is language/timezone neutral.
type persistedWaiter struct {
	ID             uint64 `json:"id"`
	Name           string `json:"name"`
	SessionKey     string `json:"session_key"`
	AgentID        string `json:"agent_id"`
	Channel        string `json:"channel"`
	ChatID         string `json:"chat_id"`
	Intent         string `json:"intent"`
	Reason         string `json:"reason"`
	DeadlineUnixMs int64  `json:"deadline_unix_ms,omitempty"`
	CreatedUnixMs  int64  `json:"created_unix_ms"`
}

type persistedEventState struct {
	Seq     uint64            `json:"seq"`
	Waiters []persistedWaiter `json:"waiters"`
}

// eventsPersistence bundles the persistence state for AgentLoop. Held via
// pointer on AgentLoop so zero-value means "persistence disabled".
type eventsPersistence struct {
	path string
	mu   sync.Mutex
}

// SetEventsStorePath enables persistence of event waiters by writing to the
// given file. Call once during startup before registering waiters. An empty
// path disables persistence.
func (al *AgentLoop) SetEventsStorePath(path string) {
	if path == "" {
		al.eventsStore = nil
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.WarnCF("agent", "Failed to create events store directory",
			map[string]any{"path": path, "error": err.Error()})
		return
	}
	al.eventsStore = &eventsPersistence{path: path}
}

// loadEventWaitersFromDisk reads persisted waiters into the in-memory
// registry. Expired deadlines are resolved after a short grace period so the
// Run loop has time to start. Safe to call from NewAgentLoop; no-op if
// persistence is disabled or the file does not exist yet.
func (al *AgentLoop) loadEventWaitersFromDisk() {
	if al.eventsStore == nil {
		return
	}
	raw, err := os.ReadFile(al.eventsStore.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.WarnCF("agent", "Failed to read events store",
				map[string]any{"path": al.eventsStore.path, "error": err.Error()})
		}
		return
	}
	var state persistedEventState
	if err := json.Unmarshal(raw, &state); err != nil {
		logger.WarnCF("agent", "Failed to parse events store; ignoring",
			map[string]any{"path": al.eventsStore.path, "error": err.Error()})
		return
	}

	if state.Seq > 0 {
		// Ensure newly issued IDs do not collide with restored ones.
		current := al.eventWaiterSeq.Load()
		if state.Seq > current {
			al.eventWaiterSeq.Store(state.Seq)
		}
	}

	restored := 0
	expired := 0
	now := time.Now()
	// Short grace so expired-on-startup resumptions have time to reach Run.
	const startupGrace = 2 * time.Second
	for _, p := range state.Waiters {
		w := &eventWaiter{
			id:         p.ID,
			name:       p.Name,
			sessionKey: p.SessionKey,
			agentID:    p.AgentID,
			channel:    p.Channel,
			chatID:     p.ChatID,
			intent:     p.Intent,
			reason:     p.Reason,
			createdAt:  time.UnixMilli(p.CreatedUnixMs),
		}
		if p.DeadlineUnixMs > 0 {
			w.deadline = time.UnixMilli(p.DeadlineUnixMs)
		}

		bucket := al.eventsBucket(w.name)
		bucket.mu.Lock()
		bucket.waiters = append(bucket.waiters, w)
		bucket.mu.Unlock()

		if !w.deadline.IsZero() && !w.deadline.After(now) {
			// Expired while offline. Schedule a late timeout resume so the
			// run loop has spun up by the time we fire it.
			expired++
			go func(waiter *eventWaiter) {
				time.Sleep(startupGrace)
				al.timeoutEventWaiter(waiter)
			}(w)
		} else if !w.deadline.IsZero() {
			go al.watchEventWaiterDeadline(w)
		}
		restored++
	}

	logger.InfoCF("agent", "Restored event waiters from disk",
		map[string]any{
			"path":     al.eventsStore.path,
			"restored": restored,
			"expired":  expired,
		})
}

// saveEventWaitersSnapshot writes the full current registry to disk. Called
// at the tail of every mutating operation (register, fire, timeout). Writes
// are serialized via a per-store mutex and use atomic rename to avoid torn
// reads. No-op if persistence is disabled.
func (al *AgentLoop) saveEventWaitersSnapshot() {
	store := al.eventsStore
	if store == nil {
		return
	}

	state := persistedEventState{
		Seq: al.eventWaiterSeq.Load(),
	}
	al.events.Range(func(_, value any) bool {
		bucket, _ := value.(*eventBucket)
		if bucket == nil {
			return true
		}
		bucket.mu.Lock()
		for _, w := range bucket.waiters {
			p := persistedWaiter{
				ID:            w.id,
				Name:          w.name,
				SessionKey:    w.sessionKey,
				AgentID:       w.agentID,
				Channel:       w.channel,
				ChatID:        w.chatID,
				Intent:        w.intent,
				Reason:        w.reason,
				CreatedUnixMs: w.createdAt.UnixMilli(),
			}
			if !w.deadline.IsZero() {
				p.DeadlineUnixMs = w.deadline.UnixMilli()
			}
			state.Waiters = append(state.Waiters, p)
		}
		bucket.mu.Unlock()
		return true
	})

	store.mu.Lock()
	defer store.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		logger.WarnCF("agent", "Failed to marshal events store",
			map[string]any{"error": err.Error()})
		return
	}

	tmp := store.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		logger.WarnCF("agent", "Failed to write events store tmp",
			map[string]any{"path": tmp, "error": err.Error()})
		return
	}
	if err := os.Rename(tmp, store.path); err != nil {
		logger.WarnCF("agent", "Failed to rename events store",
			map[string]any{"path": store.path, "error": err.Error()})
		_ = os.Remove(tmp)
	}
}
