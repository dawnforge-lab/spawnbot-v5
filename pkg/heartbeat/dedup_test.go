package heartbeat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDedup_FirstMessageIsNotDuplicate(t *testing.T) {
	d := NewDedup(24 * time.Hour)
	assert.False(t, d.IsDuplicate("alert: disk full"))
}

func TestDedup_IdenticalMessageWithinTTLIsDuplicate(t *testing.T) {
	d := NewDedup(24 * time.Hour)
	d.IsDuplicate("alert: disk full")
	assert.True(t, d.IsDuplicate("alert: disk full"))
}

func TestDedup_DifferentMessageIsNotDuplicate(t *testing.T) {
	d := NewDedup(24 * time.Hour)
	d.IsDuplicate("alert: disk full")
	assert.False(t, d.IsDuplicate("alert: memory low"))
}

func TestDedup_ExpiredEntryIsNotDuplicate(t *testing.T) {
	d := NewDedup(1 * time.Millisecond)
	d.IsDuplicate("alert: disk full")
	time.Sleep(5 * time.Millisecond)
	assert.False(t, d.IsDuplicate("alert: disk full"))
}

func TestDedup_EmptyStringNotTracked(t *testing.T) {
	d := NewDedup(24 * time.Hour)
	assert.False(t, d.IsDuplicate(""))
	assert.False(t, d.IsDuplicate(""))
}
