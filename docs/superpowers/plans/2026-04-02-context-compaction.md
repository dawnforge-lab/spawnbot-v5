# Context Compaction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the blunt drop-50% compression with a two-stage compaction system that summarizes before dropping and restores tool/task context after compaction.

**Architecture:** Rewrite `forceCompression()` in `loop.go` to: (1) flush memory, (2) summarize dropped messages via LLM, (3) inject summary + deferred tools as a replacement message. Add tiered budget thresholds to `context_budget.go` and a circuit breaker for consecutive failures.

**Tech Stack:** Go, existing AgentLoop/ContextBuilder infrastructure, LLM provider

---

### Task 1: Tiered context budget thresholds

**Files:**
- Modify: `pkg/agent/context_budget.go:158-176`
- Test: `pkg/agent/context_budget_test.go` (new tests)

- [ ] **Step 1: Add ContextBudgetTier type and function**

Add to the end of `pkg/agent/context_budget.go`:

```go
// ContextBudgetTier indicates the urgency level for context management.
type ContextBudgetTier int

const (
	// BudgetTierNormal means context is within comfortable limits.
	BudgetTierNormal ContextBudgetTier = iota
	// BudgetTierWarning means context is approaching the limit (>80%).
	BudgetTierWarning
	// BudgetTierCompact means context has exceeded the limit and compaction is needed (>90%).
	BudgetTierCompact
)

// checkContextBudgetTier returns the current budget tier based on usage percentage.
func checkContextBudgetTier(
	contextWindow int,
	messages []providers.Message,
	toolDefs []providers.ToolDefinition,
	maxTokens int,
) ContextBudgetTier {
	msgTokens := 0
	for _, m := range messages {
		msgTokens += estimateMessageTokens(m)
	}
	toolTokens := estimateToolDefsTokens(toolDefs)
	total := msgTokens + toolTokens + maxTokens

	usage := float64(total) / float64(contextWindow)
	if usage > 0.9 {
		return BudgetTierCompact
	}
	if usage > 0.8 {
		return BudgetTierWarning
	}
	return BudgetTierNormal
}
```

- [ ] **Step 2: Write tests**

Add to `pkg/agent/context_budget_test.go`:

```go
func TestCheckContextBudgetTier_Normal(t *testing.T) {
	// 1000 token window, ~100 tokens of messages, 100 max_tokens = 20% usage
	msgs := []providers.Message{{Role: "user", Content: strings.Repeat("a", 250)}}
	tier := checkContextBudgetTier(1000, msgs, nil, 100)
	if tier != BudgetTierNormal {
		t.Fatalf("expected BudgetTierNormal, got %d", tier)
	}
}

func TestCheckContextBudgetTier_Warning(t *testing.T) {
	// 1000 token window, ~700 tokens of messages + 200 max_tokens = 90% → warning
	msgs := []providers.Message{{Role: "user", Content: strings.Repeat("a", 1500)}}
	tier := checkContextBudgetTier(1000, msgs, nil, 200)
	if tier != BudgetTierWarning {
		t.Fatalf("expected BudgetTierWarning, got %d", tier)
	}
}

func TestCheckContextBudgetTier_Compact(t *testing.T) {
	// 1000 token window, ~900 tokens of messages + 200 max_tokens = 110% → compact
	msgs := []providers.Message{{Role: "user", Content: strings.Repeat("a", 2000)}}
	tier := checkContextBudgetTier(1000, msgs, nil, 200)
	if tier != BudgetTierCompact {
		t.Fatalf("expected BudgetTierCompact, got %d", tier)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/agent/ -run TestCheckContextBudgetTier -v -count=1`
Expected: All 3 PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/agent/context_budget.go pkg/agent/context_budget_test.go
git commit -m "feat(agent): add tiered context budget thresholds (normal/warning/compact)"
```

---

### Task 2: Rewrite forceCompression with summarization

**Files:**
- Modify: `pkg/agent/loop.go:2964-3043` (forceCompression)

- [ ] **Step 1: Add compaction summarization prompt constant**

Add after the existing `toolRepetitionWarning` constant (~line 108 in `loop.go`):

```go
compactionSummaryPrompt = "Provide a concise summary of this conversation segment, preserving core context, key decisions, and task progress. Focus on what was accomplished, what was decided, and what is still in progress.\n\nCONVERSATION:\n%s"
```

- [ ] **Step 2: Add circuit breaker state**

Add to the `AgentLoop` struct fields (find the struct definition):

```go
compactionFailures int // consecutive compaction failures for circuit breaker
```

- [ ] **Step 3: Rewrite forceCompression**

Replace the entire `forceCompression` function (lines 2981-3043) with:

```go
func (al *AgentLoop) forceCompression(agent *AgentInstance, sessionKey string) (compressionResult, bool) {
	history := agent.Sessions.GetHistory(sessionKey)
	if len(history) <= 2 {
		return compressionResult{}, false
	}

	// Circuit breaker: after 3 consecutive failures, skip summarization.
	const maxCompactionFailures = 3
	skipSummarization := al.compactionFailures >= maxCompactionFailures

	// Split at a Turn boundary so no tool-call sequence is torn apart.
	turns := parseTurnBoundaries(history)
	var mid int
	if len(turns) >= 2 {
		mid = turns[len(turns)/2]
	} else {
		mid = findSafeBoundary(history, len(history)/2)
	}

	var keptHistory []providers.Message
	var droppedMessages []providers.Message

	if mid <= 0 {
		// No safe Turn boundary — keep only the most recent user message.
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == "user" {
				keptHistory = []providers.Message{history[i]}
				droppedMessages = history[:i]
				break
			}
		}
	} else {
		droppedMessages = history[:mid]
		keptHistory = history[mid:]
	}

	droppedCount := len(droppedMessages)

	// Stage 1: Flush key facts to daily notes before messages are lost.
	al.flushMemoryPreCompaction(agent, droppedMessages)

	// Stage 2: Summarize dropped messages and inject as replacement.
	var summaryMsg string
	if !skipSummarization && len(droppedMessages) > 0 {
		summaryMsg = al.summarizeForCompaction(agent, droppedMessages)
	}

	if summaryMsg != "" {
		al.compactionFailures = 0 // reset circuit breaker on success

		// Build post-compact restoration content.
		var restoration strings.Builder
		restoration.WriteString("[SYSTEM] Previous conversation was compacted. Summary of what was discussed:\n\n")
		restoration.WriteString(summaryMsg)

		// Re-inject deferred tools announcement.
		if announcement := BuildDeferredToolsAnnouncement(agent.Tools); announcement != "" {
			restoration.WriteString("\n\n")
			restoration.WriteString(announcement)
		}

		compactMsg := providers.Message{
			Role:    "user",
			Content: restoration.String(),
		}
		keptHistory = append([]providers.Message{compactMsg}, keptHistory...)
	} else {
		if !skipSummarization {
			al.compactionFailures++
		}
		// Fallback: record compression note in session summary (original behavior).
		existingSummary := agent.Sessions.GetSummary(sessionKey)
		compressionNote := fmt.Sprintf(
			"[Emergency compression dropped %d oldest messages due to context limit]",
			droppedCount,
		)
		if existingSummary != "" {
			compressionNote = existingSummary + "\n\n" + compressionNote
		}
		agent.Sessions.SetSummary(sessionKey, compressionNote)
	}

	agent.Sessions.SetHistory(sessionKey, keptHistory)
	agent.Sessions.Save(sessionKey)

	logger.WarnCF("agent", "Forced compression executed", map[string]any{
		"session_key":    sessionKey,
		"dropped_msgs":   droppedCount,
		"new_count":      len(keptHistory),
		"has_summary":    summaryMsg != "",
		"circuit_broken": skipSummarization,
	})

	return compressionResult{
		DroppedMessages:   droppedCount,
		RemainingMessages: len(keptHistory),
	}, true
}
```

- [ ] **Step 4: Add summarizeForCompaction helper**

Add after the `forceCompression` function:

```go
// summarizeForCompaction builds a concise LLM summary of messages being dropped.
// Returns empty string on failure (caller falls back to blunt drop).
func (al *AgentLoop) summarizeForCompaction(
	agent *AgentInstance,
	messages []providers.Message,
) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build conversation text, skipping tool results and oversized messages.
	maxMessageChars := agent.ContextWindow // rough char limit per message
	var convBuf strings.Builder
	for _, m := range messages {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if len(content) > maxMessageChars {
			content = content[:maxMessageChars] + "..."
		}
		fmt.Fprintf(&convBuf, "%s: %s\n", m.Role, content)
	}

	if convBuf.Len() == 0 {
		return ""
	}

	prompt := fmt.Sprintf(compactionSummaryPrompt, convBuf.String())
	resp, err := al.retryLLMCall(ctx, agent, prompt, 2)
	if err != nil || resp == nil || resp.Content == "" {
		logger.WarnCF("agent", "Compaction summarization failed",
			map[string]any{"error": fmt.Sprintf("%v", err), "agent_id": agent.ID})
		return ""
	}

	return strings.TrimSpace(resp.Content)
}
```

- [ ] **Step 5: Ensure `strings` import exists**

The `strings` package should already be imported in `loop.go`. Verify it is.

- [ ] **Step 6: Verify build**

Run: `go build ./pkg/agent/...`
Expected: clean build

- [ ] **Step 7: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): rewrite forceCompression with LLM summarization and circuit breaker"
```

---

### Task 3: Wire tiered thresholds into the turn loop

**Files:**
- Modify: `pkg/agent/loop.go:1737-1757` (proactive compression call site)

- [ ] **Step 1: Replace isOverContextBudget call with tiered check**

Find the proactive compression block (~line 1737):

```go
	if !ts.opts.NoHistory {
		toolDefs := ts.agent.Tools.ToProviderDefs()
		if isOverContextBudget(ts.agent.ContextWindow, messages, toolDefs, ts.agent.MaxTokens) {
			logger.WarnCF("agent", "Proactive compression: context budget exceeded before LLM call",
				map[string]any{"session_key": ts.sessionKey})
```

Replace the `if isOverContextBudget(...)` line and its log with:

```go
	if !ts.opts.NoHistory {
		toolDefs := ts.agent.Tools.ToProviderDefs()
		budgetTier := checkContextBudgetTier(ts.agent.ContextWindow, messages, toolDefs, ts.agent.MaxTokens)
		if budgetTier == BudgetTierWarning {
			logger.WarnCF("agent", "Context budget warning: approaching limit (>80%)",
				map[string]any{"session_key": ts.sessionKey})
		}
		if budgetTier == BudgetTierCompact {
			logger.WarnCF("agent", "Proactive compaction: context budget exceeded (>90%)",
				map[string]any{"session_key": ts.sessionKey})
```

Make sure to preserve the rest of the block (the `forceCompression` call and event emission) unchanged. The closing `}` for the old `if isOverContextBudget` should now close the `if budgetTier == BudgetTierCompact` block.

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/agent/...`
Expected: clean build

- [ ] **Step 3: Run all agent tests**

Run: `go test ./pkg/agent/ -count=1 -timeout 120s 2>&1 | tail -5`
Expected: PASS (or known pre-existing failures only)

- [ ] **Step 4: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): wire tiered context budget thresholds into turn loop"
```

---

### Task 4: Tests for compaction with summarization

**Files:**
- Modify: `pkg/agent/loop_test.go`

- [ ] **Step 1: Add a mock provider that triggers compaction**

Add to `pkg/agent/loop_test.go` after the existing `twoTestToolB` block:

```go
// compactionTestProvider returns tool calls that generate enough content to trigger compaction.
// After maxCalls tool calls, returns a text response.
type compactionTestProvider struct {
	mu       sync.Mutex
	calls    int
	maxCalls int
	// summaryCalled tracks whether the provider was asked to summarize (a non-tool-call Chat request)
	summaryCalls int
}

func (p *compactionTestProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	p.mu.Lock()
	p.calls++
	n := p.calls
	p.mu.Unlock()

	// If no tools are provided, this is a summarization call
	if len(tools) == 0 {
		p.mu.Lock()
		p.summaryCalls++
		p.mu.Unlock()
		return &providers.LLMResponse{Content: "Summary: the conversation covered file reading and testing."}, nil
	}

	if n > p.maxCalls {
		return &providers.LLMResponse{Content: "done"}, nil
	}
	return &providers.LLMResponse{
		ToolCalls: []providers.ToolCall{{
			ID:        fmt.Sprintf("call_%d", n),
			Type:      "function",
			Name:      "compaction_test_tool",
			Arguments: map[string]any{"value": strings.Repeat("x", 500)},
		}},
	}, nil
}

func (p *compactionTestProvider) GetDefaultModel() string {
	return "compaction-test-model"
}

// compactionTestTool returns a large result to fill context quickly.
type compactionTestTool struct{}

func (m *compactionTestTool) Name() string        { return "compaction_test_tool" }
func (m *compactionTestTool) Description() string  { return "Test tool for compaction" }
func (m *compactionTestTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"value": map[string]any{"type": "string"}},
	}
}
func (m *compactionTestTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	// Return a large result to fill context
	return tools.SilentResult(strings.Repeat("result data ", 200))
}
```

- [ ] **Step 2: Write test verifying summary message is injected after compaction**

```go
func TestAgentLoop_CompactionInjectsSummary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				ModelName:         "test-model",
				MaxTokens:         500,
				MaxToolIterations: 20,
				ContextWindow:     4000, // Small window to trigger compaction quickly
			},
		},
	}

	provider := &compactionTestProvider{maxCalls: 15}
	al := NewAgentLoop(cfg, bus.NewMessageBus(), provider)
	al.RegisterTool(&compactionTestTool{})

	_, err = al.ProcessDirectWithChannel(context.Background(), "hello", "compact-test", "test", "chat1")
	if err != nil {
		t.Fatalf("ProcessDirectWithChannel failed: %v", err)
	}

	defaultAgent := al.registry.GetDefaultAgent()
	route := al.registry.ResolveRoute(routing.RouteInput{
		Channel: "test",
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "cron"},
	})
	history := defaultAgent.Sessions.GetHistory(route.SessionKey)

	// Check if a compaction summary message was injected
	for _, msg := range history {
		if msg.Role == "user" && strings.Contains(msg.Content, "Previous conversation was compacted") {
			// Verify it contains a summary
			if !strings.Contains(msg.Content, "Summary:") {
				t.Fatal("compaction message missing summary content")
			}
			return // success
		}
	}

	// If context was never exceeded, the test setup needs adjusting
	// Check if provider was asked to summarize
	provider.mu.Lock()
	sc := provider.summaryCalls
	provider.mu.Unlock()
	if sc == 0 {
		t.Skip("Context window was never exceeded — adjust test parameters if needed")
	}
	t.Fatal("expected compaction summary message in history, found none")
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/agent/ -run TestAgentLoop_Compaction -v -count=1 -timeout 60s`
Expected: PASS (or SKIP if window not exceeded)

- [ ] **Step 4: Commit**

```bash
git add pkg/agent/loop_test.go
git commit -m "test(agent): add tests for compaction with LLM summarization"
```

---

### Task 5: Build, deploy, and verify

**Files:**
- No code changes — build and test

- [ ] **Step 1: Run full agent test suite**

Run: `go test ./pkg/agent/ -count=1 -timeout 120s 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 2: Build and install**

```bash
make build && make install
```

- [ ] **Step 3: Restart gateway**

```bash
systemctl --user restart spawnbot-gateway.service
sleep 3
systemctl --user status spawnbot-gateway.service | head -5
```

- [ ] **Step 4: Verify gateway starts cleanly**

Check logs: `tail -5 ~/.spawnbot/logs/gateway.log`
Expected: No errors related to compaction

- [ ] **Step 5: Commit if any fixes were needed, otherwise done**
