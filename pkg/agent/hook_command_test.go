package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0755)
	if err != nil {
		t.Fatalf("write test script: %v", err)
	}
	return path
}

func TestCommandHook_Observer_ReceivesJSON(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "output.json")

	scriptPath := writeTestScript(t, dir, "hook.sh", "#!/bin/bash\ncat > "+outputPath+"\n")

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-observer",
		ScriptPath: scriptPath,
		Mode:       "observe",
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	evt := Event{
		Kind: EventKindToolExecEnd,
		Time: time.Now(),
		Meta: EventMeta{SessionKey: "sess-1", TurnID: "turn-1"},
		Payload: ToolExecEndPayload{
			Tool:    "exec",
			IsError: true,
		},
	}

	err := hook.OnEvent(context.Background(), evt)
	if err != nil {
		t.Fatalf("OnEvent: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected JSON output, got empty")
	}
}

func TestCommandHook_Observer_EventFilter(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "output.json")

	scriptPath := writeTestScript(t, dir, "hook.sh", "#!/bin/bash\ncat > "+outputPath+"\n")

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-filter",
		ScriptPath: scriptPath,
		Mode:       "observe",
		Events:     map[string]struct{}{"tool_exec_end": {}},
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	evt := Event{
		Kind:    EventKindTurnStart,
		Time:    time.Now(),
		Meta:    EventMeta{SessionKey: "sess-1"},
		Payload: TurnStartPayload{UserMessage: "hello"},
	}
	hook.OnEvent(context.Background(), evt)

	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Error("expected no output for filtered event")
	}
}

func TestCommandHook_Observer_ToolFilter(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "output.json")

	scriptPath := writeTestScript(t, dir, "hook.sh", "#!/bin/bash\ncat > "+outputPath+"\n")

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-tool-filter",
		ScriptPath: scriptPath,
		Mode:       "observe",
		Tools:      map[string]struct{}{"write_file": {}},
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	evt := Event{
		Kind: EventKindToolExecEnd,
		Time: time.Now(),
		Meta: EventMeta{SessionKey: "sess-1"},
		Payload: ToolExecEndPayload{
			Tool: "exec",
		},
	}
	hook.OnEvent(context.Background(), evt)

	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Error("expected no output for non-matching tool")
	}
}

func TestCommandHook_Observer_Timeout(t *testing.T) {
	dir := t.TempDir()

	scriptPath := writeTestScript(t, dir, "hook.sh", "#!/bin/bash\nsleep 10\n")

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-timeout",
		ScriptPath: scriptPath,
		Mode:       "observe",
		Timeout:    200 * time.Millisecond,
		Shell:      "bash",
	})

	evt := Event{
		Kind: EventKindToolExecEnd,
		Time: time.Now(),
		Meta: EventMeta{SessionKey: "sess-1"},
		Payload: ToolExecEndPayload{
			Tool: "exec",
		},
	}

	err := hook.OnEvent(context.Background(), evt)
	if err != nil {
		t.Fatalf("observer should not return error on timeout, got: %v", err)
	}
}

func TestCommandHook_Interceptor_Continue(t *testing.T) {
	dir := t.TempDir()

	scriptPath := writeTestScript(t, dir, "hook.sh", "#!/bin/bash\ncat > /dev/null\nexit 0\n")

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-continue",
		ScriptPath: scriptPath,
		Mode:       "intercept",
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	call := &ToolCallHookRequest{
		Tool:      "write_file",
		Arguments: map[string]any{"path": "/tmp/test"},
	}

	result, decision, err := hook.BeforeTool(context.Background(), call)
	if err != nil {
		t.Fatalf("BeforeTool: %v", err)
	}
	if decision.Action != HookActionContinue {
		t.Errorf("expected continue, got %v", decision.Action)
	}
	if result.Tool != "write_file" {
		t.Errorf("expected tool write_file, got %s", result.Tool)
	}
}

func TestCommandHook_Interceptor_Deny(t *testing.T) {
	dir := t.TempDir()

	scriptPath := writeTestScript(t, dir, "hook.sh", "#!/bin/bash\ncat > /dev/null\necho '{\"action\":\"deny_tool\",\"reason\":\"not allowed\"}'\nexit 1\n")

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-deny",
		ScriptPath: scriptPath,
		Mode:       "intercept",
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	call := &ToolCallHookRequest{
		Tool:      "write_file",
		Arguments: map[string]any{"path": "/etc/passwd"},
	}

	_, decision, err := hook.BeforeTool(context.Background(), call)
	if err != nil {
		t.Fatalf("BeforeTool: %v", err)
	}
	if decision.Action != HookActionDenyTool {
		t.Errorf("expected deny_tool, got %v", decision.Action)
	}
	if decision.Reason != "not allowed" {
		t.Errorf("expected reason 'not allowed', got %q", decision.Reason)
	}
}

func TestCommandHook_Interceptor_Modify(t *testing.T) {
	dir := t.TempDir()

	scriptPath := writeTestScript(t, dir, "hook.sh", "#!/bin/bash\ncat > /dev/null\necho '{\"action\":\"modify\",\"arguments\":{\"path\":\"/tmp/safe\"}}'\nexit 0\n")

	hook := NewCommandHook(CommandHookOptions{
		Name:       "test-modify",
		ScriptPath: scriptPath,
		Mode:       "intercept",
		Timeout:    5 * time.Second,
		Shell:      "bash",
	})

	call := &ToolCallHookRequest{
		Tool:      "write_file",
		Arguments: map[string]any{"path": "/etc/passwd"},
	}

	result, decision, err := hook.BeforeTool(context.Background(), call)
	if err != nil {
		t.Fatalf("BeforeTool: %v", err)
	}
	if decision.Action != HookActionModify {
		t.Errorf("expected modify, got %v", decision.Action)
	}
	if result.Arguments["path"] != "/tmp/safe" {
		t.Errorf("expected modified path, got %v", result.Arguments["path"])
	}
}
