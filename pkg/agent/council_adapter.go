package agent

import (
	"context"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/council"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// councilRunnerAdapter bridges tools.CouncilRunner to council.Engine.
type councilRunnerAdapter struct {
	engine *council.Engine
}

func (a *councilRunnerAdapter) Run(ctx context.Context, cfg tools.CouncilRunConfig) (*tools.CouncilRunResult, error) {
	result, err := a.engine.Run(ctx, council.CouncilConfig{
		ID:           cfg.ID,
		Title:        cfg.Title,
		Description:  cfg.Description,
		Topic:        cfg.Topic,
		Roster:       cfg.Roster,
		AgentContext: cfg.AgentContext,
	})
	if err != nil {
		return nil, err
	}
	tasks := make([]tools.CouncilTask, len(result.Tasks))
	for i, t := range result.Tasks {
		tasks[i] = tools.CouncilTask{
			Agent:    t.Agent,
			Task:     t.Task,
			Priority: t.Priority,
		}
	}

	return &tools.CouncilRunResult{
		ID:        result.ID,
		Title:     result.Title,
		Rounds:    result.Rounds,
		Synthesis: result.Synthesis,
		Tasks:     tasks,
		Status:    result.Status,
	}, nil
}

// councilListerAdapter bridges tools.CouncilLister to council.Store.
type councilListerAdapter struct {
	store *council.Store
}

func (a *councilListerAdapter) List() ([]*tools.CouncilMetaSummary, error) {
	metas, err := a.store.List()
	if err != nil {
		return nil, err
	}
	var result []*tools.CouncilMetaSummary
	for _, m := range metas {
		result = append(result, &tools.CouncilMetaSummary{
			ID:     m.ID,
			Title:  m.Title,
			Status: m.Status,
			Rounds: m.Rounds,
		})
	}
	return result, nil
}

// councilEventEmitter bridges council.EventEmitter to agent.EventBus.
// It maps council-local event kinds to agent.EventKind constants and
// re-wraps payloads into agent.Event with proper timestamps.
type councilEventEmitter struct {
	bus *EventBus
}

func (e *councilEventEmitter) EmitCouncilEvent(evt council.Event) {
	if e.bus == nil {
		return
	}

	var kind EventKind
	var payload any

	switch evt.Kind {
	case council.EventCouncilStart:
		kind = EventKindCouncilStart
		if p, ok := evt.Payload.(council.CouncilStartPayload); ok {
			payload = CouncilStartPayload{
				CouncilID:   p.CouncilID,
				Title:       p.Title,
				Description: p.Description,
				Roster:      p.Roster,
			}
		}
	case council.EventCouncilRoundStart:
		kind = EventKindCouncilRoundStart
		if p, ok := evt.Payload.(council.CouncilRoundStartPayload); ok {
			payload = CouncilRoundStartPayload{
				CouncilID: p.CouncilID,
				Round:     p.Round,
			}
		}
	case council.EventCouncilAgentStart:
		kind = EventKindCouncilAgentStart
		if p, ok := evt.Payload.(council.CouncilAgentStartPayload); ok {
			payload = CouncilAgentStartPayload{
				CouncilID: p.CouncilID,
				AgentID:   p.AgentID,
				AgentType: p.AgentType,
				Round:     p.Round,
			}
		}
	case council.EventCouncilAgentDelta:
		kind = EventKindCouncilAgentDelta
		if p, ok := evt.Payload.(council.CouncilAgentDeltaPayload); ok {
			payload = CouncilAgentDeltaPayload{
				CouncilID: p.CouncilID,
				AgentID:   p.AgentID,
				Delta:     p.Delta,
			}
		}
	case council.EventCouncilAgentEnd:
		kind = EventKindCouncilAgentEnd
		if p, ok := evt.Payload.(council.CouncilAgentEndPayload); ok {
			payload = CouncilAgentEndPayload{
				CouncilID: p.CouncilID,
				AgentID:   p.AgentID,
				Content:   p.Content,
				Round:     p.Round,
			}
		}
	case council.EventCouncilRoundEnd:
		kind = EventKindCouncilRoundEnd
		if p, ok := evt.Payload.(council.CouncilRoundEndPayload); ok {
			payload = CouncilRoundEndPayload{
				CouncilID: p.CouncilID,
				Round:     p.Round,
				Decision:  p.Decision,
			}
		}
	case council.EventCouncilEnd:
		kind = EventKindCouncilEnd
		if p, ok := evt.Payload.(council.CouncilEndPayload); ok {
			payload = CouncilEndPayload{
				CouncilID: p.CouncilID,
				Rounds:    p.Rounds,
				Synthesis: p.Synthesis,
				Status:    p.Status,
			}
		}
	default:
		return
	}

	if payload == nil {
		payload = evt.Payload
	}

	e.bus.Emit(Event{
		Kind:    kind,
		Time:    time.Now(),
		Payload: payload,
	})
}
