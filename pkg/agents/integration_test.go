package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndToEnd_BuiltinsLoadAndSummarize(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadBuiltins(); err != nil {
		t.Fatalf("failed to load builtins: %v", err)
	}

	list := r.List()
	if len(list) != 5 {
		t.Fatalf("expected 5 builtin agents, got %d", len(list))
	}

	names := map[string]bool{}
	for _, a := range list {
		names[a.Name] = true
	}
	for _, expected := range []string{"researcher", "coder", "reviewer", "planner", "self-improver"} {
		if !names[expected] {
			t.Fatalf("missing builtin agent: %s", expected)
		}
	}

	summary := r.Summary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	for _, name := range []string{"researcher", "coder", "reviewer", "planner"} {
		if !strings.Contains(summary, name) {
			t.Fatalf("summary missing %s: %s", name, summary)
		}
	}
}

func TestEndToEnd_WorkspaceOverridesBuiltin(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadBuiltins(); err != nil {
		t.Fatal(err)
	}

	original := r.Get("researcher")
	if original.Source != "builtin" {
		t.Fatalf("expected builtin source, got %s", original.Source)
	}

	dir := t.TempDir()
	agentDir := filepath.Join(dir, "researcher")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("---\nname: researcher\ndescription: Custom workspace researcher with extra capabilities\ntools_deny:\n  - exec\n---\n\nYou are a custom research agent with workspace-specific instructions.\n"), 0644)

	r.Reload(dir)

	overridden := r.Get("researcher")
	if overridden.Source != "workspace" {
		t.Fatalf("expected workspace source after override, got %s", overridden.Source)
	}
	if overridden.Description != "Custom workspace researcher with extra capabilities" {
		t.Fatalf("description not overridden: %s", overridden.Description)
	}

	if r.Get("coder") == nil {
		t.Fatal("coder builtin lost after reload")
	}
}

func TestEndToEnd_ToolFiltering(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadBuiltins(); err != nil {
		t.Fatal(err)
	}

	allTools := []string{
		"read_file", "write_file", "edit_file", "append_file",
		"exec", "web_search", "web_fetch", "memory_search",
		"message", "send_file", "spawn", "subagent",
		"connect_mcp", "disconnect_mcp",
	}

	researcher := r.Get("researcher")
	tools := researcher.FilterTools(allTools)
	for _, name := range tools {
		switch name {
		case "write_file", "edit_file", "append_file", "exec", "message", "send_file", "spawn", "subagent", "connect_mcp", "disconnect_mcp":
			t.Fatalf("researcher should not have tool %q", name)
		}
	}
	if len(tools) == 0 {
		t.Fatal("researcher has no tools")
	}

	coder := r.Get("coder")
	coderTools := coder.FilterTools(allTools)
	hasWriteFile := false
	for _, name := range coderTools {
		if name == "write_file" {
			hasWriteFile = true
		}
		if name == "message" || name == "send_file" {
			t.Fatalf("coder should not have tool %q", name)
		}
	}
	if !hasWriteFile {
		t.Fatal("coder should have write_file")
	}
}

func TestEndToEnd_BrokenWorkspaceAgentShowsWarning(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadBuiltins(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	brokenDir := filepath.Join(dir, "broken-agent")
	os.MkdirAll(brokenDir, 0755)
	os.WriteFile(filepath.Join(brokenDir, "AGENT.md"), []byte("no frontmatter"), 0644)

	r.Reload(dir)

	summary := r.Summary()
	if !strings.Contains(summary, "WARNING") {
		t.Fatalf("expected warning in summary for broken agent: %s", summary)
	}
	if !strings.Contains(summary, "broken-agent") {
		t.Fatalf("warning should reference the broken agent name: %s", summary)
	}
}
