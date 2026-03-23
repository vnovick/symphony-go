package workspace

import (
	"context"
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

// EnsureWorkspace creates or reuses the workspace for the given identifier.
// When cfg.Workspace.Worktree is true, a git worktree is used (branchName is
// the desired branch). Otherwise the legacy directory-based path is used and
// both ctx and branchName are ignored.
func (m *Manager) EnsureWorkspace(ctx context.Context, identifier, branchName string) (Workspace, error) {
	if m.cfg.Workspace.Worktree {
		return m.ensureWorktree(ctx, identifier, branchName)
	}
	return m.ensureDirectory(identifier)
}

// ensureDirectory is the legacy implementation of EnsureWorkspace: it creates
// or reuses a plain directory under workspace.root.
func (m *Manager) ensureDirectory(identifier string) (Workspace, error) {
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

	if err := os.MkdirAll(path, 0o755); err != nil {
		return Workspace{}, err
	}

	if err := AssertContained(root, path); err != nil {
		_ = os.RemoveAll(path)
		return Workspace{}, err
	}

	createdNow := info == nil || !info.IsDir()
	return Workspace{Path: path, Identifier: identifier, CreatedNow: createdNow}, nil
}

// RemoveWorkspace deletes the workspace for identifier.
// When cfg.Workspace.Worktree is true, the git worktree is removed (branchName
// is required). Otherwise the legacy directory is removed and branchName is
// ignored. Safe to call when the workspace does not exist.
func (m *Manager) RemoveWorkspace(identifier, branchName string) error {
	if m.cfg.Workspace.Worktree {
		return m.removeWorktree(identifier, branchName)
	}
	path := WorkspacePath(m.cfg.Workspace.Root, identifier)
	return os.RemoveAll(path)
}
