package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomicallyReplacesExistingFileWithoutLeavingTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input_required.json")

	require.NoError(t, os.WriteFile(path, []byte(`{"old":true}`), 0o644))

	require.NoError(t, writeFileAtomically(path, []byte(`{"new":true}`), 0o644))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"new":true}`, string(data))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "input_required.json", entries[0].Name())
}
