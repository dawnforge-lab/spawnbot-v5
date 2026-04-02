package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBootstrapFiles_SoulAndAgents(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("You are Spawnbot."), 0644)
	os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("## Rules\n1. Update memory"), 0644)

	cb := NewContextBuilder(workspace)
	content, err := cb.LoadBootstrapFiles()
	require.NoError(t, err)

	assert.Contains(t, content, "You are Spawnbot")
	assert.Contains(t, content, "SOUL.md")
	assert.Contains(t, content, "Update memory")
	assert.Contains(t, content, "AGENTS.md")
}

func TestLoadBootstrapFiles_NoAgentsMd(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("You are Spawnbot."), 0644)

	cb := NewContextBuilder(workspace)
	content, err := cb.LoadBootstrapFiles()
	require.NoError(t, err)

	assert.Contains(t, content, "You are Spawnbot")
	assert.NotContains(t, content, "AGENTS.md")
}

func TestLoadBootstrapFiles_ErrorsWhenSoulMdMissing(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilder(workspace)
	_, err := cb.LoadBootstrapFiles()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SOUL.md")
}
