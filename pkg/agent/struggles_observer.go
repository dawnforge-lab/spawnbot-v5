package agent

import (
	"context"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/struggles"
)

// StrugglesObserver adapts a struggles.Collector to the EventObserver interface.
// Mount it on the HookManager to receive agent-loop events.
type StrugglesObserver struct {
	collector *struggles.Collector
}

// NewStrugglesObserver creates a new StrugglesObserver wrapping the given collector.
func NewStrugglesObserver(collector *struggles.Collector) *StrugglesObserver {
	return &StrugglesObserver{collector: collector}
}

// OnEvent dispatches agent events to the appropriate collector handler.
func (so *StrugglesObserver) OnEvent(ctx context.Context, evt Event) error {
	switch evt.Kind {
	case EventKindTurnStart:
		if payload, ok := evt.Payload.(TurnStartPayload); ok {
			so.collector.HandleTurnStart(payload.UserMessage, evt.Meta.SessionKey)
		}
	case EventKindToolExecEnd:
		if payload, ok := evt.Payload.(ToolExecEndPayload); ok {
			so.collector.HandleToolEnd(payload.Tool, payload.IsError, payload.ErrorMessage, evt.Meta.SessionKey)
		}
	case EventKindTurnEnd:
		so.collector.HandleTurnEnd(evt.Meta.SessionKey)
	}
	return nil
}
