package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vnovick/symphony-go/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeKeyAlphanumericPassthrough(t *testing.T) {
	assert.Equal(t, "ENG-1", workspace.SanitizeKey("ENG-1"))
}

func TestSanitizeKeyReplacesSpecialChars(t *testing.T) {
	assert.Equal(t, "hello_world_foo", workspace.SanitizeKey("hello world/foo"))
}

func TestSanitizeKeyAllowsDotUnderscoreDash(t *testing.T) {
	assert.Equal(t, "a.b_c-d", workspace.SanitizeKey("a.b_c-d"))
}

func TestSanitizeKeyReplacesSlash(t *testing.T) {
	assert.Equal(t, "owner_repo", workspace.SanitizeKey("owner/repo"))
}

func TestWorkspacePath(t *testing.T) {
	got := workspace.WorkspacePath("/tmp/root", "ENG-1")
	assert.Equal(t, "/tmp/root/ENG-1", got)
}

func TestWorkspacePathSanitizesKey(t *testing.T) {
	got := workspace.WorkspacePath("/tmp/root", "ENG 1/foo")
	assert.Equal(t, "/tmp/root/ENG_1_foo", got)
}

func TestAssertContainedHappyPath(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.MkdirAll(child, 0755))
	err := workspace.AssertContained(root, child)
	assert.NoError(t, err)
}

func TestAssertContainedRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	err := workspace.AssertContained(root, other)
	assert.Error(t, err)
}

func TestAssertContainedRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	require.NoError(t, os.Symlink(outside, link))
	err := workspace.AssertContained(root, link)
	assert.Error(t, err)
}

func TestAssertContainedRejectsRootItself(t *testing.T) {
	root := t.TempDir()
	err := workspace.AssertContained(root, root)
	assert.Error(t, err)
}
