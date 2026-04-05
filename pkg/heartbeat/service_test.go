package heartbeat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteHeartbeat_Async(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.stopChan = make(chan struct{}) // Enable for testing

	asyncCalled := false
	asyncResult := &tools.ToolResult{
		ForLLM:  "Background task started",
		ForUser: "Task started in background",
		Silent:  false,
		IsError: false,
		Async:   true,
	}

	hs.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		asyncCalled = true
		if prompt == "" {
			t.Error("Expected non-empty prompt")
		}
		return asyncResult
	})

	// Create HEARTBEAT.md
	os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Test task"), 0o644)

	// Execute heartbeat directly (internal method for testing)
	hs.executeHeartbeat()
	hs.waitDone()

	if !asyncCalled {
		t.Error("Expected handler to be called")
	}
}

func TestExecuteHeartbeat_ResultLogging(t *testing.T) {
	tests := []struct {
		name    string
		result  *tools.ToolResult
		wantLog string
	}{
		{
			name: "error result",
			result: &tools.ToolResult{
				ForLLM:  "Heartbeat failed: connection error",
				ForUser: "",
				Silent:  false,
				IsError: true,
				Async:   false,
			},
			wantLog: "error message",
		},
		{
			name: "silent result",
			result: &tools.ToolResult{
				ForLLM:  "Heartbeat completed successfully",
				ForUser: "",
				Silent:  true,
				IsError: false,
				Async:   false,
			},
			wantLog: "completion message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			hs := NewHeartbeatService(tmpDir, 30, true)
			hs.stopChan = make(chan struct{}) // Enable for testing

			hs.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
				return tt.result
			})

			os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Test task"), 0o644)
			hs.executeHeartbeat()
			hs.waitDone()

			logFile := filepath.Join(tmpDir, "heartbeat.log")
			data, err := os.ReadFile(logFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}
			if string(data) == "" {
				t.Errorf("Expected log file to contain %s", tt.wantLog)
			}
		})
	}
}

func TestHeartbeatService_StartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 1, true)

	err = hs.Start()
	if err != nil {
		t.Fatalf("Failed to start heartbeat service: %v", err)
	}

	hs.Stop()

	time.Sleep(100 * time.Millisecond)
}

func TestHeartbeatService_Disabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 1, false)

	if hs.enabled != false {
		t.Error("Expected service to be disabled")
	}

	err = hs.Start()
	_ = err // Disabled service returns nil
}

func TestExecuteHeartbeat_NilResult(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.stopChan = make(chan struct{}) // Enable for testing

	hs.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		return nil
	})

	// Create HEARTBEAT.md
	os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Test task"), 0o644)

	// Should not panic with nil result
	hs.executeHeartbeat()
	hs.waitDone()
}

// TestLogPath verifies heartbeat log is written to workspace directory
func TestLogPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 30, true)

	// Write a log entry
	hs.logf("INFO", "Test log entry")

	// Verify log file exists at workspace root
	expectedLogPath := filepath.Join(tmpDir, "heartbeat.log")
	if _, err := os.Stat(expectedLogPath); os.IsNotExist(err) {
		t.Errorf("Expected log file at %s, but it doesn't exist", expectedLogPath)
	}
}

// TestHeartbeatFilePath verifies HEARTBEAT.md is at workspace root
func TestHeartbeatFilePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 30, true)

	// Trigger default template creation
	hs.buildPrompt()

	// Verify HEARTBEAT.md exists at workspace root
	expectedPath := filepath.Join(tmpDir, "HEARTBEAT.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected HEARTBEAT.md at %s, but it doesn't exist", expectedPath)
	}
}

func TestBuildPrompt_DefaultTemplateStaysIdle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.createDefaultHeartbeatTemplate()

	if prompt := hs.buildPrompt(); prompt != "" {
		t.Fatalf("buildPrompt() = %q, want empty prompt for untouched default template", prompt)
	}
}

func TestBuildPrompt_UserTasksAfterMarkerProducePrompt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "heartbeat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.createDefaultHeartbeatTemplate()

	path := filepath.Join(tmpDir, "HEARTBEAT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read HEARTBEAT.md: %v", err)
	}
	updated := string(data) + "\n- Check unread Feishu messages\n"
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("Failed to update HEARTBEAT.md: %v", err)
	}

	prompt := hs.buildPrompt()
	if prompt == "" {
		t.Fatal("buildPrompt() = empty, want non-empty prompt when user tasks are present")
	}
	if !strings.Contains(prompt, "Check unread Feishu messages") {
		t.Fatalf("prompt = %q, want user task content", prompt)
	}
}

func TestExecuteHeartbeat_DeduplicatesIdenticalAlerts(t *testing.T) {
	tmpDir := t.TempDir()
	hs := NewHeartbeatService(tmpDir, 30, true)
	hs.stopChan = make(chan struct{})

	hs.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		return &tools.ToolResult{ForUser: "alert: disk full", ForLLM: "alert: disk full"}
	})

	var sentCount int
	hs.Events().OnEvent(func(ev HeartbeatEvent) {
		if ev.Status == EventStatusSent {
			sentCount++
		}
	})

	os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check disk"), 0o644)

	hs.executeHeartbeat()
	hs.waitDone()
	hs.executeHeartbeat() // second call should be deduped
	hs.waitDone()

	assert.Equal(t, 1, sentCount, "duplicate alert should be suppressed")
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
	hs.waitDone()

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
	hs.waitDone()

	require.NotNil(t, lastEvent)
	assert.Equal(t, EventStatusSent, lastEvent.Status)
	assert.Greater(t, lastEvent.DurationMs, int64(-1))
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
