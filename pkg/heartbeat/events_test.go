package heartbeat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventEmitter_EmitAndGetLast(t *testing.T) {
	e := NewEventEmitter()
	ev := HeartbeatEvent{
		Timestamp:  time.Now(),
		Status:     EventStatusSent,
		DurationMs: 150,
		Preview:    "alert: disk 90%",
	}
	e.Emit(ev)

	last := e.Last()
	require.NotNil(t, last)
	assert.Equal(t, EventStatusSent, last.Status)
	assert.Equal(t, "alert: disk 90%", last.Preview)
}

func TestEventEmitter_LastReturnsNilWhenEmpty(t *testing.T) {
	e := NewEventEmitter()
	assert.Nil(t, e.Last())
}

func TestEventEmitter_ListenerReceivesEvents(t *testing.T) {
	e := NewEventEmitter()
	var received HeartbeatEvent
	e.OnEvent(func(ev HeartbeatEvent) {
		received = ev
	})

	e.Emit(HeartbeatEvent{Status: EventStatusSkipped, SkipReason: "duplicate"})
	assert.Equal(t, EventStatusSkipped, received.Status)
	assert.Equal(t, "duplicate", received.SkipReason)
}

func TestEventEmitter_RemoveListener(t *testing.T) {
	e := NewEventEmitter()
	callCount := 0
	unsub := e.OnEvent(func(ev HeartbeatEvent) {
		callCount++
	})

	e.Emit(HeartbeatEvent{Status: EventStatusOK})
	assert.Equal(t, 1, callCount)

	unsub()
	e.Emit(HeartbeatEvent{Status: EventStatusOK})
	assert.Equal(t, 1, callCount)
}
