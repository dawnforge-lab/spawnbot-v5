package tools

import (
	"context"
	"testing"
)

func TestUseSkillTool_Name(t *testing.T) {
	tool := NewUseSkillTool(nil, nil, "", "")
	if tool.Name() != "use_skill" {
		t.Errorf("expected name %q, got %q", "use_skill", tool.Name())
	}
}

func TestUseSkillTool_Parameters(t *testing.T) {
	tool := NewUseSkillTool(nil, nil, "", "")
	params := tool.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["skill"]; !ok {
		t.Error("missing 'skill' parameter")
	}
	if _, ok := props["arguments"]; !ok {
		t.Error("missing 'arguments' parameter")
	}
}

func TestUseSkillTool_SkillNotFound(t *testing.T) {
	tool := NewUseSkillTool(nil, nil, "", "")
	result := tool.Execute(context.Background(), map[string]any{
		"skill": "nonexistent",
	})
	if !result.IsError {
		t.Error("expected error for nonexistent skill")
	}
}

func TestUseSkillTool_MissingSkillName(t *testing.T) {
	tool := NewUseSkillTool(nil, nil, "", "")
	result := tool.Execute(context.Background(), map[string]any{})
	if !result.IsError {
		t.Error("expected error for missing skill name")
	}
}
