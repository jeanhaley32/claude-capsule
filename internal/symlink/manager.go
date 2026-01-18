package symlink

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jeanhaley32/claude-capsule/internal/constants"
)

// Manager implements SymlinkManager.
type Manager struct{}

// NewManager creates a new symlink manager.
func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) CreateSymlink(workspacePath, volumeMountPoint, repoID string) error {
	// Validate inputs
	if workspacePath == "" || volumeMountPoint == "" || repoID == "" {
		return fmt.Errorf("all parameters are required: workspace=%q, mount=%q, repo=%q",
			workspacePath, volumeMountPoint, repoID)
	}

	symlinkPath := filepath.Join(workspacePath, constants.DocsSymlinkName)
	targetPath := filepath.Join(volumeMountPoint, "repos", repoID)

	// Ensure target directory exists
	if err := os.MkdirAll(targetPath, constants.DirPermissions); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Use atomic symlink replacement to avoid race conditions:
	// 1. Create symlink with temporary name
	// 2. Rename to final name (atomic on Unix)
	tempPath := symlinkPath + ".tmp"

	// Remove any stale temp symlink
	_ = os.Remove(tempPath)

	// Create temp symlink
	if err := os.Symlink(targetPath, tempPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Atomic rename to final path
	if err := os.Rename(tempPath, symlinkPath); err != nil {
		// Clean up temp symlink on failure
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to rename symlink: %w", err)
	}

	return nil
}

func (m *Manager) SymlinkExists(workspacePath string) bool {
	symlinkPath := filepath.Join(workspacePath, constants.DocsSymlinkName)
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

