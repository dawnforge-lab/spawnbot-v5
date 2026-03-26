package autonomy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIdleMonitor_FiresAfterThreshold(t *testing.T) {
	fired := make(chan string, 1)
	monitor := NewIdleMonitor(100*time.Millisecond, func(channel string) {
		fired <- channel
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	monitor.Start(ctx)
	monitor.RecordActivity("telegram")

	select {
	case ch := <-fired:
		assert.Equal(t, "telegram", ch)
	case <-time.After(2 * time.Second):
		t.Fatal("idle trigger did not fire")
	}
}

func TestIdleMonitor_ResetOnActivity(t *testing.T) {
	fireCount := 0
	monitor := NewIdleMonitor(200*time.Millisecond, func(channel string) {
		fireCount++
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	monitor.Start(ctx)
	monitor.RecordActivity("telegram")

	// Activity before threshold — should reset
	time.Sleep(100 * time.Millisecond)
	monitor.RecordActivity("telegram")

	// Wait past original threshold but not the reset one
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, fireCount) // Should not have fired yet
}

func TestIdleMonitor_MultipleChannels(t *testing.T) {
	channels := make(chan string, 5)
	monitor := NewIdleMonitor(100*time.Millisecond, func(channel string) {
		channels <- channel
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	monitor.Start(ctx)
	monitor.RecordActivity("telegram")
	monitor.RecordActivity("discord")

	received := map[string]bool{}
	timeout := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case ch := <-channels:
			received[ch] = true
		case <-timeout:
			t.Fatal("not all channels fired")
		}
	}
	assert.True(t, received["telegram"])
	assert.True(t, received["discord"])
}
