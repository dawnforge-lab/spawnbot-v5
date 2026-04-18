package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers"
)

// ContinuationKind enumerates what the agent wants to happen after a turn
// ends. Declared by the model via the end_turn tool.
type ContinuationKind string

const (
	ContinuationDone        ContinuationKind = "done"
	ContinuationContinueNow ContinuationKind = "continue_now"
	ContinuationWait        ContinuationKind = "wait"
	ContinuationSchedule    ContinuationKind = "schedule"
	ContinuationAwaitEvent  ContinuationKind = "await_event"
)

// Continuation describes a post-turn action declared by the model.
type Continuation struct {
	Kind    ContinuationKind
	AfterMS int64
	At      time.Time
	Event   string
	Intent  string
	Reason  string
}

// maxAutoContinueDepth caps consecutive self-triggered continuations per
// session so a runaway chain cannot spin forever. Reset on each real inbound
// message.
const maxAutoContinueDepth = 5

// SelfContinueMarker prefixes self-injected steering messages so they are
// distinguishable in transcripts and logs.
const SelfContinueMarker = "[self-continue] "

// dispatchContinuation executes the side effect described by the declared
// continuation. Call AFTER runTurn has returned and ts.Finish has been called.
// No-op for nil / Done.
func (al *AgentLoop) dispatchContinuation(
	ctx context.Context,
	agent *AgentInstance,
	opts processOptions,
	cont *Continuation,
) {
	if cont == nil || cont.Kind == "" || cont.Kind == ContinuationDone {
		return
	}
	if agent == nil {
		return
	}

	sessionKey := opts.SessionKey
	if sessionKey == "" {
		return
	}

	depth := al.autoContinueCount(sessionKey)
	if depth >= maxAutoContinueDepth {
		logger.WarnCF("agent", "Skipping declared continuation; depth cap reached",
			map[string]any{
				"session_key": sessionKey,
				"kind":        string(cont.Kind),
				"depth":       depth,
				"cap":         maxAutoContinueDepth,
				"intent":      cont.Intent,
			})
		return
	}

	intent := strings.TrimSpace(cont.Intent)
	if intent == "" {
		intent = "(no explicit intent provided; resume working toward the goal you set yourself)"
	}

	switch cont.Kind {
	case ContinuationContinueNow:
		al.enqueueSelfContinuation(sessionKey, agent.ID, intent, cont.Reason)
	case ContinuationWait, ContinuationSchedule:
		delay := continuationDelay(cont)
		if delay <= 0 {
			al.enqueueSelfContinuation(sessionKey, agent.ID, intent, cont.Reason)
			return
		}
		go al.scheduleSelfContinuation(agent, sessionKey, opts.Channel, opts.ChatID, intent, cont.Reason, delay)
	case ContinuationAwaitEvent:
		al.registerEventWaiter(cont, opts, agent.ID, intent)
	default:
		logger.WarnCF("agent", "Unknown continuation kind",
			map[string]any{
				"session_key": sessionKey,
				"kind":        string(cont.Kind),
			})
	}
}

func (al *AgentLoop) enqueueSelfContinuation(sessionKey, agentID, intent, reason string) {
	content := SelfContinueMarker + intent
	if reason != "" {
		content += "\n(reason: " + reason + ")"
	}
	msg := providers.Message{Role: "user", Content: content}
	if err := al.enqueueSteeringMessage(sessionKey, agentID, msg); err != nil {
		logger.WarnCF("agent", "Failed to enqueue self continuation",
			map[string]any{
				"session_key": sessionKey,
				"error":       err.Error(),
			})
		return
	}
	next := al.incAutoContinueCount(sessionKey)
	logger.InfoCF("agent", "Self continuation enqueued",
		map[string]any{
			"session_key": sessionKey,
			"depth":       next,
			"intent_len":  len(intent),
		})
}

func (al *AgentLoop) scheduleSelfContinuation(
	agent *AgentInstance,
	sessionKey, channel, chatID, intent, reason string,
	delay time.Duration,
) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	<-timer.C

	al.enqueueSelfContinuation(sessionKey, agent.ID, intent, reason)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if _, err := al.Continue(ctx, sessionKey, channel, chatID); err != nil {
		logger.WarnCF("agent", "Scheduled self continuation failed",
			map[string]any{
				"session_key": sessionKey,
				"error":       err.Error(),
			})
	}
}

func continuationDelay(cont *Continuation) time.Duration {
	switch cont.Kind {
	case ContinuationWait:
		if cont.AfterMS > 0 {
			return time.Duration(cont.AfterMS) * time.Millisecond
		}
	case ContinuationSchedule:
		if !cont.At.IsZero() {
			d := time.Until(cont.At)
			if d > 0 {
				return d
			}
		}
		if cont.AfterMS > 0 {
			return time.Duration(cont.AfterMS) * time.Millisecond
		}
	}
	return 0
}

func (al *AgentLoop) autoContinueCount(sessionKey string) int32 {
	v, ok := al.autoContinueCounts.Load(sessionKey)
	if !ok {
		return 0
	}
	counter, ok := v.(*atomic.Int32)
	if !ok {
		return 0
	}
	return counter.Load()
}

func (al *AgentLoop) incAutoContinueCount(sessionKey string) int32 {
	v, _ := al.autoContinueCounts.LoadOrStore(sessionKey, &atomic.Int32{})
	counter := v.(*atomic.Int32)
	return counter.Add(1)
}

func (al *AgentLoop) resetAutoContinueCount(sessionKey string) {
	al.autoContinueCounts.Delete(sessionKey)
}

// parseContinuationArgs validates and extracts a Continuation from the
// end_turn tool arguments. Unknown / missing fields are treated generously
// with a sensible default (Done) only when continuation itself is missing.
func parseContinuationArgs(args map[string]any) (*Continuation, error) {
	kindRaw, _ := args["continuation"].(string)
	kindRaw = strings.ToLower(strings.TrimSpace(kindRaw))
	if kindRaw == "" {
		return nil, fmt.Errorf("continuation is required")
	}
	kind := ContinuationKind(kindRaw)
	switch kind {
	case ContinuationDone, ContinuationContinueNow, ContinuationWait,
		ContinuationSchedule, ContinuationAwaitEvent:
	default:
		return nil, fmt.Errorf("unknown continuation %q", kindRaw)
	}
	cont := &Continuation{Kind: kind}

	if v, ok := args["intent"].(string); ok {
		cont.Intent = strings.TrimSpace(v)
	}
	if v, ok := args["reason"].(string); ok {
		cont.Reason = strings.TrimSpace(v)
	}
	if v, ok := args["event"].(string); ok {
		cont.Event = strings.TrimSpace(v)
	}
	if v, ok := argInt(args["after_ms"]); ok {
		cont.AfterMS = v
	}
	if v, ok := args["at"].(string); ok && strings.TrimSpace(v) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("invalid at timestamp: %w", err)
		}
		cont.At = t
	}

	return cont, nil
}

func argInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// ======================== Event waiter registry ========================
//
// When the model declares a continuation of kind await_event, we record a
// waiter and resume the turn with the declared intent once the named event
// fires. Events are fired via FireEvent (exported) or the fire_event tool.
// Registry is in-memory and per-AgentLoop; waiters do not survive restart.
//
// Fan-out: all waiters on a given name resolve when the event fires.
// Optional timeout: if the waiter carries deadline > 0, a timer fires a
// synthetic resumption with a "timed_out" marker so the turn never sits
// forever on a name that never arrives.

type eventWaiter struct {
	id         uint64
	name       string
	sessionKey string
	agentID    string
	channel    string
	chatID     string
	intent     string
	reason     string
	deadline   time.Time
	createdAt  time.Time
}

type eventBucket struct {
	mu      sync.Mutex
	waiters []*eventWaiter
}

func (al *AgentLoop) eventsBucket(name string) *eventBucket {
	v, _ := al.events.LoadOrStore(name, &eventBucket{})
	return v.(*eventBucket)
}

func normalizeEventName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func (al *AgentLoop) registerEventWaiter(
	cont *Continuation,
	opts processOptions,
	agentID, intent string,
) {
	name := normalizeEventName(cont.Event)
	if name == "" {
		logger.WarnCF("agent", "await_event continuation missing event name; skipping",
			map[string]any{
				"session_key": opts.SessionKey,
				"intent":      intent,
			})
		return
	}

	id := al.eventWaiterSeq.Add(1)
	w := &eventWaiter{
		id:         id,
		name:       name,
		sessionKey: opts.SessionKey,
		agentID:    agentID,
		channel:    opts.Channel,
		chatID:     opts.ChatID,
		intent:     intent,
		reason:     cont.Reason,
		createdAt:  time.Now(),
	}
	if d := continuationDelay(cont); d > 0 {
		w.deadline = time.Now().Add(d)
	}

	bucket := al.eventsBucket(name)
	bucket.mu.Lock()
	bucket.waiters = append(bucket.waiters, w)
	bucket.mu.Unlock()

	logger.InfoCF("agent", "Event waiter registered",
		map[string]any{
			"event":       name,
			"waiter_id":   id,
			"session_key": w.sessionKey,
			"deadline":    w.deadline,
			"intent_len":  len(intent),
		})

	al.saveEventWaitersSnapshot()

	if !w.deadline.IsZero() {
		go al.watchEventWaiterDeadline(w)
	}
}

func (al *AgentLoop) watchEventWaiterDeadline(w *eventWaiter) {
	delay := time.Until(w.deadline)
	if delay <= 0 {
		al.timeoutEventWaiter(w)
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	<-timer.C
	al.timeoutEventWaiter(w)
}

// removeWaiter pops the waiter by id from its bucket. Returns true if it was
// still present (i.e. not already resolved by a FireEvent).
func (al *AgentLoop) removeWaiter(name string, id uint64) bool {
	bucket := al.eventsBucket(name)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	for i, w := range bucket.waiters {
		if w.id == id {
			bucket.waiters = append(bucket.waiters[:i], bucket.waiters[i+1:]...)
			return true
		}
	}
	return false
}

func (al *AgentLoop) timeoutEventWaiter(w *eventWaiter) {
	if !al.removeWaiter(w.name, w.id) {
		return
	}
	logger.InfoCF("agent", "Event waiter timed out",
		map[string]any{
			"event":       w.name,
			"waiter_id":   w.id,
			"session_key": w.sessionKey,
		})
	al.saveEventWaitersSnapshot()
	al.resumeEventWaiter(w, "timed_out", "")
}

// FireEvent resolves all waiters registered for the given event name and
// resumes each with the declared intent plus the supplied payload. Payload
// is injected verbatim into the self-steer content, so keep it short.
// Returns the number of waiters resolved.
func (al *AgentLoop) FireEvent(ctx context.Context, name, payload string) int {
	name = normalizeEventName(name)
	if name == "" {
		return 0
	}
	bucket := al.eventsBucket(name)
	bucket.mu.Lock()
	waiters := bucket.waiters
	bucket.waiters = nil
	bucket.mu.Unlock()

	if len(waiters) == 0 {
		logger.DebugCF("agent", "FireEvent: no waiters",
			map[string]any{"event": name})
		return 0
	}

	logger.InfoCF("agent", "FireEvent resolving waiters",
		map[string]any{
			"event":   name,
			"waiters": len(waiters),
		})

	al.saveEventWaitersSnapshot()

	for _, w := range waiters {
		al.resumeEventWaiter(w, "fired", payload)
	}
	return len(waiters)
}

func (al *AgentLoop) resumeEventWaiter(w *eventWaiter, status, payload string) {
	intent := w.intent
	if strings.TrimSpace(intent) == "" {
		intent = "(no explicit intent; resume from await_event)"
	}
	details := fmt.Sprintf("event=%s status=%s", w.name, status)
	if payload != "" {
		details += "\npayload: " + payload
	}
	reason := strings.TrimSpace(w.reason)
	if reason != "" {
		details += "\nwaiter reason: " + reason
	}

	full := fmt.Sprintf("%s\n[await_event %s]", intent, details)
	al.enqueueSelfContinuation(w.sessionKey, w.agentID, full, "")

	// Deliver by invoking Continue on a detached context so the resumption
	// runs even after the originating request context has been canceled.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	go func() {
		defer cancel()
		if _, err := al.Continue(ctx, w.sessionKey, w.channel, w.chatID); err != nil {
			logger.WarnCF("agent", "Event waiter Continue failed",
				map[string]any{
					"event":       w.name,
					"session_key": w.sessionKey,
					"error":       err.Error(),
				})
		}
	}()
}

// PendingEventWaiters returns a snapshot of active waiters for observability.
// The returned copies are independent of the registry.
func (al *AgentLoop) PendingEventWaiters() map[string]int {
	out := map[string]int{}
	al.events.Range(func(key, value any) bool {
		name, _ := key.(string)
		bucket, _ := value.(*eventBucket)
		if bucket == nil {
			return true
		}
		bucket.mu.Lock()
		n := len(bucket.waiters)
		bucket.mu.Unlock()
		if n > 0 {
			out[name] = n
		}
		return true
	})
	return out
}

// EventWaiterInfo is a public snapshot of a single pending event waiter,
// returned by SnapshotEventWaiters for observability (list_events tool,
// admin UI, tests).
type EventWaiterInfo struct {
	ID         uint64
	Name       string
	SessionKey string
	AgentID    string
	Channel    string
	ChatID     string
	Intent     string
	Reason     string
	CreatedAt  time.Time
	Deadline   time.Time
}

// SnapshotEventWaiters returns a flat, independent slice of all pending
// waiters across every event name. Safe to call concurrently with
// register/fire/timeout; the snapshot reflects the state at call time.
func (al *AgentLoop) SnapshotEventWaiters() []EventWaiterInfo {
	var out []EventWaiterInfo
	al.events.Range(func(_, value any) bool {
		bucket, _ := value.(*eventBucket)
		if bucket == nil {
			return true
		}
		bucket.mu.Lock()
		for _, w := range bucket.waiters {
			out = append(out, EventWaiterInfo{
				ID:         w.id,
				Name:       w.name,
				SessionKey: w.sessionKey,
				AgentID:    w.agentID,
				Channel:    w.channel,
				ChatID:     w.chatID,
				Intent:     w.intent,
				Reason:     w.reason,
				CreatedAt:  w.createdAt,
				Deadline:   w.deadline,
			})
		}
		bucket.mu.Unlock()
		return true
	})
	return out
}

// mentionEventPrefix marks event waiters that match on a substring of inbound
// message content. A waiter named "mention:urgent" fires whenever any inbound
// message's content contains "urgent" (case-insensitive).
const mentionEventPrefix = "mention:"

// fireMentionEventsForMessage scans the event registry for waiters with the
// mention: prefix and fires those whose keyword appears in the inbound
// message content. Called from the Run loop for every inbound message.
func (al *AgentLoop) fireMentionEventsForMessage(ctx context.Context, content string) {
	if content == "" {
		return
	}
	lower := strings.ToLower(content)

	var toFire []string
	al.events.Range(func(key, _ any) bool {
		name, _ := key.(string)
		if !strings.HasPrefix(name, mentionEventPrefix) {
			return true
		}
		keyword := strings.TrimSpace(strings.TrimPrefix(name, mentionEventPrefix))
		if keyword == "" {
			return true
		}
		if strings.Contains(lower, keyword) {
			toFire = append(toFire, name)
		}
		return true
	})

	for _, name := range toFire {
		al.FireEvent(ctx, name, content)
	}
}
