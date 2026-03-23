package logbuffer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/logbuffer"
)

func TestAddPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	buf := logbuffer.New()
	buf.SetLogDir(dir)
	buf.Add("ENG-1", "hello world")

	data, err := os.ReadFile(filepath.Join(dir, "ENG-1.log"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "hello world")
}

func TestAddToReadOnlyDirDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	// Block MkdirAll by placing a regular file at the intended log-dir path.
	badDir := filepath.Join(dir, "logs")
	require.NoError(t, os.WriteFile(badDir, []byte("block"), 0o444))

	buf := logbuffer.New()
	buf.SetLogDir(badDir) // this path is a file, not a dir — MkdirAll will fail
	// Must not panic.
	buf.Add("ENG-1", "should not panic")
}

func TestGetReturnsAddedLines(t *testing.T) {
	buf := logbuffer.New()
	buf.SetLogDir(t.TempDir())

	lines := []string{"alpha", "beta", "gamma"}
	for _, l := range lines {
		buf.Add("ENG-2", l)
	}

	got := buf.Get("ENG-2")
	require.Len(t, got, 3)
	assert.Equal(t, lines, got)
}

func TestGetUnknownIssueReturnsNil(t *testing.T) {
	buf := logbuffer.New()
	assert.Nil(t, buf.Get("NONEXISTENT"))
}

func TestMultipleIssuesAreIsolated(t *testing.T) {
	buf := logbuffer.New()
	buf.SetLogDir(t.TempDir())
	buf.Add("A-1", "line for A")
	buf.Add("B-1", "line for B")

	assert.Equal(t, []string{"line for A"}, buf.Get("A-1"))
	assert.Equal(t, []string{"line for B"}, buf.Get("B-1"))
}

func TestRemoveClearsMemoryButPreservesDisk(t *testing.T) {
	dir := t.TempDir()
	buf := logbuffer.New()
	buf.SetLogDir(dir)
	buf.Add("ENG-3", "persist me")
	buf.Remove("ENG-3")

	// In-memory gone but logDir is set, so Get falls back to disk (by design).
	assert.Contains(t, buf.Get("ENG-3"), "persist me")

	// Disk file still present after a simulated restart (new buffer, same dir).
	buf2 := logbuffer.New()
	buf2.SetLogDir(dir)
	got := buf2.Get("ENG-3")
	assert.Contains(t, got, "persist me")
}

func TestNoDiskPersistenceWhenLogDirNotSet(t *testing.T) {
	buf := logbuffer.New()
	buf.Add("ENG-4", "in memory only")
	assert.Equal(t, []string{"in memory only"}, buf.Get("ENG-4"))
}

func TestClearDeletesMemoryAndDisk(t *testing.T) {
	dir := t.TempDir()
	buf := logbuffer.New()
	buf.SetLogDir(dir)
	buf.Add("ENG-5", "to be cleared")

	// Verify disk file exists.
	diskPath := filepath.Join(dir, "ENG-5.log")
	_, err := os.Stat(diskPath)
	require.NoError(t, err)

	err = buf.Clear("ENG-5")
	require.NoError(t, err)

	// Memory gone.
	assert.Nil(t, buf.Get("ENG-5"))

	// Disk file gone.
	_, err = os.Stat(diskPath)
	assert.True(t, os.IsNotExist(err), "disk file should be deleted after Clear")
}

func TestClearNonExistentIsOK(t *testing.T) {
	dir := t.TempDir()
	buf := logbuffer.New()
	buf.SetLogDir(dir)
	// Clearing an identifier that was never added should not error.
	err := buf.Clear("NEVER-ADDED")
	require.NoError(t, err)
}

func TestClearWithoutLogDirOnlyClearsMemory(t *testing.T) {
	buf := logbuffer.New() // no SetLogDir
	buf.Add("ENG-6", "in memory only")
	err := buf.Clear("ENG-6")
	require.NoError(t, err)
	assert.Nil(t, buf.Get("ENG-6"))
}

func BenchmarkLogBuffer_Add(b *testing.B) {
	buf := logbuffer.New()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Add("ENG-1", "benchmark log line")
	}
}

func BenchmarkLogBuffer_AddMultipleIssues(b *testing.B) {
	buf := logbuffer.New()
	ids := []string{"ENG-1", "ENG-2", "ENG-3", "ENG-4", "ENG-5"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Add(ids[i%len(ids)], "benchmark log line")
	}
}
