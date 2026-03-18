package workspace

import (
	"os"

	"github.com/vnovick/symphony-go/internal/config"
)

// Workspace represents a resolved per-issue workspace directory.
type Workspace struct {
	Path       string
	Identifier string
	CreatedNow bool
}

// Manager handles creation, reuse, and removal of per-issue workspace directories.
type Manager struct {
	cfg *config.Config
}

// NewManager constructs a Manager using the given config.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// EnsureWorkspace creates the workspace directory for identifier if it does not
// exist, or reuses it if it does. Returns a Workspace with CreatedNow=true only
// when the directory was newly created. The resolved path is validated with
// filepath.EvalSymlinks to reject symlink escapes outside workspace.root.
func (m *Manager) EnsureWorkspace(identifier string) (Workspace, error) {
	root := m.cfg.Workspace.Root
	path := WorkspacePath(root, identifier)

	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			// A non-directory exists at the path — remove it and create fresh.
			if err := os.Remove(path); err != nil {
				return Workspace{}, err
			}
		}
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return Workspace{}, err
	}

	if err := AssertContained(root, path); err != nil {
		_ = os.RemoveAll(path)
		return Workspace{}, err
	}

	createdNow := info == nil || !info.IsDir()
	return Workspace{Path: path, Identifier: identifier, CreatedNow: createdNow}, nil
}

// RemoveWorkspace deletes the workspace directory for identifier.
// Safe to call when the directory does not exist.
func (m *Manager) RemoveWorkspace(identifier string) error {
	path := WorkspacePath(m.cfg.Workspace.Root, identifier)
	return os.RemoveAll(path)
}
