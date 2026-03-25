package orchestrator

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
)

// Snapshot returns a consistent copy of the current orchestrator state.
// Safe to call from any goroutine.
//
// issueProfiles are stored in o.issueProfiles (written by SetIssueProfile from
// any goroutine) rather than in the event-loop State, so they are not
// automatically included in lastSnap. We overlay them here so callers — in
// particular fetchIssues in main.go — see the live assignments without waiting
// for the next event-loop tick to rebuild the snapshot.
func (o *Orchestrator) Snapshot() State {
	o.snapMu.RLock()
	snap := o.lastSnap
	o.snapMu.RUnlock()

	o.issueProfilesMu.RLock()
	if len(o.issueProfiles) > 0 {
		merged := make(map[string]string, len(snap.IssueProfiles)+len(o.issueProfiles))
		maps.Copy(merged, snap.IssueProfiles)
		for k, v := range o.issueProfiles {
			if v == "" {
				delete(merged, k)
			} else {
				merged[k] = v
			}
		}
		snap.IssueProfiles = merged
	}
	o.issueProfilesMu.RUnlock()

	return snap
}

const maxHistory = 200

// SetHistoryFile sets the path for persisting completed runs across restarts.
// Must be called before Run; calling after Run starts is a no-op with a logged error.
// If path is empty, disk persistence is disabled.
func (o *Orchestrator) SetHistoryFile(path string) {
	if o.started.Load() {
		slog.Error("orchestrator: SetHistoryFile called after Run started; ignoring", "path", path)
		return
	}
	o.historyMu.Lock()
	o.historyFile = path
	o.historyMu.Unlock()
}

// SetHistoryKey sets the project-scoping key used to tag and filter history entries.
// Format: "<tracker-kind>:<project-slug>" (e.g. "github:org/repo").
// Entries written with a different (non-empty) key are skipped on load.
// Must be called before Run; calling after Run starts is a no-op with a logged error.
func (o *Orchestrator) SetHistoryKey(key string) {
	if o.started.Load() {
		slog.Error("orchestrator: SetHistoryKey called after Run started; ignoring", "key", key)
		return
	}
	o.historyMu.Lock()
	o.historyKey = key
	o.historyMu.Unlock()
}

// loadHistoryFromDisk reads the history file (if set) and populates completedRuns.
// Called once at startup before the event loop begins.
func (o *Orchestrator) loadHistoryFromDisk() {
	o.historyMu.Lock()
	defer o.historyMu.Unlock()
	if o.historyFile == "" {
		return
	}
	data, err := os.ReadFile(o.historyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("orchestrator: failed to load history file", "path", o.historyFile, "error", err)
		}
		return
	}
	var runs []CompletedRun
	if err := json.Unmarshal(data, &runs); err != nil {
		slog.Warn("orchestrator: failed to parse history file", "path", o.historyFile, "error", err)
		return
	}
	// Filter to only this project's runs. Legacy entries (empty ProjectKey) are
	// kept so that history written before scoping was added is not dropped.
	if o.historyKey != "" {
		filtered := runs[:0]
		for _, r := range runs {
			if r.ProjectKey == "" || r.ProjectKey == o.historyKey {
				filtered = append(filtered, r)
			}
		}
		runs = filtered
	}
	o.completedRuns = runs
	slog.Info("orchestrator: loaded history", "path", o.historyFile, "entries", len(runs))
}

// addCompletedRun appends a finished run to the in-memory history ring buffer
// and persists the ring buffer to disk when a history file is configured.
//
// INVARIANT: must only be called from the single event-loop goroutine (onTick
// and its callees). The event loop is the sole writer of completedRuns; the
// historyMu lock exists only to synchronise concurrent readers such as the SSE
// and REST handlers. historyMu is released before the disk write so those
// readers are never blocked by I/O.
func (o *Orchestrator) addCompletedRun(run CompletedRun) {
	o.historyMu.Lock()
	o.completedRuns = append(o.completedRuns, run)
	if len(o.completedRuns) > maxHistory {
		o.completedRuns = o.completedRuns[len(o.completedRuns)-maxHistory:]
	}
	// Snapshot the slice and the path while holding the lock, then release
	// before performing disk I/O so concurrent readers are not blocked.
	path := o.historyFile
	snapshot := make([]CompletedRun, len(o.completedRuns))
	copy(snapshot, o.completedRuns)
	o.historyMu.Unlock()

	if path != "" {
		data, err := json.Marshal(snapshot)
		if err == nil {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				slog.Warn("orchestrator: failed to write history file", "path", path, "error", err)
			}
		}
	}
}

// SetPausedFile sets the path for persisting PausedIdentifiers across restarts.
// Must be called before Run.
func (o *Orchestrator) SetPausedFile(path string) {
	o.pausedMu.Lock()
	o.pausedFile = path
	o.pausedMu.Unlock()
}

// loadPausedFromDisk reads the paused file and pre-populates state.PausedIdentifiers.
// Called once at startup. state is the freshly-initialised event-loop State.
// Supports both the new format (map[identifier]issueID) and the legacy format
// ([]string of identifiers), storing an empty UUID for legacy entries.
func (o *Orchestrator) loadPausedFromDisk(state State) State {
	o.pausedMu.RLock()
	path := o.pausedFile
	o.pausedMu.RUnlock()
	if path == "" {
		return state
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("orchestrator: failed to load paused file", "path", path, "error", err)
		}
		return state
	}
	// Try new format: {"identifier": "issueUUID", ...}
	var newFmt map[string]string
	if err := json.Unmarshal(data, &newFmt); err == nil {
		maps.Copy(state.PausedIdentifiers, newFmt)
		// Pre-populate PrevActiveIdentifiers so the first-tick auto-resume guard
		// treats these as "was already active before daemon start" and does not
		// clear the pause. Without this, the empty PrevActiveIdentifiers on startup
		// causes every disk-persisted pause to be auto-resumed on the first tick —
		// this happens whenever WORKFLOW.md is written (e.g. BumpWorkers), which
		// triggers the file watcher and restarts the orchestrator.
		for id := range newFmt {
			state.PrevActiveIdentifiers[id] = struct{}{}
		}
		slog.Info("orchestrator: loaded paused identifiers", "path", path, "count", len(newFmt))
		return state
	}
	// Fallback: legacy format ["identifier1", "identifier2"]
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		slog.Warn("orchestrator: failed to parse paused file", "path", path, "error", err)
		return state
	}
	for _, id := range ids {
		state.PausedIdentifiers[id] = "" // UUID unknown from legacy format — Discard won't auto-move to Backlog
		state.PrevActiveIdentifiers[id] = struct{}{}
	}
	slog.Info("orchestrator: loaded paused identifiers (legacy format)", "path", path, "count", len(ids))
	return state
}

// copyStringMap returns a copy of a map[string]string.
func copyStringMap(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	maps.Copy(cp, m)
	return cp
}

// copyStructMap returns a copy of a map[string]struct{}.
func copyStructMap(m map[string]struct{}) map[string]struct{} {
	cp := make(map[string]struct{}, len(m))
	for k := range m {
		cp[k] = struct{}{}
	}
	return cp
}

// copyRunningMap returns a deep copy of a map[string]*RunEntry.
// Each RunEntry value is copied by value so that external goroutines reading
// the snapshot cannot observe in-progress mutations by the event loop
// (TurnCount, TotalTokens, LastMessage, etc.). WorkerCancel is intentionally
// omitted from the copy — snapshot readers must never cancel a live worker.
func copyRunningMap(m map[string]*RunEntry) map[string]*RunEntry {
	cp := make(map[string]*RunEntry, len(m))
	for k, v := range m {
		if v == nil {
			cp[k] = nil
			continue
		}
		e := *v           // copy struct value
		e.WorkerCancel = nil // not safe to share across goroutines
		cp[k] = &e
	}
	return cp
}

// copyRetryMap returns a shallow copy of a map[string]*RetryEntry.
func copyRetryMap(m map[string]*RetryEntry) map[string]*RetryEntry {
	cp := make(map[string]*RetryEntry, len(m))
	maps.Copy(cp, m)
	return cp
}

// savePausedToDisk writes PausedIdentifiers to disk in the new map format
// {"identifier": "issueUUID"}. Must NOT be called with snapMu held.
func (o *Orchestrator) savePausedToDisk(paused map[string]string) {
	o.pausedMu.RLock()
	path := o.pausedFile
	o.pausedMu.RUnlock()
	if path == "" {
		return
	}
	data, err := json.Marshal(paused)
	if err == nil {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			slog.Warn("orchestrator: failed to write paused file", "path", path, "error", err)
		}
	}
}

// RunHistory returns a snapshot of recently completed runs (newest last).
func (o *Orchestrator) RunHistory() []CompletedRun {
	o.historyMu.RLock()
	defer o.historyMu.RUnlock()
	result := make([]CompletedRun, len(o.completedRuns))
	copy(result, o.completedRuns)
	return result
}

func (o *Orchestrator) storeSnap(s State) {
	// Deep-copy every map field so that lastSnap contains independent copies.
	// The event loop mutates state.* maps without holding snapMu (they are its
	// private data). External goroutines read lastSnap.* under snapMu. Sharing
	// the same underlying maps would be a data race; separate copies prevent it.
	snap := s
	snap.Running = copyRunningMap(s.Running)
	snap.Claimed = copyStructMap(s.Claimed)
	snap.RetryAttempts = copyRetryMap(s.RetryAttempts)
	snap.PausedIdentifiers = copyStringMap(s.PausedIdentifiers)
	snap.IssueProfiles = copyStringMap(s.IssueProfiles)
	snap.PausedOpenPRs = copyStringMap(s.PausedOpenPRs)
	snap.ForceReanalyze = copyStructMap(s.ForceReanalyze)
	snap.PrevActiveIdentifiers = copyStructMap(s.PrevActiveIdentifiers)
	snap.DiscardingIdentifiers = copyStructMap(s.DiscardingIdentifiers)

	o.snapMu.Lock()
	o.lastSnap = snap
	o.snapMu.Unlock()

	o.savePausedToDisk(snap.PausedIdentifiers)
	if o.OnStateChange != nil {
		o.OnStateChange()
	}
}
