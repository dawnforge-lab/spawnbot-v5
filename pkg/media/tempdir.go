package media

import (
	"os"
	"path/filepath"
	"sync"
)

const TempDirName = "media"

var (
	workspaceDir string
	wdMu         sync.RWMutex
)

// SetWorkspace sets the workspace root so media files are stored inside it.
// Must be called during startup before any media operations.
func SetWorkspace(workspace string) {
	wdMu.Lock()
	defer wdMu.Unlock()
	workspaceDir = workspace
}

// TempDir returns the directory used for downloaded media.
// If a workspace has been set, returns workspace/media/ so files are
// accessible to the agent's file tools. Falls back to /tmp/spawnbot_media/.
func TempDir() string {
	wdMu.RLock()
	ws := workspaceDir
	wdMu.RUnlock()
	if ws != "" {
		return filepath.Join(ws, TempDirName)
	}
	return filepath.Join(os.TempDir(), TempDirName)
}
