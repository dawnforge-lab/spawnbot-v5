package agents

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseAgentMD_Valid(t *testing.T) {
	content := `---
name: my-agent
description: Does something useful
model: claude-opus-4
tools_allow:
  - bash
  - read
tools_deny:
  - spawn
max_iterations: 10
timeout: 2m
---

You are a helpful agent.

Be concise.
`
	def, err := ParseAgentMD(content, "workspace", "/some/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.Name != "my-agent" {
		t.Errorf("Name: got %q, want %q", def.Name, "my-agent")
	}
	if def.Description != "Does something useful" {
		t.Errorf("Description: got %q, want %q", def.Description, "Does something useful")
	}
	if def.Model != "claude-opus-4" {
		t.Errorf("Model: got %q, want %q", def.Model, "claude-opus-4")
	}
	if len(def.ToolsAllow) != 2 || def.ToolsAllow[0] != "bash" || def.ToolsAllow[1] != "read" {
		t.Errorf("ToolsAllow: got %v", def.ToolsAllow)
	}
	if len(def.ToolsDeny) != 1 || def.ToolsDeny[0] != "spawn" {
		t.Errorf("ToolsDeny: got %v", def.ToolsDeny)
	}
	if def.MaxIterations != 10 {
		t.Errorf("MaxIterations: got %d, want 10", def.MaxIterations)
	}
	if def.Timeout != 2*time.Minute {
		t.Errorf("Timeout: got %v, want 2m", def.Timeout)
	}
	if def.Source != "workspace" {
		t.Errorf("Source: got %q, want %q", def.Source, "workspace")
	}
	if def.BaseDir != "/some/dir" {
		t.Errorf("BaseDir: got %q, want %q", def.BaseDir, "/some/dir")
	}
	if def.SystemPrompt != "You are a helpful agent.\n\nBe concise." {
		t.Errorf("SystemPrompt: got %q", def.SystemPrompt)
	}
}

func TestParseAgentMD_MissingFrontmatter(t *testing.T) {
	content := `name: my-agent
description: No frontmatter markers
`
	_, err := ParseAgentMD(content, "workspace", "/some/dir")
	if err == nil {
		t.Fatal("expected error for missing frontmatter, got nil")
	}
}

func TestParseAgentMD_MinimalValid(t *testing.T) {
	content := `---
name: minimal-agent
description: Just the basics
---

Do minimal things.
`
	def, err := ParseAgentMD(content, "builtin", "/base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.Name != "minimal-agent" {
		t.Errorf("Name: got %q", def.Name)
	}
	if def.Description != "Just the basics" {
		t.Errorf("Description: got %q", def.Description)
	}
	// Verify defaults applied
	if def.MaxIterations != defaultMaxIterations {
		t.Errorf("MaxIterations default: got %d, want %d", def.MaxIterations, defaultMaxIterations)
	}
	if def.Timeout != defaultTimeout {
		t.Errorf("Timeout default: got %v, want %v", def.Timeout, defaultTimeout)
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()

	// Agent 1: valid
	agent1Dir := filepath.Join(dir, "agent-one")
	if err := os.MkdirAll(agent1Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agent1Dir, "AGENT.md"), []byte(`---
name: agent-one
description: First valid agent
---

Do things.
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Agent 2: valid
	agent2Dir := filepath.Join(dir, "agent-two")
	if err := os.MkdirAll(agent2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agent2Dir, "AGENT.md"), []byte(`---
name: agent-two
description: Second valid agent
---

Do other things.
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Agent 3: broken (missing frontmatter)
	agent3Dir := filepath.Join(dir, "agent-broken")
	if err := os.MkdirAll(agent3Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agent3Dir, "AGENT.md"), []byte(`not valid frontmatter at all`), 0644); err != nil {
		t.Fatal(err)
	}

	agents, warnings := LoadFromDir(dir)

	if len(agents) != 2 {
		t.Errorf("agents count: got %d, want 2", len(agents))
	}
	if len(warnings) != 1 {
		t.Errorf("warnings count: got %d, want 1 (got: %v)", len(warnings), warnings)
	}
}

func TestLoadFromDir_Empty(t *testing.T) {
	dir := t.TempDir()

	agents, warnings := LoadFromDir(dir)

	if len(agents) != 0 {
		t.Errorf("agents: got %d, want 0", len(agents))
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: got %d, want 0", len(warnings))
	}
}

func TestLoadFromDir_NonExistent(t *testing.T) {
	agents, warnings := LoadFromDir("/nonexistent/path/that/does/not/exist")

	if len(agents) != 0 {
		t.Errorf("agents: got %d, want 0", len(agents))
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: got %d, want 0", len(warnings))
	}
}
