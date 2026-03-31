package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTestDef(name, description, source string) *AgentDefinition {
	return &AgentDefinition{
		Name:          name,
		Description:   description,
		Source:        source,
		MaxIterations: defaultMaxIterations,
		Timeout:       defaultTimeout,
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	def := makeTestDef("my-agent", "Does something useful", "workspace")
	r.Register(def)

	got := r.Get("my-agent")
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.Name != "my-agent" {
		t.Errorf("Name: got %q, want %q", got.Name, "my-agent")
	}
	if got.Description != "Does something useful" {
		t.Errorf("Description: got %q, want %q", got.Description, "Does something useful")
	}
	if got.Source != "workspace" {
		t.Errorf("Source: got %q, want %q", got.Source, "workspace")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	got := r.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(makeTestDef("zebra-agent", "Last alphabetically", "workspace"))
	r.Register(makeTestDef("alpha-agent", "First alphabetically", "workspace"))

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("list length: got %d, want 2", len(list))
	}
	if list[0].Name != "alpha-agent" {
		t.Errorf("list[0].Name: got %q, want %q", list[0].Name, "alpha-agent")
	}
	if list[1].Name != "zebra-agent" {
		t.Errorf("list[1].Name: got %q, want %q", list[1].Name, "zebra-agent")
	}
}

func TestRegistry_WorkspaceOverridesBuiltin(t *testing.T) {
	r := NewRegistry()
	builtin := makeTestDef("shared-agent", "Builtin version", "builtin")
	workspace := makeTestDef("shared-agent", "Workspace version", "workspace")

	r.Register(builtin)
	r.Register(workspace)

	got := r.Get("shared-agent")
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.Source != "workspace" {
		t.Errorf("Source: got %q, want workspace", got.Source)
	}
	if got.Description != "Workspace version" {
		t.Errorf("Description: got %q, want %q", got.Description, "Workspace version")
	}
}

func TestRegistry_Summary(t *testing.T) {
	r := NewRegistry()
	r.Register(makeTestDef("beta-agent", "Beta description", "workspace"))
	r.Register(makeTestDef("alpha-agent", "Alpha description", "workspace"))

	summary := r.Summary()

	if !strings.HasPrefix(summary, "Available agents:") {
		t.Errorf("summary should start with 'Available agents:', got: %q", summary)
	}
	lines := strings.Split(summary, "\n")
	// lines[0] = "Available agents:", lines[1] = alpha, lines[2] = beta
	if len(lines) != 3 {
		t.Fatalf("summary line count: got %d, want 3 (got: %q)", len(lines), summary)
	}
	if lines[1] != "- alpha-agent: Alpha description" {
		t.Errorf("lines[1]: got %q, want %q", lines[1], "- alpha-agent: Alpha description")
	}
	if lines[2] != "- beta-agent: Beta description" {
		t.Errorf("lines[2]: got %q, want %q", lines[2], "- beta-agent: Beta description")
	}
}

func TestRegistry_SummaryWithWarnings(t *testing.T) {
	r := NewRegistry()
	r.Register(makeTestDef("solo-agent", "The only agent", "workspace"))
	r.AddWarning("some/broken/AGENT.md failed to load: missing frontmatter")

	summary := r.Summary()

	if !strings.Contains(summary, "WARNING: some/broken/AGENT.md failed to load: missing frontmatter") {
		t.Errorf("summary missing warning line, got: %q", summary)
	}
}

func TestRegistry_SummaryEmpty(t *testing.T) {
	r := NewRegistry()
	summary := r.Summary()
	if summary != "" {
		t.Errorf("empty registry summary should be empty string, got: %q", summary)
	}
}

func TestRegistry_Reload(t *testing.T) {
	dir := t.TempDir()

	agentDir := filepath.Join(dir, "reload-agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(`---
name: reload-agent
description: Loaded via reload
---

Do reloaded things.
`), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	// Register a builtin that should survive reload
	r.Register(makeTestDef("builtin-agent", "Builtin stays", "builtin"))

	r.Reload(dir)

	reloaded := r.Get("reload-agent")
	if reloaded == nil {
		t.Fatal("expected reload-agent after Reload, got nil")
	}
	if reloaded.Description != "Loaded via reload" {
		t.Errorf("Description: got %q, want %q", reloaded.Description, "Loaded via reload")
	}

	// Builtin should still be present
	builtin := r.Get("builtin-agent")
	if builtin == nil {
		t.Fatal("expected builtin-agent to survive Reload, got nil")
	}
}

func TestRegistry_IsBuiltin(t *testing.T) {
	r := NewRegistry()
	r.Register(makeTestDef("builtin-agent", "A builtin", "builtin"))
	r.Register(makeTestDef("workspace-agent", "A workspace agent", "workspace"))

	if !r.IsBuiltin("builtin-agent") {
		t.Error("expected builtin-agent to be builtin")
	}
	if r.IsBuiltin("workspace-agent") {
		t.Error("expected workspace-agent to not be builtin")
	}
	if r.IsBuiltin("nonexistent") {
		t.Error("expected nonexistent to not be builtin")
	}
}
