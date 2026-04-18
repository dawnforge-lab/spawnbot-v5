package agent

import (
	"context"
	"fmt"
	"strings"
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
		logger.WarnCF("agent", "await_event continuation declared but not yet wired",
			map[string]any{
				"session_key": sessionKey,
				"event":       cont.Event,
				"intent":      intent,
			})
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
