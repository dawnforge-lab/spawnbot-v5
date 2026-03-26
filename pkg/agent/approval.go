package agent

import (
	"context"
	"fmt"
	"time"
)

// dangerousTools lists tools that require approval in Approval mode.
var dangerousTools = map[string]bool{
	"exec":        true,
	"write_file":  true,
	"edit_file":   true,
	"append_file": true,
	"spawn":       true,
}

// ApprovalCallback is a function called when a dangerous tool needs user confirmation.
// Returns true to approve, false to deny.
type ApprovalCallback func(toolName string, args map[string]any) bool

// ApprovalHook implements ToolApprover with two modes:
//   - "yolo": all tools are auto-approved
//   - "approval": dangerous tools require confirmation via callback; safe tools are auto-approved
type ApprovalHook struct {
	mode             string // "yolo" or "approval"
	timeoutSeconds   int
	approvalCallback ApprovalCallback
}

// NewApprovalHook creates a new ApprovalHook with the given mode and timeout.
// mode should be "yolo" or "approval". timeoutSeconds is used when waiting
// for a callback in approval mode (falls back to deny on timeout).
func NewApprovalHook(mode string, timeoutSeconds int) *ApprovalHook {
	return &ApprovalHook{
		mode:           mode,
		timeoutSeconds: timeoutSeconds,
	}
}

// SetApprovalCallback registers the callback that will be invoked when a
// dangerous tool is about to run in approval mode.
func (h *ApprovalHook) SetApprovalCallback(cb ApprovalCallback) {
	h.approvalCallback = cb
}

// ApproveTool implements ToolApprover.
func (h *ApprovalHook) ApproveTool(_ context.Context, req *ToolApprovalRequest) (ApprovalDecision, error) {
	if req == nil {
		return ApprovalDecision{Approved: true}, nil
	}

	// YOLO mode: approve everything unconditionally.
	if h.mode != "approval" {
		return ApprovalDecision{Approved: true}, nil
	}

	// Approval mode: safe tools are always approved.
	if !dangerousTools[req.Tool] {
		return ApprovalDecision{Approved: true}, nil
	}

	// Dangerous tool: invoke callback if set.
	if h.approvalCallback != nil {
		approved := h.approvalCallback(req.Tool, req.Arguments)
		if approved {
			return ApprovalDecision{Approved: true}, nil
		}
		return ApprovalDecision{
			Approved: false,
			Reason:   fmt.Sprintf("denied by user for tool %q", req.Tool),
		}, nil
	}

	// No callback set — wait for timeout then deny.
	timeout := time.Duration(h.timeoutSeconds) * time.Second
	select {
	case <-time.After(timeout):
		return ApprovalDecision{
			Approved: false,
			Reason:   fmt.Sprintf("denied — timed out after %s waiting for approval of tool %q", timeout, req.Tool),
		}, nil
	}
}
