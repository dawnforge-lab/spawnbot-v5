# Markdown Agent Definitions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add markdown-based agent definitions so Spawnbot can spawn specialized subagents (researcher, coder, reviewer, planner) and create new agent types autonomously.

**Architecture:** New `pkg/agents/` package with AgentDefinition struct, Registry (thread-safe map), and AGENT.md loader. Integrates with existing SubTurnSpawner — the `spawn`/`subagent` tools gain an `agent_type` parameter that resolves from the registry. Agent summaries injected into system prompt alongside skills. A `create_agent` tool lets the main agent create new agent types at runtime.

**Tech Stack:** Go, YAML frontmatter parsing (gopkg.in/yaml.v3 — already a dependency), `embed` for builtins

---

## File Structure

```
pkg/agents/                          # NEW PACKAGE
  definition.go                      # AgentDefinition struct + validation
  definition_test.go                 # Tests for validation logic
  registry.go                        # Registry — Get, List, Summary, Register, Reload
  registry_test.go                   # Tests for registry operations
  loader.go                          # AGENT.md frontmatter parsing + directory scanning
  loader_test.go                     # Tests for parsing and loading
  builtin.go                         # go:embed for 4 builtin agents
  builtin/                           # Embedded AGENT.md files
    researcher/AGENT.md
    coder/AGENT.md
    reviewer/AGENT.md
    planner/AGENT.md

pkg/tools/
  create_agent.go                    # NEW: create_agent tool
  create_agent_test.go               # Tests for create_agent tool
  subagent.go                        # MODIFY: add agent_type parameter

pkg/agent/
  context.go                         # MODIFY: inject agent summary into system prompt
  loop.go                            # MODIFY: resolve agent_type in SubTurn spawner
  instance.go                        # MODIFY: store agents.Registry reference
```

---

### Task 1: AgentDefinition Struct + Validation

**Files:**
- Create: `pkg/agents/definition.go`
- Create: `pkg/agents/definition_test.go`

- [ ] **Step 1: Write the failing test for AgentDefinition validation**

Create `pkg/agents/definition_test.go`:

```go
package agents

import (
	"testing"
	"time"
)

func TestValidateDefinition_Valid(t *testing.T) {
	def := &AgentDefinition{
		Name:          "researcher",
		Description:   "Gathers information",
		SystemPrompt:  "You are a research agent.",
		ToolsDeny:     []string{"write_file", "exec"},
		MaxIterations: 20,
		Timeout:       5 * time.Minute,
		Source:        "builtin",
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
}

func TestValidateDefinition_MissingName(t *testing.T) {
	def := &AgentDefinition{
		Description:  "No name",
		SystemPrompt: "prompt",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestValidateDefinition_InvalidNameChars(t *testing.T) {
	def := &AgentDefinition{
		Name:         "bad name!",
		Description:  "invalid chars",
		SystemPrompt: "prompt",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for invalid name characters")
	}
}

func TestValidateDefinition_NameTooLong(t *testing.T) {
	longName := ""
	for i := 0; i < 65; i++ {
		longName += "a"
	}
	def := &AgentDefinition{
		Name:         longName,
		Description:  "too long",
		SystemPrompt: "prompt",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for name too long")
	}
}

func TestValidateDefinition_MissingDescription(t *testing.T) {
	def := &AgentDefinition{
		Name:         "test",
		SystemPrompt: "prompt",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestValidateDefinition_Defaults(t *testing.T) {
	def := &AgentDefinition{
		Name:         "test",
		Description:  "test agent",
		SystemPrompt: "prompt",
	}
	def.ApplyDefaults()
	if def.MaxIterations != 20 {
		t.Fatalf("expected MaxIterations=20, got %d", def.MaxIterations)
	}
	if def.Timeout != 5*time.Minute {
		t.Fatalf("expected Timeout=5m, got %v", def.Timeout)
	}
}

func TestFilterTools_DenyOnly(t *testing.T) {
	all := []string{"read_file", "write_file", "exec", "web_search", "spawn", "subagent"}
	def := &AgentDefinition{
		Name:        "test",
		Description: "test",
		ToolsDeny:   []string{"write_file", "exec"},
	}
	got := def.FilterTools(all)
	expected := map[string]bool{"read_file": true, "web_search": true}
	if len(got) != len(expected) {
		t.Fatalf("expected %d tools, got %d: %v", len(expected), len(got), got)
	}
	for _, name := range got {
		if !expected[name] {
			t.Fatalf("unexpected tool: %s", name)
		}
	}
}

func TestFilterTools_AllowOnly(t *testing.T) {
	all := []string{"read_file", "write_file", "exec", "web_search"}
	def := &AgentDefinition{
		Name:        "test",
		Description: "test",
		ToolsAllow:  []string{"read_file", "web_search"},
	}
	got := def.FilterTools(all)
	if len(got) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(got), got)
	}
}

func TestFilterTools_AllowAndDeny(t *testing.T) {
	all := []string{"read_file", "write_file", "exec", "web_search"}
	def := &AgentDefinition{
		Name:        "test",
		Description: "test",
		ToolsAllow:  []string{"read_file", "write_file", "web_search"},
		ToolsDeny:   []string{"write_file"},
	}
	got := def.FilterTools(all)
	expected := map[string]bool{"read_file": true, "web_search": true}
	if len(got) != len(expected) {
		t.Fatalf("expected %d tools, got %d: %v", len(expected), len(got), got)
	}
}

func TestFilterTools_NeitherRemovesSpawn(t *testing.T) {
	all := []string{"read_file", "spawn", "subagent", "web_search"}
	def := &AgentDefinition{
		Name:        "test",
		Description: "test",
	}
	got := def.FilterTools(all)
	for _, name := range got {
		if name == "spawn" || name == "subagent" {
			t.Fatalf("spawn/subagent should be removed when no allow/deny set, got: %v", got)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -run TestValidate -count=1`
Expected: Compilation error — package doesn't exist yet

- [ ] **Step 3: Implement AgentDefinition**

Create `pkg/agents/definition.go`:

```go
package agents

import (
	"fmt"
	"regexp"
	"time"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-]*$`)

const (
	maxNameLength        = 64
	maxDescriptionLength = 1024
	defaultMaxIterations = 20
	defaultTimeout       = 5 * time.Minute
)

type AgentDefinition struct {
	Name         string
	Description  string
	SystemPrompt string
	Model        string
	ToolsAllow   []string
	ToolsDeny    []string
	MaxIterations int
	Timeout      time.Duration
	Source       string // "builtin" | "workspace"
	BaseDir      string
}

func (d *AgentDefinition) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if len(d.Name) > maxNameLength {
		return fmt.Errorf("agent name must be at most %d characters, got %d", maxNameLength, len(d.Name))
	}
	if !namePattern.MatchString(d.Name) {
		return fmt.Errorf("agent name must be alphanumeric + hyphens, got %q", d.Name)
	}
	if d.Description == "" {
		return fmt.Errorf("agent description is required")
	}
	if len(d.Description) > maxDescriptionLength {
		return fmt.Errorf("agent description must be at most %d characters", maxDescriptionLength)
	}
	return nil
}

func (d *AgentDefinition) ApplyDefaults() {
	if d.MaxIterations <= 0 {
		d.MaxIterations = defaultMaxIterations
	}
	if d.Timeout <= 0 {
		d.Timeout = defaultTimeout
	}
}

func (d *AgentDefinition) FilterTools(allTools []string) []string {
	deny := make(map[string]bool)
	for _, t := range d.ToolsDeny {
		deny[t] = true
	}

	// Always deny spawn/subagent unless explicitly allowed
	hasExplicitAllow := len(d.ToolsAllow) > 0
	if !hasExplicitAllow {
		deny["spawn"] = true
		deny["subagent"] = true
	}

	var pool []string
	if hasExplicitAllow {
		allow := make(map[string]bool)
		for _, t := range d.ToolsAllow {
			allow[t] = true
		}
		for _, t := range allTools {
			if allow[t] {
				pool = append(pool, t)
			}
		}
	} else {
		pool = allTools
	}

	var result []string
	for _, t := range pool {
		if !deny[t] {
			result = append(result, t)
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/agents/definition.go pkg/agents/definition_test.go
git commit -m "feat(agents): add AgentDefinition struct with validation and tool filtering"
```

---

### Task 2: AGENT.md Loader

**Files:**
- Create: `pkg/agents/loader.go`
- Create: `pkg/agents/loader_test.go`

- [ ] **Step 1: Write the failing test for AGENT.md parsing**

Create `pkg/agents/loader_test.go`:

```go
package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAgentMD_Valid(t *testing.T) {
	content := `---
name: researcher
description: Gathers information from web and files
model: gpt-4
tools_deny:
  - write_file
  - exec
max_iterations: 15
timeout: 3m
---

You are a research agent. Gather information thoroughly.
`
	def, err := ParseAgentMD(content, "builtin", "/some/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Name != "researcher" {
		t.Fatalf("expected name=researcher, got %s", def.Name)
	}
	if def.Description != "Gathers information from web and files" {
		t.Fatalf("expected description mismatch, got %s", def.Description)
	}
	if def.Model != "gpt-4" {
		t.Fatalf("expected model=gpt-4, got %s", def.Model)
	}
	if len(def.ToolsDeny) != 2 {
		t.Fatalf("expected 2 denied tools, got %d", len(def.ToolsDeny))
	}
	if def.MaxIterations != 15 {
		t.Fatalf("expected max_iterations=15, got %d", def.MaxIterations)
	}
	if def.Timeout.Minutes() != 3 {
		t.Fatalf("expected timeout=3m, got %v", def.Timeout)
	}
	if def.SystemPrompt != "You are a research agent. Gather information thoroughly." {
		t.Fatalf("unexpected system prompt: %q", def.SystemPrompt)
	}
}

func TestParseAgentMD_MissingFrontmatter(t *testing.T) {
	content := "Just a markdown body without frontmatter."
	_, err := ParseAgentMD(content, "workspace", "/path")
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestParseAgentMD_MinimalValid(t *testing.T) {
	content := `---
name: simple
description: A simple agent
---

Do stuff.
`
	def, err := ParseAgentMD(content, "workspace", "/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.MaxIterations != defaultMaxIterations {
		t.Fatalf("expected default max_iterations, got %d", def.MaxIterations)
	}
	if def.Timeout != defaultTimeout {
		t.Fatalf("expected default timeout, got %v", def.Timeout)
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()

	// Create two agent directories
	researcherDir := filepath.Join(dir, "researcher")
	os.MkdirAll(researcherDir, 0755)
	os.WriteFile(filepath.Join(researcherDir, "AGENT.md"), []byte(`---
name: researcher
description: Research agent
---

Research things.
`), 0644)

	coderDir := filepath.Join(dir, "coder")
	os.MkdirAll(coderDir, 0755)
	os.WriteFile(filepath.Join(coderDir, "AGENT.md"), []byte(`---
name: coder
description: Coding agent
---

Write code.
`), 0644)

	// Create a broken agent
	brokenDir := filepath.Join(dir, "broken")
	os.MkdirAll(brokenDir, 0755)
	os.WriteFile(filepath.Join(brokenDir, "AGENT.md"), []byte(`no frontmatter here`), 0644)

	agents, warnings := LoadFromDir(dir)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for broken agent, got %d", len(warnings))
	}
}

func TestLoadFromDir_Empty(t *testing.T) {
	dir := t.TempDir()
	agents, warnings := LoadFromDir(dir)
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestLoadFromDir_NonExistent(t *testing.T) {
	agents, warnings := LoadFromDir("/nonexistent/path")
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d", len(warnings))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -run "TestParseAgentMD|TestLoadFromDir" -count=1`
Expected: Compilation error — ParseAgentMD and LoadFromDir not defined

- [ ] **Step 3: Implement the loader**

Create `pkg/agents/loader.go`:

```go
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type agentFrontmatter struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Model         string   `yaml:"model"`
	ToolsAllow    []string `yaml:"tools_allow"`
	ToolsDeny     []string `yaml:"tools_deny"`
	MaxIterations int      `yaml:"max_iterations"`
	Timeout       string   `yaml:"timeout"`
}

func ParseAgentMD(content, source, baseDir string) (*AgentDefinition, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var meta agentFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	var timeout time.Duration
	if meta.Timeout != "" {
		timeout, err = time.ParseDuration(meta.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", meta.Timeout, err)
		}
	}

	def := &AgentDefinition{
		Name:          meta.Name,
		Description:   meta.Description,
		SystemPrompt:  strings.TrimSpace(body),
		Model:         meta.Model,
		ToolsAllow:    meta.ToolsAllow,
		ToolsDeny:     meta.ToolsDeny,
		MaxIterations: meta.MaxIterations,
		Timeout:       timeout,
		Source:        source,
		BaseDir:       baseDir,
	}

	def.ApplyDefaults()

	if err := def.Validate(); err != nil {
		return nil, err
	}

	return def, nil
}

func LoadFromDir(dir string) ([]*AgentDefinition, []string) {
	var agents []*AgentDefinition
	var warnings []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agentFile := filepath.Join(dir, entry.Name(), "AGENT.md")
		content, err := os.ReadFile(agentFile)
		if err != nil {
			continue // no AGENT.md in this directory, skip silently
		}

		baseDir := filepath.Join(dir, entry.Name())
		def, err := ParseAgentMD(string(content), "workspace", baseDir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("agents/%s/AGENT.md failed to load: %s", entry.Name(), err))
			continue
		}

		agents = append(agents, def)
	}

	return agents, warnings
}

func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("missing YAML frontmatter (file must start with ---)")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", "", fmt.Errorf("unclosed frontmatter (missing closing ---)")
	}

	fm := strings.Join(lines[1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")
	body = strings.TrimLeft(body, "\n")

	return fm, body, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/agents/loader.go pkg/agents/loader_test.go
git commit -m "feat(agents): add AGENT.md loader with frontmatter parsing"
```

---

### Task 3: Agent Registry

**Files:**
- Create: `pkg/agents/registry.go`
- Create: `pkg/agents/registry_test.go`

- [ ] **Step 1: Write the failing test for registry operations**

Create `pkg/agents/registry_test.go`:

```go
package agents

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	def := &AgentDefinition{
		Name:          "test-agent",
		Description:   "A test agent",
		SystemPrompt:  "You are a test agent.",
		MaxIterations: 20,
		Timeout:       5 * time.Minute,
		Source:        "workspace",
	}
	r.Register(def)

	got := r.Get("test-agent")
	if got == nil {
		t.Fatal("expected to find test-agent")
	}
	if got.Description != "A test agent" {
		t.Fatalf("wrong description: %s", got.Description)
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	if r.Get("nonexistent") != nil {
		t.Fatal("expected nil for nonexistent agent")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentDefinition{Name: "a", Description: "A", SystemPrompt: "p", MaxIterations: 20, Timeout: 5 * time.Minute, Source: "builtin"})
	r.Register(&AgentDefinition{Name: "b", Description: "B", SystemPrompt: "p", MaxIterations: 20, Timeout: 5 * time.Minute, Source: "workspace"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(list))
	}
}

func TestRegistry_WorkspaceOverridesBuiltin(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentDefinition{Name: "researcher", Description: "builtin version", SystemPrompt: "p", Source: "builtin", MaxIterations: 20, Timeout: 5 * time.Minute})
	r.Register(&AgentDefinition{Name: "researcher", Description: "workspace version", SystemPrompt: "p", Source: "workspace", MaxIterations: 20, Timeout: 5 * time.Minute})

	got := r.Get("researcher")
	if got.Description != "workspace version" {
		t.Fatalf("expected workspace override, got %s", got.Description)
	}
}

func TestRegistry_Summary(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentDefinition{Name: "researcher", Description: "Gathers information", SystemPrompt: "p", MaxIterations: 20, Timeout: 5 * time.Minute, Source: "builtin"})
	r.Register(&AgentDefinition{Name: "coder", Description: "Writes code", SystemPrompt: "p", MaxIterations: 20, Timeout: 5 * time.Minute, Source: "builtin"})

	summary := r.Summary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if !contains(summary, "researcher") || !contains(summary, "coder") {
		t.Fatalf("summary missing agent names: %s", summary)
	}
	if !contains(summary, "Gathers information") || !contains(summary, "Writes code") {
		t.Fatalf("summary missing descriptions: %s", summary)
	}
}

func TestRegistry_SummaryWithWarnings(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentDefinition{Name: "ok", Description: "Works", SystemPrompt: "p", MaxIterations: 20, Timeout: 5 * time.Minute, Source: "builtin"})
	r.AddWarning("agents/broken/AGENT.md failed to load: missing name")

	summary := r.Summary()
	if !contains(summary, "WARNING") {
		t.Fatalf("expected warning in summary: %s", summary)
	}
}

func TestRegistry_Reload(t *testing.T) {
	dir := t.TempDir()

	agentDir := filepath.Join(dir, "dynamic")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(`---
name: dynamic
description: Dynamically loaded
---

Dynamic agent prompt.
`), 0644)

	r := NewRegistry()
	r.Reload(dir)

	got := r.Get("dynamic")
	if got == nil {
		t.Fatal("expected dynamic agent after reload")
	}
}

func TestRegistry_IsBuiltin(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentDefinition{Name: "researcher", Description: "builtin", SystemPrompt: "p", Source: "builtin", MaxIterations: 20, Timeout: 5 * time.Minute})

	if !r.IsBuiltin("researcher") {
		t.Fatal("expected researcher to be builtin")
	}
	if r.IsBuiltin("custom") {
		t.Fatal("expected custom to not be builtin")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -run "TestRegistry" -count=1`
Expected: Compilation error — NewRegistry, AddWarning, IsBuiltin not defined

- [ ] **Step 3: Implement the registry**

Create `pkg/agents/registry.go`:

```go
package agents

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	agents   map[string]*AgentDefinition
	builtins map[string]bool
	warnings []string
}

func NewRegistry() *Registry {
	return &Registry{
		agents:   make(map[string]*AgentDefinition),
		builtins: make(map[string]bool),
	}
}

func (r *Registry) Register(def *AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[def.Name] = def
	if def.Source == "builtin" {
		r.builtins[def.Name] = true
	}
}

func (r *Registry) Get(name string) *AgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[name]
}

func (r *Registry) List() []*AgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*AgentDefinition, 0, len(r.agents))
	for _, def := range r.agents {
		list = append(list, def)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func (r *Registry) IsBuiltin(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.builtins[name]
}

func (r *Registry) AddWarning(warning string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warnings = append(r.warnings, warning)
}

func (r *Registry) Summary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.agents) == 0 && len(r.warnings) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "Available agents:")

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		def := r.agents[name]
		lines = append(lines, fmt.Sprintf("- %s: %s", def.Name, def.Description))
	}

	for _, w := range r.warnings {
		lines = append(lines, fmt.Sprintf("WARNING: %s", w))
	}

	return strings.Join(lines, "\n")
}

func (r *Registry) Reload(workspaceAgentsDir string) {
	agents, warnings := LoadFromDir(workspaceAgentsDir)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear workspace agents and warnings, keep builtins
	for name, def := range r.agents {
		if def.Source != "builtin" {
			delete(r.agents, name)
		}
	}
	r.warnings = nil

	for _, def := range agents {
		r.agents[def.Name] = def
	}
	r.warnings = warnings
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/agents/registry.go pkg/agents/registry_test.go
git commit -m "feat(agents): add agent Registry with Get, List, Summary, Reload"
```

---

### Task 4: Builtin Agent Definitions

**Files:**
- Create: `pkg/agents/builtin/researcher/AGENT.md`
- Create: `pkg/agents/builtin/coder/AGENT.md`
- Create: `pkg/agents/builtin/reviewer/AGENT.md`
- Create: `pkg/agents/builtin/planner/AGENT.md`
- Create: `pkg/agents/builtin.go`

- [ ] **Step 1: Create the researcher AGENT.md**

Create `pkg/agents/builtin/researcher/AGENT.md`:

```markdown
---
name: researcher
description: Gathers information from web and files without making changes
tools_deny:
  - write_file
  - edit_file
  - append_file
  - exec
  - message
  - send_file
  - spawn
  - subagent
  - connect_mcp
  - disconnect_mcp
max_iterations: 20
timeout: 5m
---

You are a research agent for Spawnbot. Your job is to gather information thoroughly and report findings clearly.

You must NOT modify any files, execute commands, or send messages. You are read-only.

Focus on:
- Reading files to understand code, configuration, and documentation
- Searching the web for relevant information
- Fetching web pages for detailed content
- Searching memory for prior knowledge

Report your findings in a structured format:
- Lead with the key answer or finding
- Include relevant details with source references
- Flag any uncertainties or conflicting information
- Suggest follow-up research if needed
```

- [ ] **Step 2: Create the coder AGENT.md**

Create `pkg/agents/builtin/coder/AGENT.md`:

```markdown
---
name: coder
description: Writes code, scripts, and configuration files
tools_deny:
  - message
  - send_file
  - spawn
  - subagent
max_iterations: 20
timeout: 10m
---

You are a coding agent for Spawnbot. Your job is to write clean, working code and report what you built.

You must NOT communicate with users directly. Focus entirely on the coding task.

Guidelines:
- Read existing code before modifying it — understand patterns and conventions
- Write minimal, focused changes that accomplish the task
- Use exec to run tests or verify your work when possible
- Do not add unnecessary abstractions, comments, or error handling for impossible scenarios
- If something fails, diagnose the root cause before retrying

Report your results:
- What files were created or modified
- What the code does
- How to test or verify it
- Any issues encountered and how they were resolved
```

- [ ] **Step 3: Create the reviewer AGENT.md**

Create `pkg/agents/builtin/reviewer/AGENT.md`:

```markdown
---
name: reviewer
description: Reviews work for bugs, security issues, and improvements
tools_deny:
  - write_file
  - edit_file
  - append_file
  - exec
  - message
  - send_file
  - spawn
  - subagent
max_iterations: 15
timeout: 5m
---

You are a code review agent for Spawnbot. Your job is to review work and identify issues.

You must NOT modify any files or execute commands. You are read-only.

Review checklist:
- Logic errors and bugs
- Security vulnerabilities (injection, path traversal, credential exposure)
- Error handling gaps (silent failures, swallowed errors)
- Race conditions and concurrency issues
- Performance concerns
- Deviation from existing code patterns and conventions

Report format:
- Severity: CRITICAL / HIGH / MEDIUM / LOW
- Location: file path and line number or function name
- Issue: clear description of the problem
- Suggestion: how to fix it

Only report issues you are confident about. Do not pad the review with style nitpicks.
```

- [ ] **Step 4: Create the planner AGENT.md**

Create `pkg/agents/builtin/planner/AGENT.md`:

```markdown
---
name: planner
description: Breaks down complex tasks into structured plans with dependencies
tools_deny:
  - exec
  - message
  - send_file
  - spawn
  - subagent
max_iterations: 20
timeout: 5m
---

You are a planning agent for Spawnbot. Your job is to analyze tasks and produce structured implementation plans.

You can read files to understand the current state and write plan documents to the workspace.

Planning process:
1. Understand the goal — what needs to be achieved
2. Research the current state — read relevant files, check existing patterns
3. Identify dependencies — what must happen before what
4. Break into steps — each step should be independently completable
5. Estimate complexity — flag steps that are risky or uncertain

Plan format:
- Goal: one sentence
- Steps: numbered, with dependencies noted
- For each step: what to do, which files to touch, what to verify
- Risks: anything that could go wrong or block progress

Keep plans concrete and actionable. Avoid vague steps like "implement the feature" — break them down into specific file changes.
```

- [ ] **Step 5: Create builtin.go with embedded agents**

Create `pkg/agents/builtin.go`:

```go
package agents

import "embed"

//go:embed builtin/researcher/AGENT.md
var researcherAgentMD string

//go:embed builtin/coder/AGENT.md
var coderAgentMD string

//go:embed builtin/reviewer/AGENT.md
var reviewerAgentMD string

//go:embed builtin/planner/AGENT.md
var plannerAgentMD string

func (r *Registry) LoadBuiltins() error {
	builtins := map[string]string{
		"researcher": researcherAgentMD,
		"coder":      coderAgentMD,
		"reviewer":   reviewerAgentMD,
		"planner":    plannerAgentMD,
	}

	for name, content := range builtins {
		def, err := ParseAgentMD(content, "builtin", "")
		if err != nil {
			return fmt.Errorf("failed to parse builtin agent %q: %w", name, err)
		}
		r.Register(def)
	}

	return nil
}
```

Note: add `"fmt"` to the imports.

- [ ] **Step 6: Run all agent tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -count=1`
Expected: All tests PASS (builtins parse correctly via existing ParseAgentMD tests; LoadBuiltins tested indirectly)

- [ ] **Step 7: Commit**

```bash
git add pkg/agents/builtin/ pkg/agents/builtin.go
git commit -m "feat(agents): add 4 builtin agent definitions (researcher, coder, reviewer, planner)"
```

---

### Task 5: Integrate Registry into AgentInstance

**Files:**
- Modify: `pkg/agent/instance.go` — add `AgentRegistry` field
- Modify: `pkg/agent/context.go` — inject agent summary into system prompt
- Modify: `pkg/agent/loop.go` — wire up registry initialization

- [ ] **Step 1: Read the current files to understand exact integration points**

Read these files before making changes:
- `pkg/agent/instance.go` — find where `AgentInstance` struct is defined
- `pkg/agent/context.go` — find `BuildSystemPrompt()` where skills summary is injected
- `pkg/agent/loop.go` — find where `AgentInstance` is initialized and where `SubTurnConfig` is built

- [ ] **Step 2: Add AgentRegistry field to AgentInstance**

In `pkg/agent/instance.go`, add to the `AgentInstance` struct:

```go
AgentRegistry *agents.Registry
```

Add the import:
```go
"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
```

- [ ] **Step 3: Add agent registry to ContextBuilder**

In `pkg/agent/context.go`, add to the `ContextBuilder` struct:

```go
agentRegistry *agents.Registry
```

Add a setter method:

```go
func (cb *ContextBuilder) SetAgentRegistry(r *agents.Registry) {
	cb.agentRegistry = r
}
```

- [ ] **Step 4: Inject agent summary into BuildSystemPrompt()**

In `pkg/agent/context.go`, in the `BuildSystemPrompt()` function, after the skills summary injection (around line 141 where `BuildSkillsSummary()` is called), add:

```go
// Agent definitions summary
if cb.agentRegistry != nil {
	agentSummary := cb.agentRegistry.Summary()
	if agentSummary != "" {
		parts = append(parts, fmt.Sprintf("# Agents\n\nYou can spawn specialized agents using the spawn or subagent tools with the agent_type parameter.\n\n%s", agentSummary))
	}
}
```

- [ ] **Step 5: Initialize registry in the agent startup path**

In `pkg/agent/loop.go`, find where `AgentInstance` is set up (where tools are registered and `ContextBuilder` is created). Add registry initialization:

```go
// Initialize agent registry
agentRegistry := agents.NewRegistry()
if err := agentRegistry.LoadBuiltins(); err != nil {
	logger.WarnCF("agent", "Failed to load builtin agents", map[string]any{"error": err.Error()})
}
workspaceAgentsDir := filepath.Join(agent.Workspace, "agents")
agentRegistry.Reload(workspaceAgentsDir)
agent.AgentRegistry = agentRegistry
agent.ContextBuilder.SetAgentRegistry(agentRegistry)
```

- [ ] **Step 6: Build and verify compilation**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go build ./...`
Expected: Compiles without errors

- [ ] **Step 7: Commit**

```bash
git add pkg/agent/instance.go pkg/agent/context.go pkg/agent/loop.go
git commit -m "feat(agents): integrate agent registry into AgentInstance and system prompt"
```

---

### Task 6: Add agent_type to spawn/subagent Tools

**Files:**
- Modify: `pkg/tools/subagent.go` — add `agent_type` parameter, resolve from registry
- Modify: `pkg/agent/loop.go` — update SubTurn spawner to use agent definitions

- [ ] **Step 1: Read current subagent.go to understand exact structure**

Read `pkg/tools/subagent.go` — find the `Parameters()` method and `Execute()` method for both the spawn and subagent tools.

- [ ] **Step 2: Add agent_type parameter to subagent tool**

In `pkg/tools/subagent.go`, in the subagent tool's `Parameters()` method, add:

```go
"agent_type": map[string]any{
	"type":        "string",
	"description": "The type of agent to use (e.g. researcher, coder, reviewer, planner). Omit for a general-purpose subagent.",
},
```

- [ ] **Step 3: Add agent_type parameter to spawn tool**

In the spawn tool's `Parameters()` method (same file or `pkg/tools/spawn.go` — check which file), add the same parameter.

- [ ] **Step 4: Update SubTurnConfig to carry agent_type**

In `pkg/tools/subagent.go`, add to `SubTurnConfig`:

```go
AgentType string // agent type name from registry, empty = default
```

- [ ] **Step 5: Extract agent_type in Execute() methods**

In both spawn and subagent `Execute()` methods, extract the parameter:

```go
agentType, _ := args["agent_type"].(string)
```

Pass it through to `SubTurnConfig`:

```go
cfg.AgentType = agentType
```

- [ ] **Step 6: Update SubTurn spawner in loop.go to resolve agent definitions**

In `pkg/agent/loop.go`, in the function that builds the subagent's `SubTurnConfig` (around lines 301-357), add agent resolution before the existing system prompt construction:

```go
if cfg.AgentType != "" {
	if al.agent.AgentRegistry == nil {
		return tools.ErrorResult("agent registry not initialized"), nil
	}
	agentDef := al.agent.AgentRegistry.Get(cfg.AgentType)
	if agentDef == nil {
		available := al.agent.AgentRegistry.Summary()
		return tools.ErrorResult(fmt.Sprintf("agent type %q not found. %s", cfg.AgentType, available)), nil
	}

	// Use agent definition's system prompt
	systemPrompt = agentDef.SystemPrompt + "\n\nTask: " + task

	// Apply model override
	if agentDef.Model != "" {
		modelToUse = agentDef.Model
	}

	// Filter tools
	allToolNames := make([]string, 0)
	for _, t := range tlSlice {
		allToolNames = append(allToolNames, t.Name())
	}
	allowedNames := agentDef.FilterTools(allToolNames)
	allowedSet := make(map[string]bool)
	for _, n := range allowedNames {
		allowedSet[n] = true
	}
	var filteredTools []tools.Tool
	for _, t := range tlSlice {
		if allowedSet[t.Name()] {
			filteredTools = append(filteredTools, t)
		}
	}
	if len(filteredTools) == 0 {
		return tools.ErrorResult(fmt.Sprintf("agent %q has no tools available after filtering — check tools_allow/tools_deny", cfg.AgentType)), nil
	}
	tlSlice = filteredTools

	// Apply iteration/timeout overrides
	if agentDef.MaxIterations > 0 {
		cfg.MaxIterations = agentDef.MaxIterations
	}
	if agentDef.Timeout > 0 {
		cfg.Timeout = agentDef.Timeout
	}
}
```

Note: The exact variable names (`systemPrompt`, `modelToUse`, `tlSlice`, `task`) will need to match the local variables in the actual function. Read the file first in Step 1 to confirm.

- [ ] **Step 7: Build and verify compilation**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go build ./...`
Expected: Compiles without errors

- [ ] **Step 8: Commit**

```bash
git add pkg/tools/subagent.go pkg/agent/loop.go
git commit -m "feat(agents): add agent_type parameter to spawn/subagent tools with registry resolution"
```

---

### Task 7: create_agent Tool

**Files:**
- Create: `pkg/tools/create_agent.go`
- Create: `pkg/tools/create_agent_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/tools/create_agent_test.go`:

```go
package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
)

func TestCreateAgentTool_Success(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()

	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name":         "sql-expert",
		"description":  "Specializes in database queries",
		"system_prompt": "You are a SQL expert agent.",
		"tools_deny":   []any{"message", "send_file"},
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	// Verify file was created
	agentFile := filepath.Join(dir, "sql-expert", "AGENT.md")
	if _, err := os.Stat(agentFile); os.IsNotExist(err) {
		t.Fatal("AGENT.md was not created")
	}

	// Verify registered in registry
	if registry.Get("sql-expert") == nil {
		t.Fatal("agent not registered in registry")
	}
}

func TestCreateAgentTool_RejectBuiltinOverwrite(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()
	registry.Register(&agents.AgentDefinition{
		Name: "researcher", Description: "builtin", SystemPrompt: "p",
		Source: "builtin", MaxIterations: 20, Timeout: 5 * 60e9,
	})

	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name":         "researcher",
		"description":  "custom researcher",
		"system_prompt": "override",
	})

	if result.Error == nil {
		t.Fatal("expected error when overwriting builtin")
	}
}

func TestCreateAgentTool_InvalidName(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()
	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name":         "bad name!",
		"description":  "invalid",
		"system_prompt": "prompt",
	})

	if result.Error == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestCreateAgentTool_MissingRequired(t *testing.T) {
	dir := t.TempDir()
	registry := agents.NewRegistry()
	tool := NewCreateAgentTool(dir, registry)

	result := tool.Execute(context.Background(), map[string]any{
		"name": "test",
	})

	if result.Error == nil {
		t.Fatal("expected error for missing description")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/ -v -run "TestCreateAgent" -count=1`
Expected: Compilation error — NewCreateAgentTool not defined

- [ ] **Step 3: Implement create_agent tool**

Create `pkg/tools/create_agent.go`:

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
)

type CreateAgentTool struct {
	agentsDir string
	registry  *agents.Registry
}

func NewCreateAgentTool(agentsDir string, registry *agents.Registry) *CreateAgentTool {
	return &CreateAgentTool{
		agentsDir: agentsDir,
		registry:  registry,
	}
}

func (t *CreateAgentTool) Name() string {
	return "create_agent"
}

func (t *CreateAgentTool) Description() string {
	return "Create a new agent type definition. The agent becomes immediately available for use with spawn/subagent tools."
}

func (t *CreateAgentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Unique agent name (alphanumeric + hyphens, max 64 chars)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "What this agent does (shown in agent summary)",
			},
			"system_prompt": map[string]any{
				"type":        "string",
				"description": "The agent's system prompt (markdown)",
			},
			"tools_allow": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tools to allow (empty = all tools)",
			},
			"tools_deny": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tools to deny",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Model override (empty = inherit parent model)",
			},
			"max_iterations": map[string]any{
				"type":        "integer",
				"description": "Maximum iterations (default 20)",
			},
			"timeout": map[string]any{
				"type":        "string",
				"description": "Timeout duration (default 5m)",
			},
		},
		"required": []string{"name", "description", "system_prompt"},
	}
}

func (t *CreateAgentTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	systemPrompt, _ := args["system_prompt"].(string)
	model, _ := args["model"].(string)
	maxIters, _ := args["max_iterations"].(float64)
	timeoutStr, _ := args["timeout"].(string)

	// Check builtin protection
	if t.registry.IsBuiltin(name) {
		return ErrorResult(fmt.Sprintf("cannot overwrite builtin agent %q — use a different name", name))
	}

	// Build frontmatter
	var fm strings.Builder
	fm.WriteString("---\n")
	fm.WriteString(fmt.Sprintf("name: %s\n", name))
	fm.WriteString(fmt.Sprintf("description: %q\n", description))
	if model != "" {
		fm.WriteString(fmt.Sprintf("model: %s\n", model))
	}

	if toolsAllow, ok := args["tools_allow"]; ok {
		if arr, ok := toolsAllow.([]any); ok && len(arr) > 0 {
			fm.WriteString("tools_allow:\n")
			for _, v := range arr {
				fm.WriteString(fmt.Sprintf("  - %s\n", v))
			}
		}
	}

	if toolsDeny, ok := args["tools_deny"]; ok {
		if arr, ok := toolsDeny.([]any); ok && len(arr) > 0 {
			fm.WriteString("tools_deny:\n")
			for _, v := range arr {
				fm.WriteString(fmt.Sprintf("  - %s\n", v))
			}
		}
	}

	if maxIters > 0 {
		fm.WriteString(fmt.Sprintf("max_iterations: %d\n", int(maxIters)))
	}
	if timeoutStr != "" {
		fm.WriteString(fmt.Sprintf("timeout: %s\n", timeoutStr))
	}
	fm.WriteString("---\n\n")
	fm.WriteString(systemPrompt)

	content := fm.String()

	// Validate by parsing
	def, err := agents.ParseAgentMD(content, "workspace", filepath.Join(t.agentsDir, name))
	if err != nil {
		return ErrorResult(fmt.Sprintf("generated AGENT.md failed validation: %s", err))
	}

	// Write to disk
	agentDir := filepath.Join(t.agentsDir, name)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create agent directory: %s", err))
	}

	agentFile := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
		return ErrorResult(fmt.Sprintf("failed to write AGENT.md: %s", err))
	}

	// Register immediately
	t.registry.Register(def)

	return NewToolResult(fmt.Sprintf("Agent %q created and registered. Available immediately for spawn/subagent with agent_type=%q.", name, name))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/tools/ -v -run "TestCreateAgent" -count=1`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/create_agent.go pkg/tools/create_agent_test.go
git commit -m "feat(agents): add create_agent tool for autonomous agent creation"
```

---

### Task 8: Register create_agent Tool in Agent Startup

**Files:**
- Modify: `pkg/agent/instance.go` or `pkg/agent/loop.go` — register the create_agent tool alongside other tools

- [ ] **Step 1: Read the tool registration code**

Read `pkg/agent/instance.go` (or wherever tools are registered — around lines 77-105) to find the exact pattern.

- [ ] **Step 2: Register create_agent tool**

In the tool registration section, add:

```go
workspaceAgentsDir := filepath.Join(agent.Workspace, "agents")
if agent.AgentRegistry != nil {
	toolsRegistry.Register(NewCreateAgentTool(workspaceAgentsDir, agent.AgentRegistry))
}
```

This should go after the agent registry is initialized (Task 5) and before the agent starts accepting messages.

- [ ] **Step 3: Build and verify compilation**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go build ./...`
Expected: Compiles without errors

- [ ] **Step 4: Commit**

```bash
git add pkg/agent/instance.go
git commit -m "feat(agents): register create_agent tool in agent startup"
```

---

### Task 9: End-to-End Integration Test

**Files:**
- Create: `pkg/agents/integration_test.go`

- [ ] **Step 1: Write the integration test**

Create `pkg/agents/integration_test.go`:

```go
package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEndToEnd_BuiltinsLoadAndSummarize(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadBuiltins(); err != nil {
		t.Fatalf("failed to load builtins: %v", err)
	}

	list := r.List()
	if len(list) != 4 {
		t.Fatalf("expected 4 builtin agents, got %d", len(list))
	}

	// Verify all 4 are present
	names := map[string]bool{}
	for _, a := range list {
		names[a.Name] = true
	}
	for _, expected := range []string{"researcher", "coder", "reviewer", "planner"} {
		if !names[expected] {
			t.Fatalf("missing builtin agent: %s", expected)
		}
	}

	// Verify summary format
	summary := r.Summary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	for _, name := range []string{"researcher", "coder", "reviewer", "planner"} {
		if !containsStr(summary, name) {
			t.Fatalf("summary missing %s: %s", name, summary)
		}
	}
}

func TestEndToEnd_WorkspaceOverridesBuiltin(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadBuiltins(); err != nil {
		t.Fatal(err)
	}

	// Verify builtin researcher
	original := r.Get("researcher")
	if original.Source != "builtin" {
		t.Fatalf("expected builtin source, got %s", original.Source)
	}

	// Create workspace override
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "researcher")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(`---
name: researcher
description: Custom workspace researcher with extra capabilities
tools_deny:
  - exec
---

You are a custom research agent with workspace-specific instructions.
`), 0644)

	r.Reload(dir)

	// Verify override
	overridden := r.Get("researcher")
	if overridden.Source != "workspace" {
		t.Fatalf("expected workspace source after override, got %s", overridden.Source)
	}
	if overridden.Description != "Custom workspace researcher with extra capabilities" {
		t.Fatalf("description not overridden: %s", overridden.Description)
	}

	// Verify other builtins still present
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

	// Researcher should only have read-only tools
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

	// Coder should have file tools but not messaging
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
	if !containsStr(summary, "WARNING") {
		t.Fatalf("expected warning in summary for broken agent: %s", summary)
	}
	if !containsStr(summary, "broken-agent") {
		t.Fatalf("warning should reference the broken agent name: %s", summary)
	}
}
```

- [ ] **Step 2: Run integration tests**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -run "TestEndToEnd" -count=1`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/agents/integration_test.go
git commit -m "test(agents): add end-to-end integration tests for agent registry"
```

---

### Task 10: Full Build Verification

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go test ./pkg/agents/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 2: Run build**

Run: `cd /home/eugen-dev/Workflows/picoclaw && go build ./cmd/spawnbot/`
Expected: Compiles without errors

- [ ] **Step 3: Run existing tests to verify no regressions**

Run: `cd /home/eugen-dev/Workflows/picoclaw && CGO_ENABLED=1 go test ./pkg/tools/ -v -count=1 -tags fts5`
Expected: All existing tests still PASS

- [ ] **Step 4: Verify agent summary appears in system prompt**

Run the agent in interactive mode and check that the system prompt now includes the "Available agents:" section. This is a manual verification:

```bash
cd /home/eugen-dev/Workflows/picoclaw && SPAWNBOT_DEBUG=1 ./spawnbot agent -m "list your available agents"
```

Expected: Agent response mentions researcher, coder, reviewer, planner

- [ ] **Step 5: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "fix(agents): address issues found during integration verification"
```
