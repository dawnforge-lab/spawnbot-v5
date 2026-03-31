package tasks

import (
	"crypto/rand"
	"fmt"
	"time"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"

	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"

	ttlDays = 7
)

var validStatuses = map[string]bool{
	StatusPending: true, StatusInProgress: true,
	StatusCompleted: true, StatusFailed: true,
}

var validPriorities = map[string]bool{
	PriorityLow: true, PriorityMedium: true, PriorityHigh: true,
}

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	AgentType   string `json:"agent_type,omitempty"`
	Result      string `json:"result,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (t *Task) Validate() error {
	if t.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if t.Status != "" && !validStatuses[t.Status] {
		return fmt.Errorf("invalid status %q. Valid: pending, in_progress, completed, failed", t.Status)
	}
	if t.Priority != "" && !validPriorities[t.Priority] {
		return fmt.Errorf("invalid priority %q. Valid: low, medium, high", t.Priority)
	}
	return nil
}

func (t *Task) ApplyDefaults() {
	if t.Priority == "" {
		t.Priority = PriorityMedium
	}
	if t.Status == "" {
		t.Status = StatusPending
	}
	now := time.Now().UnixMilli()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	if t.UpdatedAt == 0 {
		t.UpdatedAt = now
	}
}

func IsTerminal(status string) bool {
	return status == StatusCompleted || status == StatusFailed
}

func IsExpired(task *Task) bool {
	if !IsTerminal(task.Status) {
		return false
	}
	cutoff := time.Now().Add(-ttlDays * 24 * time.Hour).UnixMilli()
	return task.UpdatedAt < cutoff
}

func GenerateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
