package agents

import (
	"strings"
	"testing"
	"time"
)

func TestValidateDefinition_Valid(t *testing.T) {
	d := &AgentDefinition{
		Name:        "my-agent",
		Description: "A valid agent description",
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateDefinition_MissingName(t *testing.T) {
	d := &AgentDefinition{
		Description: "A valid description",
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
	if err.Error() != "agent name is required" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateDefinition_InvalidNameChars(t *testing.T) {
	cases := []string{
		"-starts-with-hyphen",
		"has spaces",
		"has_underscore",
		"has.dot",
		"has@symbol",
	}
	for _, name := range cases {
		d := &AgentDefinition{Name: name, Description: "desc"}
		err := d.Validate()
		if err == nil {
			t.Errorf("expected error for name %q, got nil", name)
		}
	}
}

func TestValidateDefinition_NameTooLong(t *testing.T) {
	d := &AgentDefinition{
		Name:        strings.Repeat("a", maxNameLength+1),
		Description: "desc",
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for name too long, got nil")
	}
}

func TestValidateDefinition_MissingDescription(t *testing.T) {
	d := &AgentDefinition{
		Name: "valid-name",
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for missing description, got nil")
	}
	if err.Error() != "agent description is required" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateDefinition_Defaults(t *testing.T) {
	d := &AgentDefinition{
		Name:        "test-agent",
		Description: "desc",
	}
	d.ApplyDefaults()
	if d.MaxIterations != defaultMaxIterations {
		t.Errorf("expected MaxIterations=%d, got %d", defaultMaxIterations, d.MaxIterations)
	}
	if d.Timeout != defaultTimeout {
		t.Errorf("expected Timeout=%v, got %v", defaultTimeout, d.Timeout)
	}

	// ApplyDefaults should not override values that are already set
	d2 := &AgentDefinition{
		Name:          "test-agent",
		Description:   "desc",
		MaxIterations: 5,
		Timeout:       10 * time.Second,
	}
	d2.ApplyDefaults()
	if d2.MaxIterations != 5 {
		t.Errorf("expected MaxIterations=5 to be preserved, got %d", d2.MaxIterations)
	}
	if d2.Timeout != 10*time.Second {
		t.Errorf("expected Timeout=10s to be preserved, got %v", d2.Timeout)
	}
}

// TestFilterTools_DenyOnly: input has spawn/subagent, deny has write_file/exec
// expect only read_file and web_search
func TestFilterTools_DenyOnly(t *testing.T) {
	d := &AgentDefinition{
		Name:        "test",
		Description: "desc",
		ToolsDeny:   []string{"write_file", "exec"},
	}
	allTools := []string{"read_file", "web_search", "write_file", "exec", "spawn", "subagent"}
	result := d.FilterTools(allTools)

	want := map[string]bool{"read_file": true, "web_search": true}
	if len(result) != len(want) {
		t.Fatalf("expected %d tools, got %d: %v", len(want), len(result), result)
	}
	for _, tool := range result {
		if !want[tool] {
			t.Errorf("unexpected tool in result: %q", tool)
		}
	}
}

func TestFilterTools_AllowOnly(t *testing.T) {
	d := &AgentDefinition{
		Name:        "test",
		Description: "desc",
		ToolsAllow:  []string{"read_file", "web_search"},
	}
	allTools := []string{"read_file", "web_search", "write_file", "exec", "spawn", "subagent"}
	result := d.FilterTools(allTools)

	want := map[string]bool{"read_file": true, "web_search": true}
	if len(result) != len(want) {
		t.Fatalf("expected %d tools, got %d: %v", len(want), len(result), result)
	}
	for _, tool := range result {
		if !want[tool] {
			t.Errorf("unexpected tool in result: %q", tool)
		}
	}
}

func TestFilterTools_AllowAndDeny(t *testing.T) {
	d := &AgentDefinition{
		Name:        "test",
		Description: "desc",
		ToolsAllow:  []string{"read_file", "web_search", "write_file"},
		ToolsDeny:   []string{"write_file"},
	}
	allTools := []string{"read_file", "web_search", "write_file", "exec", "spawn", "subagent"}
	result := d.FilterTools(allTools)

	want := map[string]bool{"read_file": true, "web_search": true}
	if len(result) != len(want) {
		t.Fatalf("expected %d tools, got %d: %v", len(want), len(result), result)
	}
	for _, tool := range result {
		if !want[tool] {
			t.Errorf("unexpected tool in result: %q", tool)
		}
	}
}

func TestFilterTools_NeitherRemovesSpawn(t *testing.T) {
	d := &AgentDefinition{
		Name:        "test",
		Description: "desc",
		// No ToolsAllow, no ToolsDeny
	}
	allTools := []string{"read_file", "web_search", "spawn", "subagent"}
	result := d.FilterTools(allTools)

	for _, tool := range result {
		if tool == "spawn" || tool == "subagent" {
			t.Errorf("spawn/subagent should have been removed, but found %q in result", tool)
		}
	}
	// Should contain read_file and web_search
	found := make(map[string]bool)
	for _, t := range result {
		found[t] = true
	}
	if !found["read_file"] || !found["web_search"] {
		t.Errorf("expected read_file and web_search in result, got: %v", result)
	}
}
