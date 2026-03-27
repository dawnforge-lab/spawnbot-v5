package onboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyEmbeddedToTargetUsesStructuredAgentFiles(t *testing.T) {
	targetDir := t.TempDir()

	if err := copyEmbeddedToTarget(targetDir, "TestUser"); err != nil {
		t.Fatalf("copyEmbeddedToTarget() error = %v", err)
	}

	// Verify all identity files are created
	for _, name := range []string{"SOUL.md", "USER.md", "GOALS.md", "PLAYBOOK.md", "HEARTBEAT.md"} {
		path := filepath.Join(targetDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	// Verify SOUL.md was rendered with the user name
	soulData, err := os.ReadFile(filepath.Join(targetDir, "SOUL.md"))
	if err != nil {
		t.Fatalf("failed to read SOUL.md: %v", err)
	}
	if got := string(soulData); !contains(got, "TestUser") {
		t.Errorf("expected SOUL.md to contain 'TestUser', got:\n%s", got)
	}

	for _, legacyName := range []string{"AGENT.md", "AGENTS.md", "IDENTITY.md"} {
		legacyPath := filepath.Join(targetDir, legacyName)
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Fatalf("expected legacy file %s to be absent, got err=%v", legacyPath, err)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
