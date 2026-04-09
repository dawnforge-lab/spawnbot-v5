package council

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agent"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers/protocoltypes"
)

// mockProvider implements providers.LLMProvider and returns responses in order.
type mockProvider struct {
	mu        sync.Mutex
	responses []string
	callIndex int
}

func (m *mockProvider) Chat(ctx context.Context, messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition, model string, options map[string]any) (*protocoltypes.LLMResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.callIndex >= len(m.responses) {
		return &protocoltypes.LLMResponse{Content: "fallback response"}, nil
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return &protocoltypes.LLMResponse{Content: resp}, nil
}

func (m *mockProvider) GetDefaultModel() string {
	return "mock-model"
}

func setupRegistry() *agents.Registry {
	reg := agents.NewRegistry()
	reg.Register(&agents.AgentDefinition{
		Name:         "researcher",
		Description:  "Research agent",
		SystemPrompt: "You are a researcher. Analyze topics thoroughly.",
		Model:        "mock-model",
		Source:       "builtin",
	})
	reg.Register(&agents.AgentDefinition{
		Name:         "coder",
		Description:  "Coding agent",
		SystemPrompt: "You are a coder. Write clean, efficient code.",
		Model:        "mock-model",
		Source:       "builtin",
	})
	return reg
}

func TestEngine_RunBasicCouncil(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	reg := setupRegistry()
	eventBus := agent.NewEventBus()
	defer eventBus.Close()

	// Collect events
	sub := eventBus.Subscribe(64)
	var events []agent.Event
	var eventsMu sync.Mutex
	done := make(chan struct{})
	go func() {
		defer close(done)
		for evt := range sub.C {
			eventsMu.Lock()
			events = append(events, evt)
			eventsMu.Unlock()
		}
	}()

	mock := &mockProvider{
		responses: []string{
			// Round 1: researcher response
			"Research findings: The topic is well-studied.",
			// Round 1: coder response
			"Code suggestion: Use a hash map for O(1) lookups.",
			// Round 1: moderator decision
			"CONCLUDE - the agents have provided sufficient input.",
			// Synthesis
			"Synthesis: The council recommends using a hash map approach based on thorough research.",
		},
	}

	eng := NewEngine(store, reg, mock, eventBus)

	result, err := eng.Run(context.Background(), CouncilConfig{
		Title:  "Test Council",
		Topic:  "How to optimize data lookups?",
		Roster: []string{"researcher", "coder"},
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Close eventBus to flush events
	eventBus.Close()
	<-done

	// Verify result
	if result.ID == "" {
		t.Error("expected non-empty council ID")
	}
	if result.Title != "Test Council" {
		t.Errorf("expected title 'Test Council', got %q", result.Title)
	}
	if result.Rounds != 1 {
		t.Errorf("expected 1 round, got %d", result.Rounds)
	}
	if result.Status != StatusClosed {
		t.Errorf("expected status %q, got %q", StatusClosed, result.Status)
	}
	if !strings.Contains(result.Synthesis, "hash map") {
		t.Errorf("expected synthesis to contain 'hash map', got %q", result.Synthesis)
	}

	// Verify transcript was persisted
	transcript, err := store.GetTranscript(result.ID)
	if err != nil {
		t.Fatalf("GetTranscript() error: %v", err)
	}
	// Expect: user topic + researcher + coder + moderator + synthesis = 5 entries
	if len(transcript) < 4 {
		t.Errorf("expected at least 4 transcript entries, got %d", len(transcript))
	}

	// Verify events were emitted
	eventsMu.Lock()
	defer eventsMu.Unlock()

	var hasStart, hasEnd bool
	for _, evt := range events {
		if evt.Kind == agent.EventKindCouncilStart {
			hasStart = true
		}
		if evt.Kind == agent.EventKindCouncilEnd {
			hasEnd = true
		}
	}
	if !hasStart {
		t.Error("expected EventKindCouncilStart event")
	}
	if !hasEnd {
		t.Error("expected EventKindCouncilEnd event")
	}
}

func TestEngine_ResumeCouncil(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	reg := setupRegistry()

	// First run: complete a council
	mock1 := &mockProvider{
		responses: []string{
			"Researcher: Initial analysis complete.",
			"Coder: Initial implementation plan ready.",
			"CONCLUDE",
			"Synthesis: Initial council concluded with a plan.",
		},
	}

	eng1 := NewEngine(store, reg, mock1, nil)

	result1, err := eng1.Run(context.Background(), CouncilConfig{
		Title:  "Resume Test Council",
		Topic:  "Initial topic: design a cache system",
		Roster: []string{"researcher", "coder"},
	})
	if err != nil {
		t.Fatalf("first Run() error: %v", err)
	}

	firstID := result1.ID

	// Resume with a new topic
	mock2 := &mockProvider{
		responses: []string{
			"Researcher: Updated analysis with new requirements.",
			"Coder: Revised implementation considering new constraints.",
			"CONCLUDE",
			"Synthesis: Council resumed and addressed new requirements.",
		},
	}

	eng2 := NewEngine(store, reg, mock2, nil)

	result2, err := eng2.Run(context.Background(), CouncilConfig{
		ID:     firstID,
		Topic:  "Follow-up: add TTL support to the cache",
		Roster: []string{"researcher", "coder"},
	})
	if err != nil {
		t.Fatalf("resume Run() error: %v", err)
	}

	// Verify same council ID
	if result2.ID != firstID {
		t.Errorf("expected same council ID %q, got %q", firstID, result2.ID)
	}

	// Verify transcript contains entries from both runs
	transcript, err := store.GetTranscript(result2.ID)
	if err != nil {
		t.Fatalf("GetTranscript() error: %v", err)
	}

	// Both runs should have entries in the transcript
	// First run: user topic + researcher + coder + moderator + synthesis = 5
	// Second run: user topic + researcher + coder + moderator + synthesis = 5
	// Total should be at least 8
	if len(transcript) < 8 {
		t.Errorf("expected at least 8 transcript entries from both runs, got %d", len(transcript))
	}

	// Verify second run has higher round numbers
	maxRound := 0
	for _, entry := range transcript {
		if entry.Round > maxRound {
			maxRound = entry.Round
		}
	}
	if maxRound < 2 {
		t.Errorf("expected max round >= 2 from resume, got %d", maxRound)
	}
}
