# Heartbeat Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the spawnbot heartbeat system with deduplication, HEARTBEAT_OK suppression, wake retry, structured events, per-channel visibility, session timestamp restoration, and a CLI command to set heartbeat interval — inspired by Paperclip's heartbeat architecture.

**Architecture:** Add deduplication state, event emission, visibility config, and retry logic to the existing `pkg/heartbeat/` package. Add a `heartbeat` CLI subcommand under `cmd/spawnbot/internal/heartbeat/`. The heartbeat service gains a `SetInterval` method and the ticker is restarted on interval change. All new features are opt-in or backward-compatible.

**Tech Stack:** Go, Cobra CLI, testify, existing `pkg/fileutil`, `pkg/bus`, `pkg/state`, `pkg/config` packages.

---

## File Structure

| File | Responsibility |
|------|---------------|
| `pkg/heartbeat/service.go` | Modify: add dedup state, HEARTBEAT_OK suppression, SetInterval, retry, session restore |
| `pkg/heartbeat/events.go` | Create: structured HeartbeatEvent type, emitter, listener registry |
| `pkg/heartbeat/dedup.go` | Create: deduplication logic (content hash, 24h TTL) |
| `pkg/heartbeat/dedup_test.go` | Create: dedup unit tests |
| `pkg/heartbeat/events_test.go` | Create: event emission tests |
| `pkg/heartbeat/service_test.go` | Modify: add tests for new features |
| `pkg/config/config.go` | Modify: add HeartbeatVisibility to config |
| `cmd/spawnbot/internal/heartbeat/command.go` | Create: `heartbeat` CLI with `set-interval` and `last` subcommands |
| `cmd/spawnbot/main.go` | Modify: register heartbeat command |

---

### Task 1: Deduplication Module

**Files:**
- Create: `pkg/heartbeat/dedup.go`
- Create: `pkg/heartbeat/dedup_test.go`

- [ ] **Step 1: Write the failing test for dedup**

Create `pkg/heartbeat/dedup_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/heartbeat/ -run TestDedup -v`
Expected: compilation error — `NewDedup` not defined

- [ ] **Step 3: Write the implementation**

Create `pkg/heartbeat/dedup.go`:

```go
package heartbeat

import (
	"crypto/sha256"
	"sync"
	"time"
)

// Dedup tracks recently sent heartbeat messages to avoid sending identical
// alerts within a configurable TTL window.
type Dedup struct {
	ttl   time.Duration
	mu    sync.Mutex
	seen  map[[32]byte]time.Time
}

// NewDedup creates a deduplicator with the given TTL.
func NewDedup(ttl time.Duration) *Dedup {
	return &Dedup{
		ttl:  ttl,
		seen: make(map[[32]byte]time.Time),
	}
}

// IsDuplicate returns true if the same content was seen within the TTL window.
// Empty strings are never considered duplicates.
func (d *Dedup) IsDuplicate(content string) bool {
	if content == "" {
		return false
	}

	hash := sha256.Sum256([]byte(content))
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Prune expired entries opportunistically
	for k, v := range d.seen {
		if now.Sub(v) > d.ttl {
			delete(d.seen, k)
		}
	}

	if sentAt, ok := d.seen[hash]; ok {
		if now.Sub(sentAt) <= d.ttl {
			return true
		}
	}

	d.seen[hash] = now
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/heartbeat/ -run TestDedup -v`
Expected: all 5 tests PASS

- [ ] **Step 5: Commit**

```bash
cd /home/eugen-dev/Workflows/picoclaw
git add pkg/heartbeat/dedup.go pkg/heartbeat/dedup_test.go
git commit -m "feat(heartbeat): add message deduplication with 24h TTL"
```

---

### Task 2: Structured Heartbeat Events

**Files:**
- Create: `pkg/heartbeat/events.go`
- Create: `pkg/heartbeat/events_test.go`

- [ ] **Step 1: Write the failing test for events**

Create `pkg/heartbeat/events_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/heartbeat/ -run TestEventEmitter -v`
Expected: compilation error — `NewEventEmitter` not defined

- [ ] **Step 3: Write the implementation**

Create `pkg/heartbeat/events.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/heartbeat/ -run TestEventEmitter -v`
Expected: all 4 tests PASS

- [ ] **Step 5: Commit**

```bash
cd /home/eugen-dev/Workflows/picoclaw
git add pkg/heartbeat/events.go pkg/heartbeat/events_test.go
git commit -m "feat(heartbeat): add structured event emission with listener support"
```

---

### Task 3: Integrate Dedup, Events, HEARTBEAT_OK Suppression, and Retry into Service

**Files:**
- Modify: `pkg/heartbeat/service.go`
- Modify: `pkg/heartbeat/service_test.go`

- [ ] **Step 1: Write the failing tests for new service features**

Append to `pkg/heartbeat/service_test.go`:

```go
func TestExecuteHeartbeat_DeduplicatesIdenticalAlerts(t *testing.T) {
	tmpDir := t.TempDir()
	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.stopChan = make(chan struct{})

	var sentMessages []string
	hs.SetBus(createTestBus(t))
	hs.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		return &tools.ToolResult{ForUser: "alert: disk full", ForLLM: "alert: disk full"}
	})
	// Track sent messages via a test event listener
	hs.Events().OnEvent(func(ev HeartbeatEvent) {
		if ev.Status == EventStatusSent {
			sentMessages = append(sentMessages, ev.Preview)
		}
	})

	os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check disk"), 0o644)

	hs.executeHeartbeat()
	hs.executeHeartbeat() // second call should be deduped

	assert.Equal(t, 1, len(sentMessages), "duplicate alert should be suppressed")
}

func TestExecuteHeartbeat_SuppressesHeartbeatOKResponse(t *testing.T) {
	tmpDir := t.TempDir()
	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.stopChan = make(chan struct{})

	hs.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		return &tools.ToolResult{ForLLM: "HEARTBEAT_OK", ForUser: "HEARTBEAT_OK"}
	})

	var lastEvent *HeartbeatEvent
	hs.Events().OnEvent(func(ev HeartbeatEvent) {
		lastEvent = &ev
	})

	os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check status"), 0o644)
	hs.executeHeartbeat()

	require.NotNil(t, lastEvent)
	assert.Equal(t, EventStatusOK, lastEvent.Status)
}

func TestExecuteHeartbeat_EmitsEventOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.stopChan = make(chan struct{})

	hs.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		return &tools.ToolResult{ForUser: "alert: something happened", ForLLM: "alert"}
	})

	var lastEvent *HeartbeatEvent
	hs.Events().OnEvent(func(ev HeartbeatEvent) {
		lastEvent = &ev
	})

	os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check things"), 0o644)
	hs.executeHeartbeat()

	require.NotNil(t, lastEvent)
	assert.Equal(t, EventStatusSent, lastEvent.Status)
	assert.Greater(t, lastEvent.DurationMs, int64(0))
}

func TestSetInterval_UpdatesTicker(t *testing.T) {
	tmpDir := t.TempDir()
	hs := NewHeartbeatService(tmpDir, 30, true)

	hs.SetInterval(10)
	assert.Equal(t, 10*time.Minute, hs.interval)

	// Minimum enforcement
	hs.SetInterval(2)
	assert.Equal(t, 5*time.Minute, hs.interval)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/heartbeat/ -run "TestExecuteHeartbeat_Dedup|TestExecuteHeartbeat_Suppresses|TestExecuteHeartbeat_EmitsEvent|TestSetInterval" -v`
Expected: compilation errors — `Events()`, `SetInterval` not defined

- [ ] **Step 3: Add imports to test file**

Add `"github.com/stretchr/testify/assert"` and `"github.com/stretchr/testify/require"` to the imports in `service_test.go`. These are needed by the new tests.

- [ ] **Step 4: Modify service.go — add new fields and methods**

Add to the `HeartbeatService` struct (after `stopChan`):

```go
	dedup    *Dedup
	events   *EventEmitter
	retryCh  chan struct{} // signals a retry after main-busy skip
```

Update `NewHeartbeatService` to initialize them:

```go
	return &HeartbeatService{
		workspace: workspace,
		interval:  time.Duration(intervalMinutes) * time.Minute,
		enabled:   enabled,
		state:     state.NewManager(workspace),
		dedup:     NewDedup(24 * time.Hour),
		events:    NewEventEmitter(),
		retryCh:   make(chan struct{}, 1),
	}
```

Add the `Events()` accessor:

```go
// Events returns the event emitter for subscribing to heartbeat events.
func (hs *HeartbeatService) Events() *EventEmitter {
	return hs.events
}
```

Add the `SetInterval` method:

```go
// SetInterval updates the heartbeat interval in minutes. Restarts the ticker
// if the service is running. Enforces the minimum interval.
func (hs *HeartbeatService) SetInterval(minutes int) {
	if minutes < minIntervalMinutes {
		minutes = minIntervalMinutes
	}

	hs.mu.Lock()
	hs.interval = time.Duration(minutes) * time.Minute
	wasRunning := hs.stopChan != nil
	hs.mu.Unlock()

	if wasRunning {
		hs.Stop()
		hs.Start()
	}
}
```

- [ ] **Step 5: Modify executeHeartbeat — add dedup, HEARTBEAT_OK suppression, events, retry**

Replace the `executeHeartbeat` method in `service.go` with:

```go
func (hs *HeartbeatService) executeHeartbeat() {
	start := time.Now()

	hs.mu.RLock()
	handler := hs.handler
	if !hs.enabled || hs.stopChan == nil {
		hs.mu.RUnlock()
		return
	}
	hs.mu.RUnlock()

	logger.DebugC("heartbeat", "Executing heartbeat")

	prompt := hs.buildPrompt()
	if prompt == "" {
		hs.events.Emit(HeartbeatEvent{
			Timestamp:  start,
			Status:     EventStatusSkipped,
			SkipReason: "empty-heartbeat-file",
		})
		logger.InfoC("heartbeat", "No heartbeat prompt (HEARTBEAT.md empty or missing)")
		return
	}

	if handler == nil {
		hs.logErrorf("Heartbeat handler not configured")
		hs.events.Emit(HeartbeatEvent{
			Timestamp:  start,
			Status:     EventStatusFailed,
			SkipReason: "no-handler",
		})
		return
	}

	lastChannel := hs.state.GetLastChannel()
	channel, chatID := hs.parseLastChannel(lastChannel)
	hs.logInfof("Resolved channel: %s, chatID: %s (from lastChannel: %s)", channel, chatID, lastChannel)

	result := handler(prompt, channel, chatID)
	durationMs := time.Since(start).Milliseconds()

	if result == nil {
		hs.logInfof("Heartbeat handler returned nil result")
		hs.events.Emit(HeartbeatEvent{
			Timestamp:  start,
			Status:     EventStatusFailed,
			DurationMs: durationMs,
			SkipReason: "nil-result",
		})
		return
	}

	if result.IsError {
		hs.logErrorf("Heartbeat error: %s", result.ForLLM)
		hs.events.Emit(HeartbeatEvent{
			Timestamp:  start,
			Status:     EventStatusFailed,
			DurationMs: durationMs,
			SkipReason: result.ForLLM,
		})
		return
	}

	if result.Async {
		hs.logInfof("Async task started: %s", result.ForLLM)
		hs.events.Emit(HeartbeatEvent{
			Timestamp:  start,
			Status:     EventStatusSent,
			DurationMs: durationMs,
			Preview:    truncatePreview(result.ForLLM, 200),
			Channel:    channel,
		})
		return
	}

	// HEARTBEAT_OK suppression: if the response is just HEARTBEAT_OK, suppress delivery
	responseText := result.ForUser
	if responseText == "" {
		responseText = result.ForLLM
	}

	if isHeartbeatOK(responseText) || result.Silent {
		hs.logInfof("Heartbeat OK - silent")
		hs.events.Emit(HeartbeatEvent{
			Timestamp:  start,
			Status:     EventStatusOK,
			DurationMs: durationMs,
			Channel:    channel,
		})
		return
	}

	// Deduplication: skip if identical message sent within TTL
	if hs.dedup.IsDuplicate(responseText) {
		hs.logInfof("Heartbeat skipped - duplicate alert within 24h")
		hs.events.Emit(HeartbeatEvent{
			Timestamp:  start,
			Status:     EventStatusSkipped,
			DurationMs: durationMs,
			SkipReason: "duplicate",
			Preview:    truncatePreview(responseText, 200),
			Channel:    channel,
		})
		return
	}

	hs.sendResponse(responseText)
	hs.logInfof("Heartbeat completed: %s", responseText)
	hs.events.Emit(HeartbeatEvent{
		Timestamp:  start,
		Status:     EventStatusSent,
		DurationMs: durationMs,
		Preview:    truncatePreview(responseText, 200),
		Channel:    channel,
	})
}
```

Add these helper functions at the bottom of `service.go`:

```go
// isHeartbeatOK returns true if the response is effectively just HEARTBEAT_OK.
func isHeartbeatOK(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.EqualFold(trimmed, "HEARTBEAT_OK") ||
		strings.EqualFold(trimmed, "Heartbeat OK")
}

// truncatePreview truncates a string to maxLen, appending "..." if truncated.
func truncatePreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

- [ ] **Step 6: Update runLoop to listen for retry signals**

Replace the `runLoop` method:

```go
func (hs *HeartbeatService) runLoop(stopChan chan struct{}) {
	ticker := time.NewTicker(hs.interval)
	defer ticker.Stop()

	// Run first heartbeat after initial delay
	time.AfterFunc(time.Second, func() {
		hs.executeHeartbeat()
	})

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			hs.executeHeartbeat()
		case <-hs.retryCh:
			hs.executeHeartbeat()
		}
	}
}
```

- [ ] **Step 7: Run all heartbeat tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/heartbeat/ -v`
Expected: all tests PASS (old and new)

- [ ] **Step 8: Commit**

```bash
cd /home/eugen-dev/Workflows/picoclaw
git add pkg/heartbeat/service.go pkg/heartbeat/service_test.go
git commit -m "feat(heartbeat): integrate dedup, HEARTBEAT_OK suppression, events, SetInterval, and retry"
```

---

### Task 4: CLI Command — `heartbeat set-interval` and `heartbeat last`

**Files:**
- Create: `cmd/spawnbot/internal/heartbeat/command.go`
- Modify: `cmd/spawnbot/main.go`

- [ ] **Step 1: Create the heartbeat CLI command**

Create `cmd/spawnbot/internal/heartbeat/command.go`:

```go
package heartbeat

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

func NewHeartbeatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heartbeat",
		Short: "Manage heartbeat service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newSetIntervalCommand(),
		newStatusCommand(),
	)

	return cmd
}

func newSetIntervalCommand() *cobra.Command {
	var minutes int

	cmd := &cobra.Command{
		Use:   "set-interval",
		Short: "Set heartbeat interval in minutes (minimum 5)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if minutes < 5 {
				return fmt.Errorf("interval must be at least 5 minutes, got %d", minutes)
			}

			configPath := internal.GetConfigPath()
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}

			cfg.Heartbeat.Interval = minutes
			if err := config.SaveConfig(configPath, cfg); err != nil {
				return fmt.Errorf("error saving config: %w", err)
			}

			fmt.Printf("Heartbeat interval set to %d minutes.\n", minutes)
			fmt.Println("Restart the gateway for changes to take effect.")
			return nil
		},
	}

	cmd.Flags().IntVarP(&minutes, "minutes", "m", 30, "Interval in minutes (min 5)")
	cmd.MarkFlagRequired("minutes")

	return cmd
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current heartbeat configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := internal.LoadConfig()
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}

			info := map[string]any{
				"enabled":          cfg.Heartbeat.Enabled,
				"interval_minutes": cfg.Heartbeat.Interval,
			}

			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}
```

- [ ] **Step 2: Register the command in main.go**

In `cmd/spawnbot/main.go`, add the import:

```go
"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/heartbeat"
```

Add to the `cmd.AddCommand(...)` block:

```go
heartbeat.NewHeartbeatCommand(),
```

- [ ] **Step 3: Build to verify compilation**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go build ./cmd/spawnbot/`
Expected: successful build, no errors

- [ ] **Step 4: Test CLI help output**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go run ./cmd/spawnbot/ heartbeat --help`
Expected: shows `set-interval` and `status` subcommands

- [ ] **Step 5: Commit**

```bash
cd /home/eugen-dev/Workflows/picoclaw
git add cmd/spawnbot/internal/heartbeat/command.go cmd/spawnbot/main.go
git commit -m "feat(cli): add heartbeat command with set-interval and status"
```

---

### Task 5: Ensure Heartbeat Runs — Verify Gateway Integration

**Files:**
- Modify: `pkg/gateway/gateway.go` (if needed)

- [ ] **Step 1: Verify the gateway starts heartbeat correctly**

Read `pkg/gateway/gateway.go` lines 281-291 and confirm the heartbeat service is initialized with config values, handler is set, and `Start()` is called. This is already done — verify no changes needed.

- [ ] **Step 2: Verify the heartbeat is enabled in the user's config**

Run: `cd /home/eugen-dev/Workflows/picoclaw && cat ~/.spawnbot/config.json | grep -A2 heartbeat`
Expected: `"enabled": true` and an interval value. If not, note that user should set `"enabled": true` in their config or run `spawnbot heartbeat set-interval -m 30`.

- [ ] **Step 3: Run the full test suite**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/heartbeat/ -v`
Expected: all tests pass

- [ ] **Step 4: Build the binary**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go build -o build/spawnbot ./cmd/spawnbot/`
Expected: successful build

- [ ] **Step 5: Commit if any gateway changes were needed**

Only commit if changes were made. Otherwise skip.
