package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/struggles"
)

func TestStrugglesObserver_ToolError(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	collector := struggles.NewCollector(logPath)
	observer := NewStrugglesObserver(collector)

	evt := Event{
		Kind: EventKindToolExecEnd,
		Time: time.Now(),
		Meta: EventMeta{SessionKey: "test-session"},
		Payload: ToolExecEndPayload{
			Tool:         "exec",
			IsError:      true,
			ErrorMessage: "command not found",
		},
	}
	if err := observer.OnEvent(context.Background(), evt); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}

	signals, err := struggles.ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Type != struggles.TypeToolError {
		t.Errorf("expected type %q, got %q", struggles.TypeToolError, signals[0].Type)
	}
}

func TestStrugglesObserver_TurnStart_Correction(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	collector := struggles.NewCollector(logPath)
	observer := NewStrugglesObserver(collector)

	evt := Event{
		Kind: EventKindTurnStart,
		Time: time.Now(),
		Meta: EventMeta{SessionKey: "test-session"},
		Payload: TurnStartPayload{
			UserMessage: "no that's wrong",
		},
	}
	if err := observer.OnEvent(context.Background(), evt); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}

	signals, err := struggles.ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Type != struggles.TypeUserCorrection {
		t.Errorf("expected type %q, got %q", struggles.TypeUserCorrection, signals[0].Type)
	}
}

func TestStrugglesObserver_TurnEnd_RepeatedTool(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	collector := struggles.NewCollector(logPath)
	observer := NewStrugglesObserver(collector)

	observer.OnEvent(context.Background(), Event{
		Kind:    EventKindTurnStart,
		Meta:    EventMeta{SessionKey: "test-session"},
		Payload: TurnStartPayload{UserMessage: "run exec a lot"},
	})

	for i := 0; i < 4; i++ {
		observer.OnEvent(context.Background(), Event{
			Kind:    EventKindToolExecEnd,
			Meta:    EventMeta{SessionKey: "test-session"},
			Payload: ToolExecEndPayload{Tool: "exec", IsError: false},
		})
	}

	observer.OnEvent(context.Background(), Event{
		Kind:    EventKindTurnEnd,
		Meta:    EventMeta{SessionKey: "test-session"},
		Payload: TurnEndPayload{},
	})

	signals, err := struggles.ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (repeated tool), got %d", len(signals))
	}
	if signals[0].Type != struggles.TypeRepeatedTool {
		t.Errorf("expected type %q, got %q", struggles.TypeRepeatedTool, signals[0].Type)
	}
	if signals[0].Count != 4 {
		t.Errorf("expected count 4, got %d", signals[0].Count)
	}
}

func TestStrugglesObserver_IgnoresIrrelevantEvents(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	collector := struggles.NewCollector(logPath)
	observer := NewStrugglesObserver(collector)

	evt := Event{
		Kind:    EventKindLLMRequest,
		Meta:    EventMeta{SessionKey: "test-session"},
		Payload: LLMRequestPayload{Model: "test"},
	}
	if err := observer.OnEvent(context.Background(), evt); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}

	signals, err := struggles.ReadLog(logPath)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for irrelevant event, got %d", len(signals))
	}
}
