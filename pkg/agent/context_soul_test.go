package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBootstrapFiles_OnlySoulMd(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("You are Spawnbot."), 0644)
	os.WriteFile(filepath.Join(workspace, "AGENT.md"), []byte("agent stuff"), 0644)
	os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("agents stuff"), 0644)
	os.WriteFile(filepath.Join(workspace, "IDENTITY.md"), []byte("identity stuff"), 0644)

	cb := NewContextBuilder(workspace)
	content, err := cb.LoadBootstrapFiles()
	require.NoError(t, err)

	assert.Contains(t, content, "You are Spawnbot")
	assert.NotContains(t, content, "agent stuff")
	assert.NotContains(t, content, "agents stuff")
	assert.NotContains(t, content, "identity stuff")
}

func TestLoadBootstrapFiles_ErrorsWhenSoulMdMissing(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilder(workspace)
	_, err := cb.LoadBootstrapFiles()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SOUL.md")
}
