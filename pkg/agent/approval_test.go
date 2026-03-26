package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApprovalHook_YOLOMode(t *testing.T) {
	hook := NewApprovalHook("yolo", 300)
	decision, err := hook.ApproveTool(context.Background(), &ToolApprovalRequest{Tool: "exec", Arguments: map[string]any{"command": "rm -rf /"}})
	assert.NoError(t, err)
	assert.True(t, decision.Approved)
}

func TestApprovalHook_ApprovalMode_SafeToolAllowed(t *testing.T) {
	hook := NewApprovalHook("approval", 300)
	decision, err := hook.ApproveTool(context.Background(), &ToolApprovalRequest{Tool: "read_file", Arguments: map[string]any{"path": "/tmp/test"}})
	assert.NoError(t, err)
	assert.True(t, decision.Approved)
}

func TestApprovalHook_ApprovalMode_SafeTools(t *testing.T) {
	hook := NewApprovalHook("approval", 300)
	safeTools := []string{"read_file", "list_dir", "web_search", "web_fetch", "memory_search", "memory_recall", "spawn_status"}
	for _, tool := range safeTools {
		decision, err := hook.ApproveTool(context.Background(), &ToolApprovalRequest{Tool: tool})
		assert.NoError(t, err)
		assert.True(t, decision.Approved, "tool %s should be auto-approved", tool)
	}
}

func TestApprovalHook_ApprovalMode_DangerousToolBlocked(t *testing.T) {
	hook := NewApprovalHook("approval", 1) // 1 second timeout
	decision, err := hook.ApproveTool(context.Background(), &ToolApprovalRequest{Tool: "exec", Arguments: map[string]any{"command": "rm -rf /"}})
	assert.NoError(t, err)
	assert.False(t, decision.Approved)
	assert.Contains(t, decision.Reason, "timed out")
}

func TestApprovalHook_ApprovalMode_DangerousToolWithCallback(t *testing.T) {
	hook := NewApprovalHook("approval", 300)
	hook.SetApprovalCallback(func(toolName string, args map[string]any) bool {
		return true // simulate user approving
	})
	decision, err := hook.ApproveTool(context.Background(), &ToolApprovalRequest{Tool: "exec", Arguments: map[string]any{"command": "ls"}})
	assert.NoError(t, err)
	assert.True(t, decision.Approved)
}

func TestApprovalHook_ApprovalMode_DangerousToolRejectedByCallback(t *testing.T) {
	hook := NewApprovalHook("approval", 300)
	hook.SetApprovalCallback(func(toolName string, args map[string]any) bool {
		return false // simulate user rejecting
	})
	decision, err := hook.ApproveTool(context.Background(), &ToolApprovalRequest{Tool: "exec", Arguments: map[string]any{"command": "rm -rf /"}})
	assert.NoError(t, err)
	assert.False(t, decision.Approved)
	assert.Contains(t, decision.Reason, "denied")
}
