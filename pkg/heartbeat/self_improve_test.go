package heartbeat

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

func TestShouldSelfImprove_Disabled(t *testing.T) {
	dir := t.TempDir()
	hs := &HeartbeatService{
		workspace: dir,
		selfImproveConfig: config.SelfImproveConfig{
			Enabled: false,
			Hour:    3,
		},
	}

	if hs.shouldSelfImprove(3) {
		t.Error("expected false when disabled")
	}
}

func TestShouldSelfImprove_WrongHour(t *testing.T) {
	dir := t.TempDir()
	hs := &HeartbeatService{
		workspace: dir,
		selfImproveConfig: config.SelfImproveConfig{
			Enabled: true,
			Hour:    3,
		},
	}

	if hs.shouldSelfImprove(10) {
		t.Error("expected false for wrong hour")
	}
}

func TestShouldSelfImprove_AlreadyRanToday(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	os.MkdirAll(stateDir, 0o755)
	stateFile := filepath.Join(stateDir, "last_self_improve.txt")
	os.WriteFile(stateFile, []byte(time.Now().Format("2006-01-02")), 0o644)

	hs := &HeartbeatService{
		workspace: dir,
		selfImproveConfig: config.SelfImproveConfig{
			Enabled: true,
			Hour:    3,
		},
	}

	if hs.shouldSelfImprove(3) {
		t.Error("expected false when already ran today")
	}
}

func TestShouldSelfImprove_NoLogFile(t *testing.T) {
	dir := t.TempDir()
	hs := &HeartbeatService{
		workspace: dir,
		selfImproveConfig: config.SelfImproveConfig{
			Enabled: true,
			Hour:    3,
		},
	}

	if hs.shouldSelfImprove(3) {
		t.Error("expected false when no struggle log exists")
	}
}

func TestShouldSelfImprove_AllConditionsMet(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "struggles.jsonl")
	os.WriteFile(logPath, []byte(`{"type":"tool_error"}`+"\n"), 0o644)

	hs := &HeartbeatService{
		workspace: dir,
		selfImproveConfig: config.SelfImproveConfig{
			Enabled: true,
			Hour:    3,
		},
	}

	if !hs.shouldSelfImprove(3) {
		t.Error("expected true when all conditions met")
	}
}

func TestMarkSelfImproveRan(t *testing.T) {
	dir := t.TempDir()
	hs := &HeartbeatService{workspace: dir}

	hs.markSelfImproveRan()

	stateFile := filepath.Join(dir, "state", "last_self_improve.txt")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	if string(data) != today {
		t.Errorf("expected %q, got %q", today, string(data))
	}
}
