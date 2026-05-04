package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/vnovick/itervox/internal/atomicfs"
)

// PID-file conventions
// --------------------
//
// Path: `<workflowDir>/.itervox/daemon.pid`
//
// The file holds a single line: `<pid>\t<workflowPath>\n`. The trailing
// workflowPath is a sanity-check used by `itervox stop` so we don't
// accidentally kill a daemon whose PID was recycled by an unrelated process.
//
// Scoping: each project directory (where WORKFLOW.md lives) owns its own PID
// file, so multiple itervox daemons for different repos coexist without
// colliding. `itervox stop` is therefore scoped to the current project —
// other running daemons in other repos are untouched.
//
// Lifecycle: written on daemon startup, removed on clean shutdown. A stale
// PID file (daemon crashed or was SIGKILLed) is tolerated — `itervox stop`
// verifies the PID is live with `os.FindProcess` + signal 0 and reports
// "no running daemon found" when the file points at a dead PID.

// pidFilePath returns the canonical PID-file path for a given WORKFLOW.md.
func pidFilePath(workflowPath string) (string, error) {
	abs, err := filepath.Abs(workflowPath)
	if err != nil {
		return "", fmt.Errorf("resolve workflow path: %w", err)
	}
	return filepath.Join(filepath.Dir(abs), ".itervox", "daemon.pid"), nil
}

// writePIDFile writes the current process PID (plus the absolute workflow
// path, for later verification) to the canonical location. Creates the
// `.itervox/` directory if needed. Overwrites an existing file — callers
// must decide whether to allow that; see requireNoRunningDaemon.
func writePIDFile(workflowPath string) (string, error) {
	path, err := pidFilePath(workflowPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create .itervox dir: %w", err)
	}
	abs, err := filepath.Abs(workflowPath)
	if err != nil {
		return "", err
	}
	content := fmt.Sprintf("%d\t%s\n", os.Getpid(), abs)
	if err := atomicfs.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write pid file: %w", err)
	}
	return path, nil
}

// readPIDFile parses the canonical PID file and returns the PID plus the
// workflow path that was recorded at daemon startup. Returns os.ErrNotExist
// when no file is present — caller should interpret that as "no daemon".
func readPIDFile(workflowPath string) (pid int, recordedWorkflow string, path string, err error) {
	path, perr := pidFilePath(workflowPath)
	if perr != nil {
		return 0, "", "", perr
	}
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		return 0, "", path, rerr
	}
	line := strings.TrimSpace(string(data))
	parts := strings.SplitN(line, "\t", 2)
	p, perr := strconv.Atoi(parts[0])
	if perr != nil {
		return 0, "", path, fmt.Errorf("pid file malformed (expected <pid>\\t<path>): %w", perr)
	}
	recorded := ""
	if len(parts) == 2 {
		recorded = parts[1]
	}
	return p, recorded, path, nil
}

// removePIDFile deletes the canonical PID file. Returns nil if the file was
// already absent. Logs a warning on other errors but does not return them —
// PID-file cleanup is best-effort and must not fail a graceful shutdown.
func removePIDFile(workflowPath string) {
	path, err := pidFilePath(workflowPath)
	if err != nil {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("itervox: failed to remove PID file", "path", path, "error", err)
	}
}

// processAlive reports whether a process with the given PID exists and the
// calling user can signal it. Implemented with signal 0, which is the
// canonical "is it alive" probe on POSIX systems.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 performs no delivery but returns an error if the target is
	// gone or not owned by the caller. On Windows os.FindProcess only
	// succeeds for live processes, so Signal(nil)-like semantics aren't
	// needed there — but Signal(syscall.Signal(0)) is still a no-op.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
