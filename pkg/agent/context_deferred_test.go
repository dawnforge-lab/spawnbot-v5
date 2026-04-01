package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// stubTool is a minimal Tool implementation for testing deferred tool announcements.
type stubTool struct {
	name string
	desc string
}

func (s *stubTool) Name() string                                              { return s.name }
func (s *stubTool) Description() string                                       { return s.desc }
func (s *stubTool) Parameters() map[string]any                                { return map[string]any{"type": "object"} }
func (s *stubTool) Execute(_ context.Context, _ map[string]any) *tools.ToolResult { return tools.SilentResult("ok") }

func TestBuildDeferredToolsAnnouncement(t *testing.T) {
	reg := tools.NewToolRegistry()
	reg.Register(&stubTool{name: "core_tool", desc: "a core tool"})
	reg.RegisterHidden(&stubTool{name: "hidden_a", desc: "hidden A"})
	reg.RegisterHidden(&stubTool{name: "hidden_b", desc: "hidden B"})

	announcement := BuildDeferredToolsAnnouncement(reg)

	if !strings.Contains(announcement, "<available-deferred-tools>") {
		t.Error("expected opening XML tag")
	}
	if !strings.Contains(announcement, "</available-deferred-tools>") {
		t.Error("expected closing XML tag")
	}
	if !strings.Contains(announcement, "hidden_a") {
		t.Error("expected hidden_a in announcement")
	}
	if !strings.Contains(announcement, "hidden_b") {
		t.Error("expected hidden_b in announcement")
	}
	if strings.Contains(announcement, "core_tool") {
		t.Error("core_tool should NOT appear in announcement")
	}
}

func TestBuildDeferredToolsAnnouncement_Empty(t *testing.T) {
	reg := tools.NewToolRegistry()
	reg.Register(&stubTool{name: "core_only", desc: "core"})

	announcement := BuildDeferredToolsAnnouncement(reg)
	if announcement != "" {
		t.Errorf("expected empty string, got: %q", announcement)
	}
}

func TestBuildDeferredToolsAnnouncement_ShrinkAfterDiscovery(t *testing.T) {
	reg := tools.NewToolRegistry()
	reg.Register(&stubTool{name: "core_tool", desc: "core"})
	reg.RegisterHidden(&stubTool{name: "hidden_x", desc: "X"})
	reg.RegisterHidden(&stubTool{name: "hidden_y", desc: "Y"})

	// Before discovery: both hidden tools listed
	announcement := BuildDeferredToolsAnnouncement(reg)
	if !strings.Contains(announcement, "hidden_x") || !strings.Contains(announcement, "hidden_y") {
		t.Error("expected both hidden tools before discovery")
	}

	// Discover one tool
	reg.PromoteTools([]string{"hidden_x"})

	announcement = BuildDeferredToolsAnnouncement(reg)
	if strings.Contains(announcement, "hidden_x") {
		t.Error("hidden_x should no longer appear after discovery")
	}
	if !strings.Contains(announcement, "hidden_y") {
		t.Error("hidden_y should still appear")
	}
}
