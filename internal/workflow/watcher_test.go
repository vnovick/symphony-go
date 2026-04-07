package workflow_test

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/workflow"
)

func TestWatcherTriggersOnChange(t *testing.T) {
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(wfPath, []byte("---\n---\nBody.\n"), 0o644))

	var called atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		_ = workflow.Watch(ctx, wfPath, func() {
			called.Add(1)
		})
	}()

	// Give the watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Trigger a change
	require.NoError(t, os.WriteFile(wfPath, []byte("---\n---\nUpdated.\n"), 0o644))

	assert.Eventually(t, func() bool {
		return called.Load() > 0
	}, 2*time.Second, 50*time.Millisecond, "onChange should be called after file write")
}
