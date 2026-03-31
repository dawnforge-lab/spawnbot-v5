package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandHook_Integration_ObserverReceivesEvents(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "events.jsonl")

	scriptPath := writeTestScript(t, dir, "observer.sh", "#!/bin/bash\ncat >> "+outputPath+"\necho \"\" >> "+outputPath+"\n")

	eventBus := NewEventBus()
	defer eventBus.Close()
	hm := NewHookManager(eventBus)
	defer hm.Close()

	// Extend observer timeout so the script has enough time to run
	hm.ConfigureTimeouts(5*time.Second, 0, 0)

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-observer",
		ScriptPath: scriptPath,
		Mode:       "observe",
		Events:     map[string]struct{}{"tool_exec_end": {}},
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	err := hm.Mount(HookRegistration{
		Name:   "test-observer",
		Source: HookSourceInProcess,
		Hook:   hook,
	})
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}

	eventBus.Emit(Event{
		Kind: EventKindToolExecEnd,
		Time: time.Now(),
		Meta: EventMeta{SessionKey: "test", TurnID: "t1"},
		Payload: ToolExecEndPayload{
			Tool:    "exec",
			IsError: true,
		},
	})

	// Give the observer time to process
	time.Sleep(500 * time.Millisecond)

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected event data in output file")
	}
}

func TestCommandHook_Integration_InterceptorBlocks(t *testing.T) {
	dir := t.TempDir()

	scriptPath := writeTestScript(t, dir, "blocker.sh", "#!/bin/bash\ncat > /dev/null\necho '{\"action\":\"deny_tool\",\"reason\":\"blocked by test\"}'\nexit 1\n")

	eventBus := NewEventBus()
	defer eventBus.Close()
	hm := NewHookManager(eventBus)
	defer hm.Close()

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-blocker",
		ScriptPath: scriptPath,
		Mode:       "intercept",
		Tools:      map[string]struct{}{"dangerous_tool": {}},
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	err := hm.Mount(HookRegistration{
		Name:   "test-blocker",
		Source: HookSourceInProcess,
		Hook:   hook,
	})
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}

	call := &ToolCallHookRequest{
		Tool:      "dangerous_tool",
		Arguments: map[string]any{"cmd": "rm -rf /"},
	}

	_, decision := hm.BeforeTool(context.Background(), call)
	if decision.Action != HookActionDenyTool {
		t.Errorf("expected deny_tool, got %v", decision.Action)
	}
	if decision.Reason != "blocked by test" {
		t.Errorf("expected reason 'blocked by test', got %q", decision.Reason)
	}
}

func TestCommandHook_Integration_InterceptorModifies(t *testing.T) {
	dir := t.TempDir()

	scriptPath := writeTestScript(t, dir, "modifier.sh", "#!/bin/bash\ncat > /dev/null\necho '{\"action\":\"modify\",\"arguments\":{\"path\":\"/tmp/safe\"}}'\nexit 0\n")

	eventBus := NewEventBus()
	defer eventBus.Close()
	hm := NewHookManager(eventBus)
	defer hm.Close()

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-modifier",
		ScriptPath: scriptPath,
		Mode:       "intercept",
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	err := hm.Mount(HookRegistration{
		Name:   "test-modifier",
		Source: HookSourceInProcess,
		Hook:   hook,
	})
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}

	call := &ToolCallHookRequest{
		Tool:      "write_file",
		Arguments: map[string]any{"path": "/etc/passwd"},
	}

	result, decision := hm.BeforeTool(context.Background(), call)
	// HookManager.BeforeTool returns HookActionContinue after applying all modifiers;
	// the modification is reflected in the returned result's arguments, not the decision action.
	if decision.Action != HookActionContinue {
		t.Errorf("expected continue from manager (modifications are reflected in result), got %v", decision.Action)
	}
	if result.Arguments["path"] != "/tmp/safe" {
		t.Errorf("expected modified path /tmp/safe, got %v", result.Arguments["path"])
	}
}
