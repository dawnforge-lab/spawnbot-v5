package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempSkill(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	path := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func newTestLoader() *SkillsLoader {
	return NewSkillsLoader("", "", "")
}

// TestGetSkillMetadata_NewFields verifies all new fields are parsed from YAML frontmatter.
func TestGetSkillMetadata_NewFields(t *testing.T) {
	content := `---
name: my-skill
description: A skill with all new fields
arguments:
  - file
  - target
argument-hint: "<file> [target]"
context: fork
agent_type: coder
allowed_tools:
  - read_file
  - write_file
user-invocable: false
---

# my-skill

Body text.
`
	sl := newTestLoader()
	path := writeTempSkill(t, content)
	meta := sl.getSkillMetadata(path)
	require.NotNil(t, meta)

	assert.Equal(t, "my-skill", meta.Name)
	assert.Equal(t, "A skill with all new fields", meta.Description)
	assert.Equal(t, []string{"file", "target"}, meta.Arguments)
	assert.Equal(t, "<file> [target]", meta.ArgumentHint)
	assert.Equal(t, "fork", meta.Context)
	assert.Equal(t, "coder", meta.AgentType)
	assert.Equal(t, []string{"read_file", "write_file"}, meta.AllowedTools)
	assert.False(t, meta.UserInvocable)
}

// TestGetSkillMetadata_Defaults verifies that omitted fields receive their defaults.
func TestGetSkillMetadata_Defaults(t *testing.T) {
	content := `---
name: simple-skill
description: Minimal skill
---

# simple-skill

Body.
`
	sl := newTestLoader()
	path := writeTempSkill(t, content)
	meta := sl.getSkillMetadata(path)
	require.NotNil(t, meta)

	assert.Equal(t, "simple-skill", meta.Name)
	assert.Equal(t, "inline", meta.Context, "context should default to inline")
	assert.True(t, meta.UserInvocable, "user-invocable should default to true")
	assert.Empty(t, meta.Arguments, "arguments should default to empty")
	assert.Empty(t, meta.AllowedTools, "allowed_tools should default to empty")
	assert.Empty(t, meta.ArgumentHint, "argument-hint should default to empty")
}

// TestGetSkillMetadata_InvalidContext verifies that an invalid context value falls back to inline.
func TestGetSkillMetadata_InvalidContext(t *testing.T) {
	content := `---
name: bad-context
description: Skill with invalid context
context: thread
---

# bad-context

Body.
`
	sl := newTestLoader()
	path := writeTempSkill(t, content)
	meta := sl.getSkillMetadata(path)
	require.NotNil(t, meta)

	assert.Equal(t, "inline", meta.Context, "invalid context should fall back to inline")
}
