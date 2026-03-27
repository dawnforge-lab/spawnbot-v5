package onboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/workspace"
)

func TestDeployCreatesStructuredAgentFiles(t *testing.T) {
	targetDir := t.TempDir()

	if err := workspace.Deploy(targetDir, workspace.TemplateData{UserName: "TestUser"}); err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}

	// Verify all identity files are created
	for _, name := range []string{"SOUL.md", "USER.md", "GOALS.md", "PLAYBOOK.md", "HEARTBEAT.md"} {
		path := filepath.Join(targetDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}

	// Verify SOUL.md was rendered with the user name
	soulData, err := os.ReadFile(filepath.Join(targetDir, "SOUL.md"))
	if err != nil {
		t.Fatalf("failed to read SOUL.md: %v", err)
	}
	if !strings.Contains(string(soulData), "TestUser") {
		t.Errorf("expected SOUL.md to contain 'TestUser', got:\n%s", string(soulData))
	}

	// Verify skills are deployed
	skillCreatorPath := filepath.Join(targetDir, "skills", "skill-creator", "SKILL.md")
	if _, err := os.Stat(skillCreatorPath); err != nil {
		t.Fatalf("expected skill-creator/SKILL.md to exist: %v", err)
	}

	// Verify memory directory
	memoryPath := filepath.Join(targetDir, "memory", "MEMORY.md")
	if _, err := os.Stat(memoryPath); err != nil {
		t.Fatalf("expected memory/MEMORY.md to exist: %v", err)
	}

	// Verify legacy files are absent
	for _, legacyName := range []string{"AGENT.md", "AGENTS.md", "IDENTITY.md"} {
		legacyPath := filepath.Join(targetDir, legacyName)
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Fatalf("expected legacy file %s to be absent", legacyName)
		}
	}
}
