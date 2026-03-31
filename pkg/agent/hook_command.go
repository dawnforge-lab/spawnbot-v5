package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

// CommandHookOptions configures a one-shot shell script hook.
type CommandHookOptions struct {
	Name       string
	ScriptPath string
	Mode       string // "observe" or "intercept"
	Events     map[string]struct{}
	Tools      map[string]struct{}
	Timeout    time.Duration
	Shell      string
}

// CommandHook spawns a shell script per event.
// In observe mode, it implements EventObserver (fire-and-forget).
// In intercept mode, it implements ToolInterceptor (can block/modify).
type CommandHook struct {
	name       string
	scriptPath string
	mode       string
	events     map[string]struct{}
	tools      map[string]struct{}
	timeout    time.Duration
	shell      string
}

// commandHookInput is the JSON sent to the script on stdin.
type commandHookInput struct {
	Event     string         `json:"event"`
	HookName  string         `json:"hook_name"`
	Tool      string         `json:"tool,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	TurnID    string         `json:"turn_id,omitempty"`
	Session   string         `json:"session,omitempty"`
	Timestamp string         `json:"timestamp"`
}

// commandHookOutput is the JSON read from the script's stdout.
type commandHookOutput struct {
	Action    HookAction     `json:"action,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

// NewCommandHook creates a new one-shot shell script hook.
func NewCommandHook(opts CommandHookOptions) *CommandHook {
	shell := opts.Shell
	if shell == "" {
		shell = "bash"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		if opts.Mode == "intercept" {
			timeout = 10 * time.Second
		} else {
			timeout = 5 * time.Second
		}
	}
	return &CommandHook{
		name:       opts.Name,
		scriptPath: opts.ScriptPath,
		mode:       opts.Mode,
		events:     opts.Events,
		tools:      opts.Tools,
		timeout:    timeout,
		shell:      shell,
	}
}

// OnEvent implements EventObserver for observe-mode hooks.
func (ch *CommandHook) OnEvent(ctx context.Context, evt Event) error {
	if ch.mode != "observe" {
		return nil
	}
	if !ch.matchesEvent(evt.Kind.String()) {
		return nil
	}
	tool, args := extractToolInfo(evt)
	if !ch.matchesTool(tool) {
		return nil
	}

	input := commandHookInput{
		Event:     evt.Kind.String(),
		HookName:  ch.name,
		Tool:      tool,
		Arguments: args,
		TurnID:    evt.Meta.TurnID,
		Session:   evt.Meta.SessionKey,
		Timestamp: evt.Time.Format(time.RFC3339),
	}

	_, _, err := ch.run(ctx, input)
	if err != nil {
		logger.WarnCF("hooks", "Command hook observer failed", map[string]any{
			"hook":  ch.name,
			"error": err.Error(),
		})
		// Observer: don't propagate errors
		return nil
	}
	return nil
}

// BeforeTool implements ToolInterceptor for intercept-mode hooks.
func (ch *CommandHook) BeforeTool(ctx context.Context, call *ToolCallHookRequest) (*ToolCallHookRequest, HookDecision, error) {
	if ch.mode != "intercept" {
		return call, HookDecision{Action: HookActionContinue}, nil
	}
	if !ch.matchesEvent("tool_exec_start") {
		return call, HookDecision{Action: HookActionContinue}, nil
	}
	if !ch.matchesTool(call.Tool) {
		return call, HookDecision{Action: HookActionContinue}, nil
	}

	input := commandHookInput{
		Event:     "tool_exec_start",
		HookName:  ch.name,
		Tool:      call.Tool,
		Arguments: call.Arguments,
		TurnID:    call.Meta.TurnID,
		Session:   call.Meta.SessionKey,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	exitCode, output, err := ch.run(ctx, input)
	if err != nil {
		return nil, HookDecision{}, fmt.Errorf("command hook %q: %w", ch.name, err)
	}

	return ch.interpretResult(call, exitCode, output)
}

// AfterTool implements ToolInterceptor for intercept-mode hooks.
func (ch *CommandHook) AfterTool(ctx context.Context, result *ToolResultHookResponse) (*ToolResultHookResponse, HookDecision, error) {
	if ch.mode != "intercept" {
		return result, HookDecision{Action: HookActionContinue}, nil
	}
	if !ch.matchesEvent("tool_exec_end") {
		return result, HookDecision{Action: HookActionContinue}, nil
	}
	if !ch.matchesTool(result.Tool) {
		return result, HookDecision{Action: HookActionContinue}, nil
	}

	input := commandHookInput{
		Event:     "tool_exec_end",
		HookName:  ch.name,
		Tool:      result.Tool,
		Arguments: result.Arguments,
		TurnID:    result.Meta.TurnID,
		Session:   result.Meta.SessionKey,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	exitCode, output, err := ch.run(ctx, input)
	if err != nil {
		return nil, HookDecision{}, fmt.Errorf("command hook %q: %w", ch.name, err)
	}

	decision := HookDecision{Action: HookActionContinue}
	if exitCode == 1 {
		decision.Action = HookActionAbortTurn
		if output != nil && output.Reason != "" {
			decision.Reason = output.Reason
		}
	} else if output != nil && output.Action != "" {
		decision.Action = output.Action
		decision.Reason = output.Reason
	}

	return result, decision, nil
}

func (ch *CommandHook) run(ctx context.Context, input commandHookInput) (int, *commandHookOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, ch.timeout)
	defer cancel()

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return -1, nil, fmt.Errorf("marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, ch.shell, ch.scriptPath)
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			return -1, nil, fmt.Errorf("timeout after %v", ch.timeout)
		} else {
			return -1, nil, err
		}
	}

	if stderr.Len() > 0 {
		logger.WarnCF("hooks", "Command hook stderr", map[string]any{
			"hook":   ch.name,
			"stderr": stderr.String(),
		})
	}

	var output *commandHookOutput
	if stdout.Len() > 0 {
		output = &commandHookOutput{}
		if jsonErr := json.Unmarshal(stdout.Bytes(), output); jsonErr != nil {
			output = nil
		}
	}

	return exitCode, output, nil
}

func (ch *CommandHook) interpretResult(call *ToolCallHookRequest, exitCode int, output *commandHookOutput) (*ToolCallHookRequest, HookDecision, error) {
	if exitCode == 1 {
		reason := "blocked by hook"
		action := HookActionDenyTool
		if output != nil {
			if output.Reason != "" {
				reason = output.Reason
			}
			if output.Action != "" {
				action = output.Action
			}
		}
		return call, HookDecision{Action: action, Reason: reason}, nil
	}

	if exitCode >= 2 {
		return nil, HookDecision{}, fmt.Errorf("command hook %q exited with code %d", ch.name, exitCode)
	}

	// Exit 0: continue or modify
	if output != nil && output.Action == HookActionModify && output.Arguments != nil {
		modified := call.Clone()
		modified.Arguments = output.Arguments
		return modified, HookDecision{Action: HookActionModify, Reason: output.Reason}, nil
	}

	return call, HookDecision{Action: HookActionContinue}, nil
}

func (ch *CommandHook) matchesEvent(eventKind string) bool {
	if len(ch.events) == 0 {
		return true
	}
	_, ok := ch.events[eventKind]
	return ok
}

func (ch *CommandHook) matchesTool(toolName string) bool {
	if len(ch.tools) == 0 {
		return true
	}
	if toolName == "" {
		return true
	}
	_, ok := ch.tools[toolName]
	return ok
}

// extractToolInfo extracts tool name and arguments from event payloads.
func extractToolInfo(evt Event) (string, map[string]any) {
	switch p := evt.Payload.(type) {
	case ToolExecStartPayload:
		return p.Tool, p.Arguments
	case ToolExecEndPayload:
		return p.Tool, nil
	default:
		return "", nil
	}
}
