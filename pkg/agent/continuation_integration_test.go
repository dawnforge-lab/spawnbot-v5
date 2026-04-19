package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/bus"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers"
)

// scriptedProvider returns pre-loaded responses in sequence.
// After the sequence is exhausted it returns a terminal "done" response.
type scriptedProvider struct {
	responses []providers.LLMResponse
	idx       atomic.Int32
	callCount atomic.Int32
}

func (s *scriptedProvider) Chat(
	_ context.Context,
	_ []providers.Message,
	_ []providers.ToolDefinition,
	_ string,
	_ map[string]any,
) (*providers.LLMResponse, error) {
	s.callCount.Add(1)
	i := int(s.idx.Add(1)) - 1
	if i >= len(s.responses) {
		return &providers.LLMResponse{Content: "done"}, nil
	}
	r := s.responses[i]
	return &r, nil
}

func (s *scriptedProvider) GetDefaultModel() string { return "scripted-model" }

func endTurnCall(continuation, intent string) providers.ToolCall {
	return providers.ToolCall{
		Name:      "end_turn",
		Arguments: map[string]any{"continuation": continuation, "intent": intent},
	}
}

func newScriptedAgentLoop(t *testing.T, p providers.LLMProvider, maxDepth int) *AgentLoop {
	return newScriptedAgentLoopFull(t, p, maxDepth, 10)
}

func newScriptedAgentLoopFull(t *testing.T, p providers.LLMProvider, maxDepth, maxIter int) *AgentLoop {
	t.Helper()
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:            t.TempDir(),
				ModelName:            "scripted-model",
				MaxTokens:            4096,
				MaxToolIterations:    maxIter,
				MaxAutoContinueDepth: maxDepth,
			},
		},
	}
	return NewAgentLoop(cfg, bus.NewMessageBus(), p)
}

// TestContinuation_ContinueNow verifies that a continue_now declaration causes
// the agent loop to immediately queue a second turn and call the provider again.
func TestContinuation_ContinueNow(t *testing.T) {
	p := &scriptedProvider{
		responses: []providers.LLMResponse{
			{
				Content:   "I will continue.",
				ToolCalls: []providers.ToolCall{endTurnCall("continue_now", "testing continuation")},
			},
			{
				Content:   "Done now.",
				ToolCalls: []providers.ToolCall{endTurnCall("done", "finished")},
			},
		},
	}

	al := newScriptedAgentLoop(t, p, 5)

	_, err := al.processMessage(context.Background(), bus.InboundMessage{
		Content:    "start",
		SessionKey: "test-continue-now",
	})
	if err != nil {
		t.Fatalf("processMessage error: %v", err)
	}

	// Give the async self-continuation goroutine time to run.
	time.Sleep(200 * time.Millisecond)

	calls := p.callCount.Load()
	if calls < 2 {
		t.Errorf("expected provider called >= 2 times, got %d", calls)
	}
}

// TestContinuation_DepthCapEnforced verifies that MaxAutoContinueDepth prevents
// runaway self-continuation loops.
func TestContinuation_DepthCapEnforced(t *testing.T) {
	const depthCap = 3
	// 20 responses all requesting continue_now — only depthCap continuations allowed.
	// Use MaxToolIterations=1 so each turn makes exactly 1 provider call (no looping
	// within a single turn), keeping the expected call count predictable.
	responses := make([]providers.LLMResponse, 20)
	for i := range responses {
		responses[i] = providers.LLMResponse{
			Content:   "keep going",
			ToolCalls: []providers.ToolCall{endTurnCall("continue_now", "looping")},
		}
	}

	p := &scriptedProvider{responses: responses}
	al := newScriptedAgentLoopFull(t, p, depthCap, 1)

	_, err := al.processMessage(context.Background(), bus.InboundMessage{
		Content:    "start loop",
		SessionKey: "test-depth-cap",
	})
	if err != nil {
		t.Fatalf("processMessage error: %v", err)
	}

	// Wait long enough for continuations to exhaust themselves.
	time.Sleep(500 * time.Millisecond)

	calls := p.callCount.Load()
	// Each turn = 1 provider call. Initial turn + depthCap continuations = depthCap+1.
	// Allow +1 buffer for scheduling jitter.
	maxAllowed := int32(depthCap + 2)
	if calls > maxAllowed {
		t.Errorf("expected provider called <= %d times (depth cap %d), got %d", maxAllowed, depthCap, calls)
	}
}

// TestContinuation_ShutdownDrainsPending verifies that Stop() returns promptly
// even when a pending schedule continuation is waiting on its timer.
func TestContinuation_ShutdownDrainsPending(t *testing.T) {
	// Return a schedule continuation 1 hour in the future — timer will never fire.
	p := &scriptedProvider{
		responses: []providers.LLMResponse{
			{
				Content: "scheduled.",
				ToolCalls: []providers.ToolCall{
					{
						Name: "end_turn",
						Arguments: map[string]any{
							"continuation": "schedule",
							"intent":       "wake me up in an hour",
							"after_ms":     float64(3_600_000), // 1 hour
						},
					},
				},
			},
		},
	}

	al := newScriptedAgentLoop(t, p, 5)

	_, err := al.processMessage(context.Background(), bus.InboundMessage{
		Content:    "schedule something",
		SessionKey: "test-shutdown",
	})
	if err != nil {
		t.Fatalf("processMessage error: %v", err)
	}

	// Give the goroutine time to register itself before we stop.
	time.Sleep(100 * time.Millisecond)

	stopDone := make(chan struct{})
	go func() {
		al.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		// Stop returned promptly — the goroutine exited via <-al.done.
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2 seconds; pending goroutine may be leaking")
	}
}
