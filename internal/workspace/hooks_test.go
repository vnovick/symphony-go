package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/workspace"
)

func TestRunHookSuccess(t *testing.T) {
	dir := t.TempDir()
	err := workspace.RunHook(context.Background(), "echo hello", dir, 5000)
	assert.NoError(t, err)
}

func TestRunHookEmptyScriptIsNoOp(t *testing.T) {
	dir := t.TempDir()
	err := workspace.RunHook(context.Background(), "", dir, 5000)
	assert.NoError(t, err)
}

func TestRunHookNonZeroExitFails(t *testing.T) {
	dir := t.TempDir()
	err := workspace.RunHook(context.Background(), "exit 1", dir, 5000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook_failed")
}

func TestRunHookTimeoutKillsProcess(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	err := workspace.RunHook(context.Background(), "sleep 60", dir, 200)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook_timeout")
	assert.Less(t, elapsed, 5*time.Second, "hook should be killed well before 5s")
}

func TestRunHookNonPositiveTimeoutFallsBackTo60s(t *testing.T) {
	dir := t.TempDir()
	// A quick command with timeout=0 should succeed (falls back to 60000ms).
	err := workspace.RunHook(context.Background(), "echo ok", dir, 0)
	assert.NoError(t, err)
}

func TestRunHookWritesFileInWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "hook-ran")
	script := "touch hook-ran"
	err := workspace.RunHook(context.Background(), script, dir, 5000)
	require.NoError(t, err)
	_, statErr := os.Stat(sentinel)
	assert.NoError(t, statErr, "hook should have created file in workspace dir")
}
