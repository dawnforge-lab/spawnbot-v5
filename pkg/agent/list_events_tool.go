package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// listEventsTool reports pending await_event waiters for observability and
// debugging. The model can use it to audit its own subscriptions ("what am I
// still waiting on?") or to decide whether to cancel/re-declare a waiter.
type listEventsTool struct{}

func newListEventsTool() *listEventsTool { return &listEventsTool{} }

func (t *listEventsTool) Name() string { return "list_events" }

func (t *listEventsTool) Description() string {
	return "List pending event waiters (from await_event continuations). " +
		"Useful to audit what you are still waiting on, spot stale " +
		"subscriptions, and decide whether to re-declare or cancel a " +
		"waiter. Optional name_prefix filters the result (e.g. 'feed:', " +
		"'mention:'); optional session_key restricts to a specific session."
}

func (t *listEventsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name_prefix": map[string]any{
				"type":        "string",
				"description": "Only return waiters whose event name starts with this prefix (case-insensitive).",
			},
			"session_key": map[string]any{
				"type":        "string",
				"description": "Only return waiters for this session key.",
			},
		},
		"additionalProperties": false,
	}
}

func (t *listEventsTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	al := AgentLoopFromContext(ctx)
	if al == nil {
		return tools.SilentResult("list_events: no AgentLoop in context.")
	}

	prefix := strings.ToLower(strings.TrimSpace(stringArg(args, "name_prefix")))
	sessionKey := strings.TrimSpace(stringArg(args, "session_key"))

	waiters := al.SnapshotEventWaiters()
	if prefix != "" || sessionKey != "" {
		filtered := waiters[:0]
		for _, w := range waiters {
			if prefix != "" && !strings.HasPrefix(w.Name, prefix) {
				continue
			}
			if sessionKey != "" && w.SessionKey != sessionKey {
				continue
			}
			filtered = append(filtered, w)
		}
		waiters = filtered
	}

	if len(waiters) == 0 {
		return tools.SilentResult("No pending event waiters.")
	}

	// Stable ordering: by name, then by creation time.
	sort.Slice(waiters, func(i, j int) bool {
		if waiters[i].Name != waiters[j].Name {
			return waiters[i].Name < waiters[j].Name
		}
		return waiters[i].CreatedAt.Before(waiters[j].CreatedAt)
	})

	now := time.Now()
	var b strings.Builder
	fmt.Fprintf(&b, "Pending event waiters (%d):\n", len(waiters))
	for _, w := range waiters {
		age := now.Sub(w.CreatedAt).Round(time.Second)
		deadlineDesc := "none"
		if !w.Deadline.IsZero() {
			remaining := time.Until(w.Deadline).Round(time.Second)
			if remaining > 0 {
				deadlineDesc = fmt.Sprintf("in %s", remaining)
			} else {
				deadlineDesc = fmt.Sprintf("expired %s ago", (-remaining))
			}
		}
		stickyDesc := ""
		if w.Sticky {
			stickyDesc = " sticky"
		}
		fmt.Fprintf(&b, "- id=%d name=%s age=%s deadline=%s session=%s agent=%s%s\n",
			w.ID, w.Name, age, deadlineDesc, w.SessionKey, w.AgentID, stickyDesc)
		if w.Intent != "" {
			fmt.Fprintf(&b, "    intent: %s\n", truncateForListing(w.Intent, 200))
		}
		if w.Reason != "" {
			fmt.Fprintf(&b, "    reason: %s\n", truncateForListing(w.Reason, 120))
		}
	}
	return tools.SilentResult(b.String())
}

func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func truncateForListing(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
