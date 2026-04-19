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
			"scope": map[string]any{
				"type":        "string",
				"description": "Only return waiters with this scope. Use an explicit empty string to restrict to global (unscoped) waiters, or 'self' to auto-resolve to the current agent's id.",
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
	scopeArg, scopeProvided := args["scope"].(string)
	scopeFilter := strings.TrimSpace(scopeArg)
	if scopeFilter == "self" {
		if ts := TurnStateFromContext(ctx); ts != nil && ts.agent != nil {
			scopeFilter = ts.agent.ID
		} else {
			scopeFilter = ""
		}
	}

	waiters := al.SnapshotEventWaiters()
	groups := al.SnapshotEventGroups()
	if prefix != "" || sessionKey != "" || scopeProvided {
		filtered := waiters[:0]
		for _, w := range waiters {
			if prefix != "" && !strings.HasPrefix(w.Name, prefix) {
				continue
			}
			if sessionKey != "" && w.SessionKey != sessionKey {
				continue
			}
			if scopeProvided && w.Scope != scopeFilter {
				continue
			}
			filtered = append(filtered, w)
		}
		waiters = filtered

		fg := groups[:0]
		for _, g := range groups {
			if sessionKey != "" && g.SessionKey != sessionKey {
				continue
			}
			if scopeProvided && g.Scope != scopeFilter {
				continue
			}
			if prefix != "" {
				matches := false
				for _, name := range g.Pending {
					if strings.HasPrefix(name, prefix) {
						matches = true
						break
					}
				}
				if !matches {
					continue
				}
			}
			fg = append(fg, g)
		}
		groups = fg
	}

	if len(waiters) == 0 && len(groups) == 0 {
		return tools.SilentResult("No pending event waiters.")
	}

	// Stable ordering: by name, then by creation time.
	sort.Slice(waiters, func(i, j int) bool {
		if waiters[i].Name != waiters[j].Name {
			return waiters[i].Name < waiters[j].Name
		}
		return waiters[i].CreatedAt.Before(waiters[j].CreatedAt)
	})
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})

	now := time.Now()
	var b strings.Builder
	// Separate standalone waiters from group children for a cleaner read.
	standalone := 0
	grouped := 0
	for _, w := range waiters {
		if w.GroupID != 0 {
			grouped++
		} else {
			standalone++
		}
	}
	fmt.Fprintf(&b, "Pending event waiters (%d standalone, %d in groups) + %d groups:\n",
		standalone, grouped, len(groups))
	for _, w := range waiters {
		if w.GroupID != 0 {
			continue // Rendered under its group below.
		}
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
		scopeDesc := "global"
		if w.Scope != "" {
			scopeDesc = w.Scope
		}
		fmt.Fprintf(&b, "- id=%d name=%s scope=%s age=%s deadline=%s session=%s agent=%s%s\n",
			w.ID, w.Name, scopeDesc, age, deadlineDesc, w.SessionKey, w.AgentID, stickyDesc)
		if w.Intent != "" {
			fmt.Fprintf(&b, "    intent: %s\n", truncateForListing(w.Intent, 200))
		}
		if w.Reason != "" {
			fmt.Fprintf(&b, "    reason: %s\n", truncateForListing(w.Reason, 120))
		}
	}

	for _, g := range groups {
		age := now.Sub(g.CreatedAt).Round(time.Second)
		deadlineDesc := "none"
		if !g.Deadline.IsZero() {
			remaining := time.Until(g.Deadline).Round(time.Second)
			if remaining > 0 {
				deadlineDesc = fmt.Sprintf("in %s", remaining)
			} else {
				deadlineDesc = fmt.Sprintf("expired %s ago", (-remaining))
			}
		}
		pendingNames := make([]string, 0, len(g.Pending))
		for _, n := range g.Pending {
			pendingNames = append(pendingNames, n)
		}
		sort.Strings(pendingNames)
		firedNames := make([]string, 0, len(g.Fired))
		for n := range g.Fired {
			firedNames = append(firedNames, n)
		}
		sort.Strings(firedNames)
		scopeDesc := "global"
		if g.Scope != "" {
			scopeDesc = g.Scope
		}
		fmt.Fprintf(&b, "- group id=%d kind=await_%s scope=%s age=%s deadline=%s session=%s agent=%s\n",
			g.ID, g.Kind, scopeDesc, age, deadlineDesc, g.SessionKey, g.AgentID)
		fmt.Fprintf(&b, "    pending: %s\n", strings.Join(pendingNames, ", "))
		if len(firedNames) > 0 {
			fmt.Fprintf(&b, "    fired:   %s\n", strings.Join(firedNames, ", "))
		}
		if g.Intent != "" {
			fmt.Fprintf(&b, "    intent: %s\n", truncateForListing(g.Intent, 200))
		}
		if g.Reason != "" {
			fmt.Fprintf(&b, "    reason: %s\n", truncateForListing(g.Reason, 120))
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
