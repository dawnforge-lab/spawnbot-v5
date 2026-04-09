package council

import (
	"context"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
)

func TestIntegration_CouncilWithEvents(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agentRegistry := agents.NewRegistry()
	agentRegistry.Register(&agents.AgentDefinition{
		Name:         "researcher",
		SystemPrompt: "You are a researcher.",
	})

	emitter := &mockEventEmitter{}

	provider := &mockProvider{
		responses: []string{
			"My research shows the topic is well-covered.",
			"CONCLUDE - sufficient input gathered.",
			"Synthesis of findings: the research is comprehensive.",
		},
	}

	engine := NewEngine(store, agentRegistry, provider, emitter)

	result, err := engine.Run(context.Background(), CouncilConfig{
		Title:  "Event Test",
		Topic:  "Test topic",
		Roster: []string{"researcher"},
		Model:  "test-model",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Status != StatusClosed {
		t.Fatalf("expected closed, got %s", result.Status)
	}

	// Verify we got key council events
	events := emitter.allEvents()
	var gotStart, gotAgentEnd, gotEnd bool
	for _, evt := range events {
		switch evt.Kind {
		case EventCouncilStart:
			gotStart = true
		case EventCouncilAgentEnd:
			gotAgentEnd = true
		case EventCouncilEnd:
			gotEnd = true
		}
	}

	if !gotStart {
		t.Error("expected council_start event")
	}
	if !gotAgentEnd {
		t.Error("expected council_agent_end event")
	}
	if !gotEnd {
		t.Error("expected council_end event")
	}

	// Verify event ordering: start should come before end
	var startIdx, endIdx int
	for i, evt := range events {
		if evt.Kind == EventCouncilStart {
			startIdx = i
		}
		if evt.Kind == EventCouncilEnd {
			endIdx = i
		}
	}
	if startIdx >= endIdx {
		t.Errorf("expected start event (idx %d) before end event (idx %d)", startIdx, endIdx)
	}
}
