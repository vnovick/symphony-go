// Package logbuffer provides a per-identifier ring buffer for recent log lines,
// shared between worker loggers and the terminal status UI.
package logbuffer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const maxLinesPerIssue = 500

// Buffer stores the last N log lines per issue identifier.
// When a log directory is configured via SetLogDir, lines are also appended to
// per-issue files on disk so they survive restarts and issue completion.
type Buffer struct {
	mu     sync.RWMutex
	lines  map[string][]string
	logDir string // empty = no disk persistence
}

// New creates an empty Buffer.
func New() *Buffer {
	return &Buffer{lines: make(map[string][]string)}
}

// SetLogDir configures a directory for per-issue log file persistence.
// The directory is created on first use. Calling this after Add calls is safe.
func (b *Buffer) SetLogDir(dir string) {
	b.mu.Lock()
	b.logDir = dir
	b.mu.Unlock()
}

// Add appends a line for the given identifier, dropping the oldest if over capacity.
// If a log directory is configured, the line is also written to disk.
func (b *Buffer) Add(identifier, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines[identifier] = append(b.lines[identifier], line)
	if len(b.lines[identifier]) > maxLinesPerIssue {
		b.lines[identifier] = b.lines[identifier][len(b.lines[identifier])-maxLinesPerIssue:]
	}
	if b.logDir != "" {
		b.appendToDisk(identifier, line)
	}
}

// Get returns a snapshot of recent lines for the given identifier (newest last).
// If the in-memory buffer is empty and a log directory is configured, falls back
// to reading the on-disk log file.
func (b *Buffer) Get(identifier string) []string {
	b.mu.RLock()
	mem := b.lines[identifier]
	dir := b.logDir
	b.mu.RUnlock()

	if len(mem) == 0 && dir == "" {
		return nil // unknown identifier, no disk configured
	}
	if len(mem) > 0 {
		out := make([]string, len(mem))
		copy(out, mem)
		return out
	}
	// Memory is empty and disk is configured — fall back to disk.
	return b.readFromDisk(identifier)
}

// Identifiers returns all identifiers that have log data — either in-memory or
// on disk. The returned slice is unsorted and may contain duplicates if both
// sources are present; callers should deduplicate when order matters.
func (b *Buffer) Identifiers() []string {
	b.mu.RLock()
	dir := b.logDir
	ids := make([]string, 0, len(b.lines))
	seen := make(map[string]struct{}, len(b.lines))
	for id := range b.lines {
		ids = append(ids, id)
		seen[id] = struct{}{}
	}
	b.mu.RUnlock()

	if dir == "" {
		return ids
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ids
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".log")
		// reverse the filename sanitisation applied in issuePath
		id = strings.NewReplacer("_", ":").Replace(id)
		if _, exists := seen[id]; !exists {
			ids = append(ids, id)
			seen[id] = struct{}{}
		}
	}
	return ids
}

// Remove deletes the in-memory buffer for the given identifier.
// The on-disk log file is intentionally preserved so logs remain viewable after
// an issue completes.
func (b *Buffer) Remove(identifier string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.lines, identifier)
}

// ClearAll deletes all in-memory buffers and all on-disk log files in logDir.
func (b *Buffer) ClearAll() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = make(map[string][]string)
	if b.logDir == "" {
		return nil
	}
	entries, err := os.ReadDir(b.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("logbuffer: read dir %s: %w", b.logDir, err)
	}
	var first error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		if err := os.Remove(filepath.Join(b.logDir, e.Name())); err != nil && !os.IsNotExist(err) {
			if first == nil {
				first = err
			}
		}
	}
	return first
}

// Clear deletes both the in-memory buffer and the on-disk log file for identifier.
func (b *Buffer) Clear(identifier string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.lines, identifier)
	if b.logDir != "" {
		p := b.issuePath(identifier)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// --- internal helpers (must be called with mu held for write, or unlocked for read) ---

func (b *Buffer) issuePath(identifier string) string {
	// Sanitise the identifier so it is safe as a filename.
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(identifier)
	return filepath.Join(b.logDir, safe+".log")
}

func (b *Buffer) appendToDisk(identifier, line string) {
	p := b.issuePath(identifier)
	if err := os.MkdirAll(b.logDir, 0o755); err != nil {
		slog.Warn("logbuffer: failed to create log dir", "dir", b.logDir, "error", err)
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("logbuffer: failed to open log file", "path", p, "error", err)
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		slog.Warn("logbuffer: failed to write log line", "path", p, "error", err)
	}
}

func (b *Buffer) readFromDisk(identifier string) []string {
	p := b.issuePath(identifier)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > maxLinesPerIssue {
		lines = lines[len(lines)-maxLinesPerIssue:]
	}
	return lines
}
