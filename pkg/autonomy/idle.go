package autonomy

import (
	"context"
	"sync"
	"time"
)

// IdleCallback is called when a channel has been idle for longer than the threshold.
type IdleCallback func(channel string)

// IdleMonitor tracks per-channel activity and fires a callback once per idle period.
// It is safe for concurrent use.
type IdleMonitor struct {
	threshold    time.Duration
	callback     IdleCallback
	mu           sync.Mutex
	lastActivity map[string]time.Time
	firedFor     map[string]bool // prevent repeat fires until new activity
}

// NewIdleMonitor creates a new IdleMonitor with the given threshold and callback.
func NewIdleMonitor(threshold time.Duration, callback IdleCallback) *IdleMonitor {
	return &IdleMonitor{
		threshold:    threshold,
		callback:     callback,
		lastActivity: make(map[string]time.Time),
		firedFor:     make(map[string]bool),
	}
}

// Start launches the background polling goroutine. It stops when ctx is cancelled.
// The check interval is threshold/4, capped at 1 minute.
func (m *IdleMonitor) Start(ctx context.Context) {
	interval := m.threshold / 4
	if interval > time.Minute {
		interval = time.Minute
	}
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				m.checkIdle(now)
			}
		}
	}()
}

// checkIdle fires the callback for any channel that has been idle past the threshold
// and has not already been fired for this idle period.
func (m *IdleMonitor) checkIdle(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for ch, last := range m.lastActivity {
		if m.firedFor[ch] {
			continue
		}
		if now.Sub(last) >= m.threshold {
			m.firedFor[ch] = true
			ch := ch // capture for goroutine
			go m.callback(ch)
		}
	}
}

// RecordActivity marks the channel as active now, resetting the idle timer.
// Any pending fired state is cleared so the callback can fire again after
// the next full idle period.
func (m *IdleMonitor) RecordActivity(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActivity[channel] = time.Now()
	delete(m.firedFor, channel)
}

// Stop is a no-op kept for API completeness; use context cancellation to stop.
func (m *IdleMonitor) Stop() {}
