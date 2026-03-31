// Spawnbot - Personal AI assistant
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package heartbeat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/bus"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/constants"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/fileutil"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/state"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/struggles"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tasks"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

const (
	minIntervalMinutes     = 5
	defaultIntervalMinutes = 30
	userTasksMarker        = "Add your heartbeat tasks below this line:"
)

// HeartbeatHandler is the function type for handling heartbeat.
// It returns a ToolResult that can indicate async operations.
// channel and chatID are derived from the last active user channel.
type HeartbeatHandler func(prompt, channel, chatID string) *tools.ToolResult

// HeartbeatService manages periodic heartbeat checks
type HeartbeatService struct {
	workspace string
	bus       *bus.MessageBus
	state     *state.Manager
	handler   HeartbeatHandler
	interval  time.Duration
	enabled   bool
	mu        sync.RWMutex
	stopChan  chan struct{}
	dedup     *Dedup
	events    *EventEmitter
	retryCh   chan struct{} // signals a retry after main-busy skip
	taskStore         *tasks.TaskStore
	selfImproveConfig config.SelfImproveConfig
}

// NewHeartbeatService creates a new heartbeat service
func NewHeartbeatService(workspace string, intervalMinutes int, enabled bool) *HeartbeatService {
	// Apply minimum interval
	if intervalMinutes < minIntervalMinutes && intervalMinutes != 0 {
		intervalMinutes = minIntervalMinutes
	}

	if intervalMinutes == 0 {
		intervalMinutes = defaultIntervalMinutes
	}

	return &HeartbeatService{
		workspace: workspace,
		interval:  time.Duration(intervalMinutes) * time.Minute,
		enabled:   enabled,
		state:     state.NewManager(workspace),
		dedup:     NewDedup(24 * time.Hour),
		events:    NewEventEmitter(),
		retryCh:   make(chan struct{}, 1),
	}
}

// Events returns the event emitter for subscribing to heartbeat events.
func (hs *HeartbeatService) Events() *EventEmitter {
	return hs.events
}

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

// SetBus sets the message bus for delivering heartbeat results.
func (hs *HeartbeatService) SetBus(msgBus *bus.MessageBus) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.bus = msgBus
}

// SetHandler sets the heartbeat handler.
func (hs *HeartbeatService) SetHandler(handler HeartbeatHandler) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.handler = handler
}

// SetTaskStore sets the task store for injecting pending tasks into the heartbeat prompt.
func (hs *HeartbeatService) SetTaskStore(s *tasks.TaskStore) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.taskStore = s
}

// Start begins the heartbeat service
func (hs *HeartbeatService) Start() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.stopChan != nil {
		logger.InfoC("heartbeat", "Heartbeat service already running")
		return nil
	}

	if !hs.enabled {
		logger.InfoC("heartbeat", "Heartbeat service disabled")
		return nil
	}

	hs.stopChan = make(chan struct{})
	go hs.runLoop(hs.stopChan)

	logger.InfoCF("heartbeat", "Heartbeat service started", map[string]any{
		"interval_minutes": hs.interval.Minutes(),
	})

	return nil
}

// Stop gracefully stops the heartbeat service
func (hs *HeartbeatService) Stop() {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.stopChan == nil {
		return
	}

	logger.InfoC("heartbeat", "Stopping heartbeat service")
	close(hs.stopChan)
	hs.stopChan = nil
}

// IsRunning returns whether the service is running
func (hs *HeartbeatService) IsRunning() bool {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.stopChan != nil
}

// runLoop runs the heartbeat ticker
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

	// Self-improvement review runs independently of the regular heartbeat outcome.
	defer hs.maybeSelfImprove()

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

// buildPrompt builds the heartbeat prompt from HEARTBEAT.md
func (hs *HeartbeatService) buildPrompt() string {
	heartbeatPath := filepath.Join(hs.workspace, "HEARTBEAT.md")

	data, err := os.ReadFile(heartbeatPath)
	if err != nil {
		if os.IsNotExist(err) {
			hs.createDefaultHeartbeatTemplate()
			return ""
		}
		hs.logErrorf("Error reading HEARTBEAT.md: %v", err)
		return ""
	}

	content := string(data)
	hasUserTasks := heartbeatHasUserTasks(content)

	taskContext := ""
	if hs.taskStore != nil {
		pending := hs.taskStore.PendingSummary()
		if pending != "" {
			taskContext = "\n\n# Pending Tasks\n\n" + pending +
				"\n\nReview these tasks. Follow up on any that need action — " +
				"start working on pending tasks, check on in_progress tasks, " +
				"or mark tasks as completed/failed."
		}
	}

	// Skip heartbeat only if there are no user tasks AND no pending tasks
	if !hasUserTasks && taskContext == "" {
		return ""
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	return fmt.Sprintf(`# Heartbeat Check

Current time: %s

You are a proactive AI assistant. This is a scheduled heartbeat check.
Review the following tasks and execute any necessary actions using available skills.
If there is nothing that requires attention, respond ONLY with: HEARTBEAT_OK

%s%s
`, now, content, taskContext)
}

// createDefaultHeartbeatTemplate creates the default HEARTBEAT.md file
func (hs *HeartbeatService) createDefaultHeartbeatTemplate() {
	heartbeatPath := filepath.Join(hs.workspace, "HEARTBEAT.md")

	defaultContent := `# Heartbeat Check List

This file contains tasks for the heartbeat service to check periodically.

## Examples

- Check for unread messages
- Review upcoming calendar events
- Check device status (e.g., MaixCam)

## Instructions

- Execute ALL tasks listed below. Do NOT skip any task.
- For simple tasks (e.g., report current time), respond directly.
- For complex tasks that may take time, use the spawn tool to create a subagent.
- The spawn tool is async - subagent results will be sent to the user automatically.
- After spawning a subagent, CONTINUE to process remaining tasks.
- Only respond with HEARTBEAT_OK when ALL tasks are done AND nothing needs attention.

---

Add your heartbeat tasks below this line:
`

	if err := fileutil.WriteFileAtomic(heartbeatPath, []byte(defaultContent), 0o644); err != nil {
		hs.logErrorf("Failed to create default HEARTBEAT.md: %v", err)
	} else {
		hs.logInfof("Created default HEARTBEAT.md template")
	}
}

func heartbeatHasUserTasks(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	markerIdx := strings.Index(content, userTasksMarker)
	if markerIdx < 0 {
		return true
	}

	tasksSection := content[markerIdx+len(userTasksMarker):]
	for _, line := range strings.Split(tasksSection, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		if strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		return true
	}

	return false
}

// sendResponse sends the heartbeat response to the last channel
func (hs *HeartbeatService) sendResponse(response string) {
	hs.mu.RLock()
	msgBus := hs.bus
	hs.mu.RUnlock()

	if msgBus == nil {
		hs.logInfof("No message bus configured, heartbeat result not sent")
		return
	}

	// Get last channel from state
	lastChannel := hs.state.GetLastChannel()
	if lastChannel == "" {
		hs.logInfof("No last channel recorded, heartbeat result not sent")
		return
	}

	platform, userID := hs.parseLastChannel(lastChannel)

	// Skip internal channels that can't receive messages
	if platform == "" || userID == "" {
		return
	}

	pubCtx, pubCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pubCancel()
	msgBus.PublishOutbound(pubCtx, bus.OutboundMessage{
		Channel: platform,
		ChatID:  userID,
		Content: response,
	})

	hs.logInfof("Heartbeat result sent to %s", platform)
}

// parseLastChannel parses the last channel string into platform and userID.
// Returns empty strings for invalid or internal channels.
func (hs *HeartbeatService) parseLastChannel(lastChannel string) (platform, userID string) {
	if lastChannel == "" {
		return "", ""
	}

	// Parse channel format: "platform:user_id" (e.g., "telegram:123456")
	parts := strings.SplitN(lastChannel, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		hs.logErrorf("Invalid last channel format: %s", lastChannel)
		return "", ""
	}

	platform, userID = parts[0], parts[1]

	// Skip internal channels
	if constants.IsInternalChannel(platform) {
		hs.logInfof("Skipping internal channel: %s", platform)
		return "", ""
	}

	return platform, userID
}

// logInfof logs an informational message to the heartbeat log
func (hs *HeartbeatService) logInfof(format string, args ...any) {
	hs.logf("INFO", format, args...)
}

// logErrorf logs an error message to the heartbeat log
func (hs *HeartbeatService) logErrorf(format string, args ...any) {
	hs.logf("ERROR", format, args...)
}

// logf writes a message to the heartbeat log file
func (hs *HeartbeatService) logf(level, format string, args ...any) {
	logFile := filepath.Join(hs.workspace, "heartbeat.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] [%s] %s\n", timestamp, level, fmt.Sprintf(format, args...))
}

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

// SetSelfImproveConfig sets the self-improvement configuration.
func (hs *HeartbeatService) SetSelfImproveConfig(cfg config.SelfImproveConfig) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if cfg.Hour < 0 || cfg.Hour > 23 {
		cfg.Hour = 3
	}
	if cfg.MaxCreations <= 0 {
		cfg.MaxCreations = 3
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 2
	}
	hs.selfImproveConfig = cfg
}

// shouldSelfImprove checks whether the self-improvement review should run.
func (hs *HeartbeatService) shouldSelfImprove(currentHour int) bool {
	if !hs.selfImproveConfig.Enabled {
		return false
	}
	if currentHour != hs.selfImproveConfig.Hour {
		return false
	}
	// Check if already ran today
	stateFile := filepath.Join(hs.workspace, "state", "last_self_improve.txt")
	data, err := os.ReadFile(stateFile)
	if err == nil {
		today := time.Now().Format("2006-01-02")
		if strings.TrimSpace(string(data)) == today {
			return false
		}
	}
	// Check if struggle log has entries
	logPath := filepath.Join(hs.workspace, "struggles.jsonl")
	info, err := os.Stat(logPath)
	if err != nil || info.Size() == 0 {
		return false
	}
	return true
}

// markSelfImproveRan records that the self-improvement review ran today.
func (hs *HeartbeatService) markSelfImproveRan() {
	stateDir := filepath.Join(hs.workspace, "state")
	os.MkdirAll(stateDir, 0o755)
	stateFile := filepath.Join(stateDir, "last_self_improve.txt")
	today := time.Now().Format("2006-01-02")
	os.WriteFile(stateFile, []byte(today), 0o644)
}

// maybeSelfImprove checks conditions and runs the self-improvement review if appropriate.
func (hs *HeartbeatService) maybeSelfImprove() {
	currentHour := time.Now().Hour()
	if !hs.shouldSelfImprove(currentHour) {
		return
	}

	logPath := filepath.Join(hs.workspace, "struggles.jsonl")
	content, err := struggles.ReadLogContent(logPath, struggles.MaxLogBytes)
	if err != nil {
		hs.logErrorf("Failed to read struggle log: %v", err)
		return
	}
	if content == "" {
		return
	}

	hs.mu.RLock()
	handler := hs.handler
	hs.mu.RUnlock()

	if handler == nil {
		hs.logErrorf("Self-improve: no handler configured")
		return
	}

	prompt := hs.buildSelfImprovePrompt(content)
	lastChannel := hs.state.GetLastChannel()
	channel, chatID := hs.parseLastChannel(lastChannel)

	hs.logInfof("Starting self-improvement review")
	hs.events.Emit(HeartbeatEvent{
		Timestamp: time.Now(),
		Status:    EventStatusSent,
		Preview:   "Self-improvement review started",
		Channel:   channel,
	})

	result := handler(prompt, channel, chatID)

	if result != nil && !result.IsError {
		if err := struggles.TruncateLog(logPath); err != nil {
			hs.logErrorf("Failed to truncate struggle log: %v", err)
		}
		hs.markSelfImproveRan()
		hs.logInfof("Self-improvement review completed")

		responseText := result.ForUser
		if responseText == "" {
			responseText = result.ForLLM
		}
		if responseText != "" && !isHeartbeatOK(responseText) && !result.Silent {
			hs.sendResponse(responseText)
		}
	} else {
		errMsg := "unknown error"
		if result != nil {
			errMsg = result.ForLLM
		}
		hs.logErrorf("Self-improvement review failed: %s", errMsg)
		hs.sendResponse(fmt.Sprintf("Self-improvement review failed: %s. Signals carried over to next run.", errMsg))
	}
}

// buildSelfImprovePrompt constructs the prompt for the self-improvement agent.
func (hs *HeartbeatService) buildSelfImprovePrompt(struggleLog string) string {
	now := time.Now().Format("2006-01-02 15:04:05")
	maxCreations := hs.selfImproveConfig.MaxCreations
	if maxCreations <= 0 {
		maxCreations = 3
	}
	maxRetries := hs.selfImproveConfig.MaxRetries
	if maxRetries < 0 {
		maxRetries = 2
	}

	return fmt.Sprintf(`# Self-Improvement Review

Current time: %s

You are a self-improvement agent. Analyze the struggle log below and create skills or agents to address recurring patterns.

## Configuration
- Maximum creations this run: %d
- Maximum retries per creation: %d

## Struggle Log

%s

Follow your system prompt instructions to analyze, create, test, and report.
`, now, maxCreations, maxRetries, struggleLog)
}
