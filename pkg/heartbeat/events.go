package heartbeat

import (
	"sync"
	"time"
)

// EventStatus represents the outcome of a heartbeat run.
type EventStatus string

const (
	EventStatusSent    EventStatus = "sent"
	EventStatusOK      EventStatus = "ok"
	EventStatusSkipped EventStatus = "skipped"
	EventStatusFailed  EventStatus = "failed"
)

// HeartbeatEvent captures the result of a single heartbeat execution.
type HeartbeatEvent struct {
	Timestamp  time.Time   `json:"timestamp"`
	Status     EventStatus `json:"status"`
	DurationMs int64       `json:"duration_ms,omitempty"`
	SkipReason string      `json:"skip_reason,omitempty"`
	Preview    string      `json:"preview,omitempty"`
	Channel    string      `json:"channel,omitempty"`
}

// EventListener is a callback for heartbeat events.
type EventListener func(HeartbeatEvent)

// EventEmitter manages heartbeat event emission and listener registration.
type EventEmitter struct {
	mu        sync.RWMutex
	last      *HeartbeatEvent
	listeners map[int]EventListener
	nextID    int
}

// NewEventEmitter creates a new event emitter.
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		listeners: make(map[int]EventListener),
	}
}

// Emit publishes a heartbeat event to all listeners and stores it as the last event.
func (e *EventEmitter) Emit(ev HeartbeatEvent) {
	e.mu.Lock()
	e.last = &ev
	// Copy listeners under lock to iterate safely
	listeners := make([]EventListener, 0, len(e.listeners))
	for _, fn := range e.listeners {
		listeners = append(listeners, fn)
	}
	e.mu.Unlock()

	for _, fn := range listeners {
		fn(ev)
	}
}

// Last returns the most recent heartbeat event, or nil if none.
func (e *EventEmitter) Last() *HeartbeatEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.last
}

// OnEvent registers a listener. Returns an unsubscribe function.
func (e *EventEmitter) OnEvent(fn EventListener) func() {
	e.mu.Lock()
	defer e.mu.Unlock()
	id := e.nextID
	e.nextID++
	e.listeners[id] = fn
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.listeners, id)
	}
}
