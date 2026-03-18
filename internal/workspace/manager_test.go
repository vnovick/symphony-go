package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/workspace"
)

func testManager(t *testing.T) (*workspace.Manager, string) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Workspace.Root = root
	return workspace.NewManager(cfg), root
}

func TestEnsureWorkspaceCreatesDirectory(t *testing.T) {
	mgr, root := testManager(t)
	ws, err := mgr.EnsureWorkspace("ENG-1")
	require.NoError(t, err)
	assert.True(t, ws.CreatedNow)
	assert.DirExists(t, filepath.Join(root, "ENG-1"))
}

func TestEnsureWorkspaceReusesExisting(t *testing.T) {
	mgr, root := testManager(t)
	ws1, err := mgr.EnsureWorkspace("ENG-1")
	require.NoError(t, err)
	assert.True(t, ws1.CreatedNow)

	// Write a sentinel file to verify directory is not wiped
	sentinel := filepath.Join(ws1.Path, "sentinel.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("data"), 0644))

	ws2, err := mgr.EnsureWorkspace("ENG-1")
	require.NoError(t, err)
	assert.False(t, ws2.CreatedNow)
	assert.Equal(t, ws1.Path, ws2.Path)
	assert.FileExists(t, sentinel, "existing workspace must not be wiped")
	_ = root
}

func TestEnsureWorkspaceSanitizesIdentifier(t *testing.T) {
	mgr, root := testManager(t)
	ws, err := mgr.EnsureWorkspace("ENG 1/foo")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "ENG_1_foo"), ws.Path)
}

func TestRemoveWorkspaceDeletesDirectory(t *testing.T) {
	mgr, _ := testManager(t)
	ws, err := mgr.EnsureWorkspace("ENG-2")
	require.NoError(t, err)
	require.DirExists(t, ws.Path)

	err = mgr.RemoveWorkspace("ENG-2")
	require.NoError(t, err)
	assert.NoDirExists(t, ws.Path)
}

func TestRemoveWorkspaceNonExistentIsNoOp(t *testing.T) {
	mgr, _ := testManager(t)
	err := mgr.RemoveWorkspace("nonexistent-issue")
	assert.NoError(t, err)
}
