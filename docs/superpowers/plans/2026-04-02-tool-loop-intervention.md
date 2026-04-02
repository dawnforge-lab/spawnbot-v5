# Tool Loop Intervention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Inject a warning message mid-turn when the agent calls the same tool 3+ times, prompting it to reconsider its approach.

**Architecture:** Add per-tool call counters inside `runTurn()` in `loop.go`, mirroring the existing `consecutiveToolErrors` pattern. When threshold is hit, inject a `[SYSTEM]` user-role message into the conversation. One warning per tool per turn.

**Tech Stack:** Go, existing agent loop infrastructure

---

### Task 1: Add the warning constant and state variables

**Files:**
- Modify: `pkg/agent/loop.go:107` (constants block)
- Modify: `pkg/agent/loop.go:1778` (runTurn local state)

- [ ] **Step 1: Add the `toolRepetitionWarning` constant**

In `pkg/agent/loop.go`, add after `toolWindDownWarning` (line 107):

```go
toolRepetitionWarning = "[SYSTEM] You have called '%s' %d times this turn. You may be stuck in a loop. Pause and consider: why do you keep needing this tool? Try a different approach to achieve your goal."
```

- [ ] **Step 2: Add per-turn state variables**

In `pkg/agent/loop.go`, add after `const maxConsecutiveToolErrors = 3` (line 1779):

```go
toolCallCounts := make(map[string]int)
toolLoopWarned := make(map[string]bool)
const repeatedToolThreshold = 3
```

- [ ] **Step 3: Verify build**

Run: `go build ./pkg/agent/...`
Expected: clean build, no errors

- [ ] **Step 4: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): add tool repetition warning constant and per-turn counters"
```

---

### Task 2: Add the intervention logic

**Files:**
- Modify: `pkg/agent/loop.go:2658` (after consecutiveToolErrors block)

- [ ] **Step 1: Add the counting and warning injection**

In `pkg/agent/loop.go`, add after the `consecutiveToolErrors = 0` reset (line 2658), before the steering message dequeue (line 2660):

```go
			toolCallCounts[toolName]++
			if toolCallCounts[toolName] >= repeatedToolThreshold && !toolLoopWarned[toolName] {
				toolLoopWarned[toolName] = true
				repWarn := providers.Message{
					Role:    "user",
					Content: fmt.Sprintf(toolRepetitionWarning, toolName, toolCallCounts[toolName]),
				}
				ts.agent.Sessions.AddFullMessage(ts.sessionKey, repWarn)
				messages = append(messages, repWarn)
				logger.WarnCF("agent", "Tool repetition warning injected", map[string]any{
					"agent_id": ts.agent.ID,
					"tool":     toolName,
					"count":    toolCallCounts[toolName],
				})
			}
```

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/agent/...`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): inject warning when same tool called 3+ times per turn"
```

---

### Task 3: Write tests

**Files:**
- Modify: `pkg/agent/loop_test.go`

- [ ] **Step 1: Add a mock provider that repeats the same tool call**

Add to `pkg/agent/loop_test.go` after the `toolLimitTestTool` block (~line 1258):

```go
// repeatedToolProvider always asks to call the same tool, up to a max number of turns.
type repeatedToolProvider struct {
	mu       sync.Mutex
	calls    int
	maxCalls int
}

func (p *repeatedToolProvider) Chat(
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

	if n > p.maxCalls {
		return &providers.LLMResponse{Content: "done"}, nil
	}
	return &providers.LLMResponse{
		ToolCalls: []providers.ToolCall{{
			ID:        fmt.Sprintf("call_%d", n),
			Type:      "function",
			Name:      "repeated_test_tool",
			Arguments: map[string]any{"value": "x"},
		}},
	}, nil
}

func (p *repeatedToolProvider) GetDefaultModel() string {
	return "repeated-tool-model"
}

// repeatedTestTool is a tool that always succeeds.
type repeatedTestTool struct{}

func (m *repeatedTestTool) Name() string        { return "repeated_test_tool" }
func (m *repeatedTestTool) Description() string  { return "Test tool for repetition detection" }
func (m *repeatedTestTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"value": map[string]any{"type": "string"}},
	}
}
func (m *repeatedTestTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	return tools.SilentResult("ok")
}
```

- [ ] **Step 2: Write test for warning injection at threshold**

```go
func TestAgentLoop_ToolRepetitionWarningInjected(t *testing.T) {
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
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	provider := &repeatedToolProvider{maxCalls: 4}
	al := NewAgentLoop(cfg, bus.NewMessageBus(), provider)
	al.RegisterTool(&repeatedTestTool{})

	_, err = al.ProcessDirectWithChannel(context.Background(), "hello", "rep-warn", "test", "chat1")
	if err != nil {
		t.Fatalf("ProcessDirectWithChannel failed: %v", err)
	}

	defaultAgent := al.registry.GetDefaultAgent()
	route := al.registry.ResolveRoute(routing.RouteInput{
		Channel: "test",
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "cron"},
	})
	history := defaultAgent.Sessions.GetHistory(route.SessionKey)

	// Find the repetition warning in history
	found := 0
	for _, msg := range history {
		if msg.Role == "user" && strings.Contains(msg.Content, "stuck in a loop") {
			found++
		}
	}
	if found == 0 {
		t.Fatal("expected tool repetition warning in session history, found none")
	}
	if found > 1 {
		t.Fatalf("expected exactly 1 repetition warning, got %d", found)
	}
}
```

- [ ] **Step 3: Write test for independent counters per tool**

```go
// twoToolProvider alternates between two different tools.
type twoToolProvider struct {
	mu       sync.Mutex
	calls    int
	maxCalls int
}

func (p *twoToolProvider) Chat(
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

	if n > p.maxCalls {
		return &providers.LLMResponse{Content: "done"}, nil
	}
	// Alternate: tool_a, tool_b, tool_a, tool_b — neither hits threshold of 3
	toolName := "two_test_tool_a"
	if n%2 == 0 {
		toolName = "two_test_tool_b"
	}
	return &providers.LLMResponse{
		ToolCalls: []providers.ToolCall{{
			ID:        fmt.Sprintf("call_%d", n),
			Type:      "function",
			Name:      toolName,
			Arguments: map[string]any{"value": "x"},
		}},
	}, nil
}

func (p *twoToolProvider) GetDefaultModel() string {
	return "two-tool-model"
}

type twoTestToolA struct{}

func (m *twoTestToolA) Name() string        { return "two_test_tool_a" }
func (m *twoTestToolA) Description() string  { return "Test tool A" }
func (m *twoTestToolA) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"value": map[string]any{"type": "string"}},
	}
}
func (m *twoTestToolA) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	return tools.SilentResult("ok")
}

type twoTestToolB struct{}

func (m *twoTestToolB) Name() string        { return "two_test_tool_b" }
func (m *twoTestToolB) Description() string  { return "Test tool B" }
func (m *twoTestToolB) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"value": map[string]any{"type": "string"}},
	}
}
func (m *twoTestToolB) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	return tools.SilentResult("ok")
}

func TestAgentLoop_ToolRepetitionCountersIndependent(t *testing.T) {
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
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	// 4 calls alternating: a, b, a, b — each tool called only 2 times, below threshold
	provider := &twoToolProvider{maxCalls: 4}
	al := NewAgentLoop(cfg, bus.NewMessageBus(), provider)
	al.RegisterTool(&twoTestToolA{})
	al.RegisterTool(&twoTestToolB{})

	_, err = al.ProcessDirectWithChannel(context.Background(), "hello", "rep-indep", "test", "chat1")
	if err != nil {
		t.Fatalf("ProcessDirectWithChannel failed: %v", err)
	}

	defaultAgent := al.registry.GetDefaultAgent()
	route := al.registry.ResolveRoute(routing.RouteInput{
		Channel: "test",
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "cron"},
	})
	history := defaultAgent.Sessions.GetHistory(route.SessionKey)

	for _, msg := range history {
		if msg.Role == "user" && strings.Contains(msg.Content, "stuck in a loop") {
			t.Fatal("no repetition warning expected when tools alternate below threshold")
		}
	}
}
```

- [ ] **Step 4: Run all tests**

Run: `go test ./pkg/agent/ -run TestAgentLoop_ToolRepetition -v -count=1`
Expected: both tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/loop_test.go
git commit -m "test(agent): add tests for tool repetition warning injection"
```
