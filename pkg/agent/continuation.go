package agent

import (
	"context"
	"fmt"
	"sort"
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
	ContinuationAwaitAny    ContinuationKind = "await_any"
	ContinuationAwaitAll    ContinuationKind = "await_all"
)

// Continuation describes a post-turn action declared by the model.
type Continuation struct {
	Kind    ContinuationKind
	AfterMS int64
	At      time.Time
	Event   string
	Events  []string
	Intent  string
	Reason  string
	// Scope narrows an event waiter to a specific namespace. Empty means
	// the waiter is global (matches only global fires). A non-empty scope
	// only resolves when FireEvent is called with the identical scope.
	// Conventionally set to an agent ID to partition per-agent
	// subscriptions. The special value "self" is resolved to the declaring
	// agent's ID by the end_turn tool before the continuation is recorded.
	Scope string
	// Sticky only applies when Kind == ContinuationAwaitEvent. When true,
	// the waiter is re-registered automatically after each fire so the
	// subscription survives until the model explicitly cancels it or the
	// waiter times out. Does not apply to timed-out resumptions, or to
	// group waiters (await_any / await_all).
	Sticky bool
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

	// For self-triggering kinds, increment first then check so that the
	// read-check-increment sequence is atomic (no TOCTOU window). External
	// event kinds (await_event/any/all) are triggered by outside signals
	// and do not count against the depth cap.
	isSelfTrigger := cont.Kind == ContinuationContinueNow ||
		cont.Kind == ContinuationWait ||
		cont.Kind == ContinuationSchedule
	if isSelfTrigger {
		next := al.incAutoContinueCount(sessionKey)
		if next > maxAutoContinueDepth {
			al.decAutoContinueCount(sessionKey)
			logger.WarnCF("agent", "Skipping declared continuation; depth cap reached",
				map[string]any{
					"session_key":     sessionKey,
					"kind":            string(cont.Kind),
					"depth_attempted": next,
					"cap":             maxAutoContinueDepth,
					"intent":      cont.Intent,
				})
			return
		}
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
	case ContinuationAwaitAny:
		al.registerEventGroup(cont, opts, agent.ID, intent, groupKindAny)
	case ContinuationAwaitAll:
		al.registerEventGroup(cont, opts, agent.ID, intent, groupKindAll)
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
	logger.InfoCF("agent", "Self continuation enqueued",
		map[string]any{
			"session_key": sessionKey,
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

	// The depth counter was pre-charged by dispatchContinuation. On enqueue
	// failure the slot is burned until the next real inbound message resets
	// it — an acceptable edge case given how rarely enqueue fails.
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

func (al *AgentLoop) incAutoContinueCount(sessionKey string) int32 {
	v, _ := al.autoContinueCounts.LoadOrStore(sessionKey, &atomic.Int32{})
	counter := v.(*atomic.Int32)
	return counter.Add(1)
}

func (al *AgentLoop) resetAutoContinueCount(sessionKey string) {
	al.autoContinueCounts.Delete(sessionKey)
}

func (al *AgentLoop) decAutoContinueCount(sessionKey string) {
	v, ok := al.autoContinueCounts.Load(sessionKey)
	if !ok {
		// The key may have been deleted by a concurrent resetAutoContinueCount
		// (triggered by a real inbound message). That reset wipes the entire
		// counter, so the rollback is a no-op without consequence — the reset wins.
		return
	}
	if counter, ok := v.(*atomic.Int32); ok {
		counter.Add(-1)
	}
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
		ContinuationSchedule, ContinuationAwaitEvent,
		ContinuationAwaitAny, ContinuationAwaitAll:
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
	if raw, ok := args["events"]; ok {
		events, err := parseStringSliceArg(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid events: %w", err)
		}
		cont.Events = events
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
	if v, ok := args["sticky"].(bool); ok {
		cont.Sticky = v
	}
	if v, ok := args["scope"].(string); ok {
		cont.Scope = strings.TrimSpace(v)
	}

	return cont, nil
}

// parseStringSliceArg accepts either []any (JSON array) or []string and
// returns a trimmed, deduplicated list of non-empty strings.
func parseStringSliceArg(v any) ([]string, error) {
	var raw []string
	switch s := v.(type) {
	case []string:
		raw = s
	case []any:
		raw = make([]string, 0, len(s))
		for _, item := range s {
			sv, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected string, got %T", item)
			}
			raw = append(raw, sv)
		}
	default:
		return nil, fmt.Errorf("expected array of strings, got %T", v)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out, nil
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
	// scope narrows resolution to a specific namespace (empty = global).
	// Only FireEventScoped with the same scope resolves scoped waiters.
	scope string
	// afterMS is the original deadline-relative delay in ms (0 = no
	// deadline, or deadline was expressed as an absolute `at` timestamp
	// and cannot be renewed). Used when sticky=true to re-arm the
	// deadline on each re-registration.
	afterMS int64
	// sticky re-registers the waiter automatically after each fire.
	// Re-registration uses a fresh id, same name/session/agent/intent/
	// reason, and a new deadline computed as now + afterMS if afterMS > 0.
	sticky bool
	// stopCh is closed to cancel watchEventWaiterDeadline early (e.g. when a
	// sticky waiter re-registers). Selecting on a nil channel blocks forever,
	// so always initialize with make(chan struct{}).
	stopCh chan struct{}
	// groupID links the waiter to an eventGroup (await_any / await_all).
	// 0 means the waiter is standalone and resumes on fire directly.
	groupID uint64
}

// eventGroup represents a compound waiter that resolves based on its
// children:
//   - groupKindAny: resumes on the first child fire; remaining children are
//     cancelled.
//   - groupKindAll: resumes only after every child has fired; the collected
//     payloads are delivered on resumption.
//
// Group deadlines cancel all still-pending children and resume the group as
// timed_out. Groups are single-shot; they do not support sticky.
type eventGroup struct {
	mu         sync.Mutex
	id         uint64
	kind       string // groupKindAny / groupKindAll
	childIDs   map[uint64]string // child waiter id -> event name (for sibling cancel)
	fired      map[string]string // event name -> payload (await_all collection)
	resolved   bool              // true once the group has resumed
	sessionKey string
	agentID    string
	channel    string
	chatID     string
	intent     string
	reason     string
	scope      string // inherited by all child waiters
	deadline   time.Time
	createdAt  time.Time
	afterMS    int64
}

const (
	groupKindAny = "any"
	groupKindAll = "all"
)

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
		scope:      strings.TrimSpace(cont.Scope),
		afterMS:    cont.AfterMS,
		sticky:     cont.Sticky,
		stopCh:     make(chan struct{}),
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
	select {
	case <-timer.C:
		al.timeoutEventWaiter(w)
	case <-w.stopCh:
		// Waiter was cancelled (e.g. sticky re-registration fired).
	}
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

// FireEvent resolves all GLOBAL (unscoped) waiters for the given event name.
// To fire a scoped event, use FireEventScoped.
func (al *AgentLoop) FireEvent(ctx context.Context, name, payload string) int {
	return al.FireEventScoped(ctx, name, "", payload)
}

// FireEventScoped resolves waiters registered for the given event name whose
// scope matches. Empty scope matches only unscoped waiters; a non-empty scope
// matches only waiters registered with the same scope. This partitions event
// subscriptions across agents / namespaces without renaming them.
// Returns the number of waiters resolved.
func (al *AgentLoop) FireEventScoped(ctx context.Context, name, scope, payload string) int {
	name = normalizeEventName(name)
	if name == "" {
		return 0
	}
	scope = strings.TrimSpace(scope)
	bucket := al.eventsBucket(name)
	bucket.mu.Lock()
	// Partition the bucket: take waiters matching scope, leave the rest.
	remaining := bucket.waiters[:0]
	var waiters []*eventWaiter
	for _, w := range bucket.waiters {
		if w.scope == scope {
			waiters = append(waiters, w)
		} else {
			remaining = append(remaining, w)
		}
	}
	bucket.waiters = remaining
	bucket.mu.Unlock()

	if len(waiters) == 0 {
		logger.DebugCF("agent", "FireEvent: no matching waiters",
			map[string]any{"event": name, "scope": scope})
		return 0
	}

	logger.InfoCF("agent", "FireEvent resolving waiters",
		map[string]any{
			"event":   name,
			"scope":   scope,
			"waiters": len(waiters),
		})

	// Re-register sticky waiters before persisting so the snapshot reflects
	// their continued subscription. Sticky is not meaningful for group
	// children; only standalone waiters may be sticky.
	for _, w := range waiters {
		if w.sticky && w.groupID == 0 {
			al.reRegisterStickyWaiter(w)
		}
	}

	al.saveEventWaitersSnapshot()

	for _, w := range waiters {
		if w.groupID != 0 {
			al.notifyGroupChildFired(w, payload)
			continue
		}
		al.resumeEventWaiter(w, "fired", payload)
	}
	return len(waiters)
}

// reRegisterStickyWaiter installs a fresh copy of a sticky waiter after it
// has fired, with a new id and (if afterMS > 0) a renewed deadline. The
// original waiter is discarded by the caller after this returns.
func (al *AgentLoop) reRegisterStickyWaiter(prev *eventWaiter) {
	newID := al.eventWaiterSeq.Add(1)
	w := &eventWaiter{
		id:         newID,
		name:       prev.name,
		sessionKey: prev.sessionKey,
		agentID:    prev.agentID,
		channel:    prev.channel,
		chatID:     prev.chatID,
		intent:     prev.intent,
		reason:     prev.reason,
		createdAt:  time.Now(),
		scope:      prev.scope,
		afterMS:    prev.afterMS,
		sticky:     true,
		stopCh:     make(chan struct{}),
	}
	if prev.afterMS > 0 {
		w.deadline = time.Now().Add(time.Duration(prev.afterMS) * time.Millisecond)
	}

	bucket := al.eventsBucket(w.name)
	bucket.mu.Lock()
	bucket.waiters = append(bucket.waiters, w)
	bucket.mu.Unlock()

	logger.InfoCF("agent", "Sticky waiter re-registered after fire",
		map[string]any{
			"event":       w.name,
			"prev_id":     prev.id,
			"new_id":      w.id,
			"session_key": w.sessionKey,
		})

	close(prev.stopCh) // cancel the previous waiter's deadline goroutine

	if !w.deadline.IsZero() {
		go al.watchEventWaiterDeadline(w)
	}
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
	Sticky     bool
	AfterMS    int64
	// GroupID is non-zero when the waiter is a child of an await_any or
	// await_all group. Such waiters resolve via the group, not directly.
	GroupID uint64
	Scope   string
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
				Sticky:     w.sticky,
				AfterMS:    w.afterMS,
				GroupID:    w.groupID,
				Scope:      w.scope,
			})
		}
		bucket.mu.Unlock()
		return true
	})
	return out
}

// EventGroupInfo is a public snapshot of a pending compound waiter.
type EventGroupInfo struct {
	ID         uint64
	Kind       string
	SessionKey string
	AgentID    string
	Channel    string
	ChatID     string
	Intent     string
	Reason     string
	CreatedAt  time.Time
	Deadline   time.Time
	AfterMS    int64
	Scope      string
	// Pending maps child waiter id to event name for children still awaiting
	// a fire.
	Pending map[uint64]string
	// Fired maps already-fired event names to their payloads (await_all).
	Fired map[string]string
}

// SnapshotEventGroups returns all pending eventGroups for observability.
func (al *AgentLoop) SnapshotEventGroups() []EventGroupInfo {
	var out []EventGroupInfo
	al.eventGroups.Range(func(_, value any) bool {
		g, _ := value.(*eventGroup)
		if g == nil {
			return true
		}
		g.mu.Lock()
		if g.resolved {
			g.mu.Unlock()
			return true
		}
		info := EventGroupInfo{
			ID:         g.id,
			Kind:       g.kind,
			SessionKey: g.sessionKey,
			AgentID:    g.agentID,
			Channel:    g.channel,
			ChatID:     g.chatID,
			Intent:     g.intent,
			Reason:     g.reason,
			CreatedAt:  g.createdAt,
			Deadline:   g.deadline,
			AfterMS:    g.afterMS,
			Scope:      g.scope,
			Pending:    make(map[uint64]string, len(g.childIDs)),
			Fired:      make(map[string]string, len(g.fired)),
		}
		for id, name := range g.childIDs {
			info.Pending[id] = name
		}
		for k, v := range g.fired {
			info.Fired[k] = v
		}
		g.mu.Unlock()
		out = append(out, info)
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

// ======================== Group waiters (await_any / await_all) ========

// registerEventGroup creates an eventGroup and installs one child waiter per
// named event. Children do not carry individual deadlines; the group owns the
// deadline. kind must be groupKindAny or groupKindAll.
func (al *AgentLoop) registerEventGroup(
	cont *Continuation,
	opts processOptions,
	agentID, intent, kind string,
) {
	names := make([]string, 0, len(cont.Events))
	seen := map[string]bool{}
	for _, raw := range cont.Events {
		n := normalizeEventName(raw)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, n)
	}
	if len(names) == 0 {
		logger.WarnCF("agent", "await_any/all continuation missing events; skipping",
			map[string]any{
				"kind":        kind,
				"session_key": opts.SessionKey,
			})
		return
	}

	groupID := al.eventGroupSeq.Add(1)
	scope := strings.TrimSpace(cont.Scope)
	g := &eventGroup{
		id:         groupID,
		kind:       kind,
		childIDs:   make(map[uint64]string, len(names)),
		fired:      make(map[string]string),
		sessionKey: opts.SessionKey,
		agentID:    agentID,
		channel:    opts.Channel,
		chatID:     opts.ChatID,
		intent:     intent,
		reason:     cont.Reason,
		scope:      scope,
		createdAt:  time.Now(),
		afterMS:    cont.AfterMS,
	}
	if d := continuationDelay(cont); d > 0 {
		g.deadline = time.Now().Add(d)
	}

	for _, name := range names {
		childID := al.eventWaiterSeq.Add(1)
		w := &eventWaiter{
			id:         childID,
			name:       name,
			sessionKey: opts.SessionKey,
			agentID:    agentID,
			channel:    opts.Channel,
			chatID:     opts.ChatID,
			intent:     intent,
			reason:     cont.Reason,
			createdAt:  time.Now(),
			scope:      scope,
			groupID:    groupID,
			stopCh:     make(chan struct{}),
		}
		g.childIDs[childID] = name

		bucket := al.eventsBucket(name)
		bucket.mu.Lock()
		bucket.waiters = append(bucket.waiters, w)
		bucket.mu.Unlock()
	}

	al.eventGroups.Store(groupID, g)

	logger.InfoCF("agent", "Event group registered",
		map[string]any{
			"group_id":    groupID,
			"kind":        kind,
			"names":       names,
			"session_key": opts.SessionKey,
			"deadline":    g.deadline,
		})

	al.saveEventWaitersSnapshot()

	if !g.deadline.IsZero() {
		go al.watchEventGroupDeadline(g)
	}
}

func (al *AgentLoop) watchEventGroupDeadline(g *eventGroup) {
	delay := time.Until(g.deadline)
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
	}
	al.timeoutEventGroup(g)
}

// notifyGroupChildFired is called from FireEvent when a drained waiter
// belongs to a group. It records the fire (for await_all collection) and
// resumes the group when the kind's condition is satisfied.
func (al *AgentLoop) notifyGroupChildFired(child *eventWaiter, payload string) {
	v, ok := al.eventGroups.Load(child.groupID)
	if !ok {
		return
	}
	g, ok := v.(*eventGroup)
	if !ok {
		return
	}

	g.mu.Lock()
	if g.resolved {
		g.mu.Unlock()
		return
	}
	g.fired[child.name] = payload
	delete(g.childIDs, child.id)
	kind := g.kind
	remaining := len(g.childIDs)

	var shouldResume bool
	switch kind {
	case groupKindAny:
		shouldResume = true
	case groupKindAll:
		shouldResume = remaining == 0
	}
	if shouldResume {
		g.resolved = true
	}
	// Capture snapshot for post-unlock use.
	fired := make(map[string]string, len(g.fired))
	for k, v := range g.fired {
		fired[k] = v
	}
	siblings := make(map[uint64]string, len(g.childIDs))
	for id, name := range g.childIDs {
		siblings[id] = name
	}
	g.mu.Unlock()

	if !shouldResume {
		al.saveEventWaitersSnapshot()
		return
	}

	// Cancel any still-pending siblings. For await_any they are leftover
	// children; for await_all there are none (remaining was 0).
	for id, name := range siblings {
		al.removeWaiter(name, id)
	}
	al.eventGroups.Delete(g.id)
	al.saveEventWaitersSnapshot()
	al.resumeEventGroup(g, "fired", child.name, payload, fired)
}

func (al *AgentLoop) timeoutEventGroup(g *eventGroup) {
	g.mu.Lock()
	if g.resolved {
		g.mu.Unlock()
		return
	}
	g.resolved = true
	siblings := make(map[uint64]string, len(g.childIDs))
	for id, name := range g.childIDs {
		siblings[id] = name
	}
	fired := make(map[string]string, len(g.fired))
	for k, v := range g.fired {
		fired[k] = v
	}
	g.childIDs = map[uint64]string{}
	g.mu.Unlock()

	for id, name := range siblings {
		al.removeWaiter(name, id)
	}
	al.eventGroups.Delete(g.id)
	logger.InfoCF("agent", "Event group timed out",
		map[string]any{
			"group_id":    g.id,
			"kind":        g.kind,
			"session_key": g.sessionKey,
			"fired_count": len(fired),
		})
	al.saveEventWaitersSnapshot()
	al.resumeEventGroup(g, "timed_out", "", "", fired)
}

func (al *AgentLoop) resumeEventGroup(
	g *eventGroup,
	status, triggerName, triggerPayload string,
	fired map[string]string,
) {
	intent := strings.TrimSpace(g.intent)
	if intent == "" {
		intent = "(no explicit intent; resume from group waiter)"
	}
	var details strings.Builder
	fmt.Fprintf(&details, "group=%s kind=%s status=%s", fmt.Sprintf("%d", g.id), g.kind, status)
	if triggerName != "" {
		fmt.Fprintf(&details, " trigger=%s", triggerName)
	}
	if triggerPayload != "" {
		fmt.Fprintf(&details, "\ntrigger payload: %s", triggerPayload)
	}
	if len(fired) > 0 {
		details.WriteString("\nfired:")
		// Deterministic ordering.
		names := make([]string, 0, len(fired))
		for n := range fired {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if p := fired[n]; p != "" {
				fmt.Fprintf(&details, "\n  - %s: %s", n, p)
			} else {
				fmt.Fprintf(&details, "\n  - %s", n)
			}
		}
	}
	if reason := strings.TrimSpace(g.reason); reason != "" {
		fmt.Fprintf(&details, "\nwaiter reason: %s", reason)
	}

	full := fmt.Sprintf("%s\n[await_%s %s]", intent, g.kind, details.String())
	al.enqueueSelfContinuation(g.sessionKey, g.agentID, full, "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	go func() {
		defer cancel()
		if _, err := al.Continue(ctx, g.sessionKey, g.channel, g.chatID); err != nil {
			logger.WarnCF("agent", "Event group Continue failed",
				map[string]any{
					"group_id":    g.id,
					"session_key": g.sessionKey,
					"error":       err.Error(),
				})
		}
	}()
}
