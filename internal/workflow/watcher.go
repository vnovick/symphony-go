package workflow

import (
	"context"
	"crypto/sha256"
	"io"
	"log/slog"
	"os"
	"time"
)

const pollInterval = 1 * time.Second

// fileStamp captures the identity of a file at a point in time.
type fileStamp struct {
	mtime time.Time
	size  int64
	hash  [32]byte // sha256
}

// stampOf returns a fileStamp for the file at path.
// prev is the previously known stamp; if mtime and size match prev,
// the sha256 is not recomputed and prev is returned unchanged (fast path).
func stampOf(path string, prev fileStamp) (fileStamp, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}

	mtime := fi.ModTime()
	size := fi.Size()

	// Fast path: mtime and size match — assume content is identical.
	if mtime.Equal(prev.mtime) && size == prev.size {
		return prev, nil
	}

	// Compute sha256 of file content.
	f, err := os.Open(path)
	if err != nil {
		return fileStamp{}, err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fileStamp{}, err
	}

	var digest [32]byte
	copy(digest[:], h.Sum(nil))

	return fileStamp{mtime: mtime, size: size, hash: digest}, nil
}

// Watch monitors path for changes by polling every second with a content-hash
// stamp ({mtime, size, sha256}). onChange is called only when the stamp changes,
// preventing spurious reloads when an editor writes identical content.
// Blocks until ctx is cancelled. Returns any setup or context error.
func Watch(ctx context.Context, path string, onChange func()) error {
	// Capture the initial stamp so we don't fire on startup.
	current, err := stampOf(path, fileStamp{})
	if err != nil {
		slog.Warn("workflow watcher: initial stat failed", "path", path, "error", err)
		// Don't abort — the file may appear shortly; proceed with zero stamp.
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			next, err := stampOf(path, current)
			if err != nil {
				slog.Warn("workflow watcher: stat error", "path", path, "error", err)
				continue
			}

			if next == current {
				continue
			}

			slog.Debug("workflow watcher: file changed", "path", path,
				"old_mtime", current.mtime, "new_mtime", next.mtime)
			current = next
			onChange()
		}
	}
}
