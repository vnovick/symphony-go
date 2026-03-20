package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vnovick/symphony-go/internal/agent"
	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/domain"
	"github.com/vnovick/symphony-go/internal/logbuffer"
	"github.com/vnovick/symphony-go/internal/prompt"
	"github.com/vnovick/symphony-go/internal/tracker"
	"github.com/vnovick/symphony-go/internal/workspace"
)

// Orchestrator is the single-goroutine state machine that owns all dispatch state.
type Orchestrator struct {
	// DryRun disables actual agent execution: issues are claimed but no worker
	// subprocess is started. Set SYMPHONY_DRY_RUN=1 or assign before calling Run.
	DryRun bool

	cfg       *config.Config
	tracker   tracker.Tracker
	runners   *agent.RunnerRegistry
	workspace *workspace.Manager // nil is safe — workspace ops skipped (useful in tests)
	logBuf    *logbuffer.Buffer  // nil is safe — log buffering disabled
	events    chan OrchestratorEvent
	refresh   chan struct{} // signals an immediate re-poll (e.g. from the web dashboard)
	// OnDispatch is an optional hook called (in-goroutine) when an issue is dispatched.
	OnDispatch func(issueID string)
	// OnStateChange is called after every state snapshot update (see storeSnap).
	// Both fields must be set before calling Run — Run spawns goroutines that
	// read them, so the Go memory model's happens-before guarantee (set before
	// goroutine start) ensures visibility without any additional synchronisation.
	OnStateChange func()

	snapMu     sync.RWMutex
	lastSnap   State
	sshHostIdx int // round-robin index for SSH host selection; only accessed in event loop

	// workersMu guards SetMaxWorkers/MaxWorkers, which may be called from any goroutine.
	workersMu sync.Mutex

	// cfgMu guards the cfg fields mutated at runtime from HTTP handler goroutines:
	// cfg.Agent.AgentMode, cfg.Agent.Profiles, cfg.Tracker.ActiveStates,
	// cfg.Tracker.TerminalStates, cfg.Tracker.CompletionState.
	// All other cfg fields are read-only after startup and need no lock.
	cfgMu sync.RWMutex

	// historyMu guards completedRuns, which is written by the event loop and read by RunHistory.
	historyMu     sync.RWMutex
	completedRuns []CompletedRun
	historyFile   string // optional path for persisting completedRuns to disk
	pausedFile    string // optional path for persisting PausedIdentifiers across restarts

	// userCancelledMu guards userCancelledIDs, which is written by CancelIssue
	// (any goroutine) and read by handleEvent (event loop goroutine).
	userCancelledMu  sync.Mutex
	userCancelledIDs map[string]struct{} // keyed by identifier (e.g. "TIPRD-25")

	// userTerminatedMu guards userTerminatedIDs, which is written by TerminateIssue
	// (any goroutine) and read by handleEvent (event loop goroutine).
	userTerminatedMu  sync.Mutex
	userTerminatedIDs map[string]struct{} // like userCancelledIDs but releases claim without pausing

	// issueProfilesMu guards issueProfiles, which is written by SetIssueProfile
	// (any goroutine) and read by dispatch (event loop goroutine).
	issueProfilesMu sync.Mutex
	issueProfiles   map[string]string // identifier → profile name

	// prURLsMu guards prURLsBeforePause, which is written by runWorker goroutines
	// and read by the event loop when handling TerminalSkippedOpenPR events.
	prURLsMu          sync.Mutex
	prURLsBeforePause map[string]string // identifier → PR URL, ephemeral

	// runCtx is the context passed to Run. Stored atomically so DispatchReviewer
	// can read it safely from any goroutine without a mutex.
	runCtx atomic.Pointer[context.Context]
}

// New constructs an Orchestrator ready to Run. wm may be nil (workspace ops skipped).
// runners is the multi-agent registry; if nil, a default registry (claude-code only) is created.
func New(cfg *config.Config, tr tracker.Tracker, runners *agent.RunnerRegistry, wm *workspace.Manager) *Orchestrator {
	if runners == nil {
		runners = agent.NewRunnerRegistry(cfg.Agent.Runner)
	}
	return &Orchestrator{
		cfg:               cfg,
		tracker:           tr,
		runners:           runners,
		workspace:         wm,
		events:            make(chan OrchestratorEvent, 64),
		refresh:           make(chan struct{}, 1),
		userCancelledIDs:  make(map[string]struct{}),
		userTerminatedIDs: make(map[string]struct{}),
		issueProfiles:     make(map[string]string),
		prURLsBeforePause: make(map[string]string),
	}
}

// Refresh triggers an immediate re-poll on the next select iteration.
// Safe to call from any goroutine; non-blocking (drops the signal if one is already pending).
func (o *Orchestrator) Refresh() {
	select {
	case o.refresh <- struct{}{}:
	default:
	}
}

// SetMaxWorkers updates the maximum number of concurrent agents at runtime.
// The value is clamped to [1, 50]. Safe to call from any goroutine.
func (o *Orchestrator) SetMaxWorkers(n int) {
	if n < 1 {
		n = 1
	}
	if n > 50 {
		n = 50
	}
	o.workersMu.Lock()
	o.cfg.Agent.MaxConcurrentAgents = n
	o.workersMu.Unlock()
	slog.Info("orchestrator: max workers updated", "max_concurrent_agents", n)
	if o.OnStateChange != nil {
		o.OnStateChange()
	}
}

// MaxWorkers returns the current maximum concurrent agents setting.
// Safe to call from any goroutine.
func (o *Orchestrator) MaxWorkers() int {
	o.workersMu.Lock()
	defer o.workersMu.Unlock()
	return o.cfg.Agent.MaxConcurrentAgents
}

// AgentModeCfg returns cfg.Agent.AgentMode under cfgMu.
// Safe to call from any goroutine.
func (o *Orchestrator) AgentModeCfg() string {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	return o.cfg.Agent.AgentMode
}

// SetAgentModeCfg sets cfg.Agent.AgentMode under cfgMu.
// Safe to call from any goroutine.
func (o *Orchestrator) SetAgentModeCfg(mode string) {
	o.cfgMu.Lock()
	o.cfg.Agent.AgentMode = mode
	o.cfgMu.Unlock()
}

// ProfilesCfg returns a shallow copy of cfg.Agent.Profiles under cfgMu.
// Safe to call from any goroutine.
func (o *Orchestrator) ProfilesCfg() map[string]config.AgentProfile {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	cp := make(map[string]config.AgentProfile, len(o.cfg.Agent.Profiles))
	for k, v := range o.cfg.Agent.Profiles {
		cp[k] = v
	}
	return cp
}

// SetProfilesCfg atomically replaces cfg.Agent.Profiles under cfgMu.
// Safe to call from any goroutine.
func (o *Orchestrator) SetProfilesCfg(p map[string]config.AgentProfile) {
	o.cfgMu.Lock()
	o.cfg.Agent.Profiles = p
	o.cfgMu.Unlock()
}

// TrackerStatesCfg returns copies of cfg.Tracker.ActiveStates, TerminalStates,
// and CompletionState under cfgMu.
// Safe to call from any goroutine.
func (o *Orchestrator) TrackerStatesCfg() (active, terminal []string, completion string) {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	return append([]string{}, o.cfg.Tracker.ActiveStates...),
		append([]string{}, o.cfg.Tracker.TerminalStates...),
		o.cfg.Tracker.CompletionState
}

// SetTrackerStatesCfg atomically updates ActiveStates, TerminalStates, and
// CompletionState under cfgMu.
// Safe to call from any goroutine.
func (o *Orchestrator) SetTrackerStatesCfg(active, terminal []string, completion string) {
	o.cfgMu.Lock()
	o.cfg.Tracker.ActiveStates = active
	o.cfg.Tracker.TerminalStates = terminal
	o.cfg.Tracker.CompletionState = completion
	o.cfgMu.Unlock()
}

// SetLogBuffer attaches a log buffer so worker output is captured per-identifier
// for display in the interactive TUI.
func (o *Orchestrator) SetLogBuffer(buf *logbuffer.Buffer) {
	o.logBuf = buf
}

// bufLogger wraps a slog.Logger and also writes INFO/WARN lines to a LogBuffer.
type bufLogger struct {
	base       *slog.Logger
	buf        *logbuffer.Buffer
	identifier string
}

// Info logs at INFO level and writes the message to the log buffer.
func (l *bufLogger) Info(msg string, args ...any) {
	l.base.Info(msg, args...)
	if l.buf != nil {
		l.buf.Add(l.identifier, formatBufLine("INFO", msg, args))
	}
}

// Debug logs at DEBUG level (not written to log buffer).
func (l *bufLogger) Debug(msg string, args ...any) {
	l.base.Debug(msg, args...)
}

// Warn logs at WARN level and writes the message to the log buffer.
func (l *bufLogger) Warn(msg string, args ...any) {
	l.base.Warn(msg, args...)
	if l.buf != nil {
		l.buf.Add(l.identifier, formatBufLine("WARN", msg, args))
	}
}

func formatBufLine(level, msg string, args []any) string {
	var sb strings.Builder
	sb.WriteString(level)
	sb.WriteString(" ")
	sb.WriteString(msg)
	for i := 0; i+1 < len(args); i += 2 {
		sb.WriteString(" ")
		fmt.Fprintf(&sb, "%v", args[i])
		sb.WriteString("=")
		// Quote string values that contain spaces so parsers can extract them correctly.
		v := fmt.Sprintf("%v", args[i+1])
		if strings.ContainsAny(v, " \t\n\"") {
			fmt.Fprintf(&sb, "%q", v)
		} else {
			sb.WriteString(v)
		}
	}
	// Append wall-clock timestamp so the web/TUI can display HH:MM:SS per entry.
	sb.WriteString(" time=")
	sb.WriteString(time.Now().Format("15:04:05"))
	return sb.String()
}

// makeBufLine builds a timestamped log buffer line for direct (non-slog) entries.
func makeBufLine(level, msg string) string {
	return fmt.Sprintf("%s %s time=%s", level, msg, time.Now().Format("15:04:05"))
}

// formatSessionComment builds a Markdown comment summarising the full agent session.
// allText is every assistant text block emitted across all turns.
// Returns empty string if there is nothing worth posting.
func formatSessionComment(allText []string, identifier string) string {
	var nonEmpty []string
	for _, t := range allText {
		if strings.TrimSpace(t) != "" {
			nonEmpty = append(nonEmpty, t)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Symphony Agent Session — ")
	sb.WriteString(identifier)
	sb.WriteString("\n\n")
	for _, t := range nonEmpty {
		sb.WriteString(t)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// CancelIssue cancels the running worker for the given issue identifier and
// marks it as paused so it is not automatically retried. The issue stays paused
// until ResumeIssue is called.
// Returns true if a running worker was found and cancelled, false otherwise.
// Safe to call from any goroutine.
func (o *Orchestrator) CancelIssue(identifier string) bool {
	// Mark as user-cancelled BEFORE cancelling the worker so the exit handler
	// in the event loop sees the flag when the EventWorkerExited arrives.
	o.userCancelledMu.Lock()
	o.userCancelledIDs[identifier] = struct{}{}
	o.userCancelledMu.Unlock()

	return o.cancelRunningWorker(identifier, func() {
		// Worker wasn't running — clear the marker so it doesn't accidentally pause.
		o.userCancelledMu.Lock()
		delete(o.userCancelledIDs, identifier)
		o.userCancelledMu.Unlock()
	})
}

// ResumeIssue removes a paused issue from the pause set, allowing it to be
// dispatched again on the next tick.
// Returns true if the issue was paused and is now resumed, false if not found.
// Safe to call from any goroutine.
func (o *Orchestrator) ResumeIssue(identifier string) bool {
	o.snapMu.RLock()
	_, isPaused := o.lastSnap.PausedIdentifiers[identifier]
	o.snapMu.RUnlock()
	if !isPaused {
		return false
	}
	// Route state mutation through the event loop so the change is applied to
	// state.PausedIdentifiers (the event loop's source of truth), not just to
	// the lastSnap copy — which would be overwritten on the next storeSnap.
	select {
	case o.events <- OrchestratorEvent{Type: EventResumeIssue, Identifier: identifier}:
	default:
		return false // channel full; caller can retry
	}
	slog.Info("orchestrator: issue resume queued", "identifier", identifier)
	return true
}

// TerminateIssue hard-stops an issue without adding it to PausedIdentifiers:
//   - If a worker is running, it is cancelled and the claim is released
//     (the issue will be re-dispatched on the next poll cycle).
//   - If the issue is paused, it is removed from PausedIdentifiers so it can
//     be re-dispatched without a manual resume.
//
// Returns true if any action was taken (worker cancelled or paused removed).
// Safe to call from any goroutine.
func (o *Orchestrator) TerminateIssue(identifier string) bool {
	// Check if the issue is currently paused (read-only under RLock).
	o.snapMu.RLock()
	issueID, isPaused := o.lastSnap.PausedIdentifiers[identifier]
	o.snapMu.RUnlock()
	if isPaused {
		// Route state mutation through the event loop (same reason as ResumeIssue).
		select {
		case o.events <- OrchestratorEvent{Type: EventTerminatePaused, Identifier: identifier, IssueID: issueID}:
		default:
			return false // channel full; caller can retry
		}
		slog.Info("orchestrator: paused issue terminate queued", "identifier", identifier)
		return true
	}

	// If running, cancel the worker and mark as terminated (not paused).
	o.userTerminatedMu.Lock()
	o.userTerminatedIDs[identifier] = struct{}{}
	o.userTerminatedMu.Unlock()

	return o.cancelRunningWorker(identifier, func() {
		// Worker wasn't running — clear the marker.
		o.userTerminatedMu.Lock()
		delete(o.userTerminatedIDs, identifier)
		o.userTerminatedMu.Unlock()
	})
}

// ReanalyzeIssue moves a paused issue from the pause set to the ForceReanalyze queue
// so that the next dispatch cycle runs the agent again, bypassing the open-PR guard.
// Returns false if the issue is not currently paused or the event channel is full.
// Safe to call from any goroutine.
func (o *Orchestrator) ReanalyzeIssue(identifier string) bool {
	// Read-only check: is the issue actually paused?
	o.snapMu.RLock()
	_, paused := o.lastSnap.PausedIdentifiers[identifier]
	o.snapMu.RUnlock()
	if !paused {
		return false
	}
	// Route state mutation through the event loop — avoids concurrent map access
	// between this goroutine and the event loop which reads state.ForceReanalyze.
	select {
	case o.events <- OrchestratorEvent{Type: EventForceReanalyze, Identifier: identifier}:
	default:
		// Event channel full; caller can retry.
		return false
	}
	slog.Info("orchestrator: issue queued for forced re-analysis", "identifier", identifier)
	return true
}

// GetPausedOpenPRs returns a copy of the map of paused identifiers that were
// auto-paused due to an open PR being detected. Safe to call from any goroutine.
func (o *Orchestrator) GetPausedOpenPRs() map[string]string {
	o.snapMu.RLock()
	defer o.snapMu.RUnlock()
	result := make(map[string]string, len(o.lastSnap.PausedOpenPRs))
	for k, v := range o.lastSnap.PausedOpenPRs {
		result[k] = v
	}
	return result
}

// SetIssueProfile sets (or clears) a named agent profile override for a specific issue.
// Pass an empty profileName to reset the issue to the default profile.
// Safe to call from any goroutine.
func (o *Orchestrator) SetIssueProfile(identifier, profileName string) {
	o.issueProfilesMu.Lock()
	if profileName == "" {
		delete(o.issueProfiles, identifier)
	} else {
		o.issueProfiles[identifier] = profileName
	}
	o.issueProfilesMu.Unlock()
	slog.Info("orchestrator: issue profile updated", "identifier", identifier, "profile", profileName)
	if o.OnStateChange != nil {
		o.OnStateChange()
	}
}

// DispatchReviewer fetches the issue with the given identifier from the tracker
// and starts a reviewer worker goroutine. The reviewer uses the ReviewerPrompt
// template from the config (falling back to DefaultReviewerPrompt).
// Returns an error if the issue cannot be found; otherwise returns nil immediately
// (the reviewer runs asynchronously in the background).
// Safe to call from any goroutine.
func (o *Orchestrator) DispatchReviewer(identifier string) error {
	var ctx context.Context
	if p := o.runCtx.Load(); p != nil {
		ctx = *p
	} else {
		ctx = context.Background() // fallback if called before Run (tests, etc.)
	}
	o.cfgMu.RLock()
	allStates := append(append([]string{}, o.cfg.Tracker.ActiveStates...), o.cfg.Tracker.TerminalStates...)
	o.cfgMu.RUnlock()
	issues, err := o.tracker.FetchIssuesByStates(ctx, allStates)
	if err != nil {
		return fmt.Errorf("reviewer: fetch issues failed: %w", err)
	}
	var found *domain.Issue
	for i := range issues {
		if issues[i].Identifier == identifier {
			found = &issues[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("reviewer: issue %s not found", identifier)
	}
	go o.runReviewerWorker(ctx, *found)
	return nil
}

// runReviewerWorker runs a single reviewer agent session for the given issue.
// Unlike runWorker it does not register in the orchestrator state machine or
// attempt retries — it is a fire-and-forget background job.
func (o *Orchestrator) runReviewerWorker(ctx context.Context, issue domain.Issue) {
	workerLog := &bufLogger{
		base:       slog.With("issue_id", issue.ID, "issue_identifier", issue.Identifier, "kind", "reviewer"),
		buf:        o.logBuf,
		identifier: issue.Identifier,
	}
	workerLog.Info("worker: reviewer started", "identifier", issue.Identifier)
	if o.logBuf != nil {
		o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: AI reviewer dispatched for %s", issue.Identifier)))
	}

	// Workspace: use the same workspace as the issue so the reviewer can read the PR branch.
	wsPath := ""
	if o.workspace != nil {
		ws, err := o.workspace.EnsureWorkspace(issue.Identifier)
		if err != nil {
			workerLog.Warn("reviewer: workspace setup failed", "error", err)
			return
		}
		wsPath = ws.Path
	}

	// Enrich issue with full details (comments, branch, etc.) before rendering prompt.
	if detailed, err := o.tracker.FetchIssueDetail(ctx, issue.ID); err != nil {
		workerLog.Warn("reviewer: fetch issue detail failed (using cached issue)", "error", err)
	} else {
		issue = *detailed
	}

	reviewerPrompt := o.cfg.Agent.ReviewerPrompt
	renderedPrompt, err := prompt.Render(reviewerPrompt, issue, nil)
	if err != nil {
		workerLog.Warn("reviewer: prompt render failed", "error", err)
		return
	}

	workerHost := ""
	if hosts := o.cfg.Agent.SSHHosts; len(hosts) > 0 {
		workerHost = hosts[0]
	}

	reviewerRunner := o.runners.Get(agent.RunnerKindFrom(o.cfg.Agent.Runner))
	reviewerCommand := o.cfg.Agent.Command
	if reviewerCommand == "claude" && o.cfg.Agent.Runner != "" && o.cfg.Agent.Runner != "claude-code" {
		reviewerCommand = agent.DefaultCommand(agent.RunnerKindFrom(o.cfg.Agent.Runner))
	}
	result, runErr := reviewerRunner.RunTurn(ctx, workerLog, nil, nil, renderedPrompt, wsPath,
		reviewerCommand, workerHost, o.cfg.Agent.ReadTimeoutMs, o.cfg.Agent.TurnTimeoutMs)

	if runErr != nil || result.Failed {
		workerLog.Warn("reviewer: agent turn failed", "error", runErr, "failure", result.FailureText)
		if o.logBuf != nil {
			o.logBuf.Add(issue.Identifier, makeBufLine("WARN", "worker: reviewer turn failed"))
		}
		return
	}

	workerLog.Info("worker: reviewer completed", "identifier", issue.Identifier, "tokens", result.TotalTokens)
	if o.logBuf != nil {
		o.logBuf.Add(issue.Identifier, makeBufLine("INFO", "worker: AI reviewer completed"))
	}
}

// GetRunningIssue returns a copy of the domain.Issue for the currently running
// worker identified by identifier, or nil if no such worker is running.
// Safe to call from any goroutine.
func (o *Orchestrator) GetRunningIssue(identifier string) *domain.Issue {
	o.snapMu.RLock()
	defer o.snapMu.RUnlock()
	for _, entry := range o.lastSnap.Running {
		if entry.Issue.Identifier == identifier {
			issue := entry.Issue
			return &issue
		}
	}
	return nil
}

// Snapshot returns a consistent copy of the current orchestrator state.
// Safe to call from any goroutine.
func (o *Orchestrator) Snapshot() State {
	o.snapMu.RLock()
	defer o.snapMu.RUnlock()
	return o.lastSnap
}

const maxHistory = 200

// SetHistoryFile sets the path for persisting completed runs across restarts.
// Must be called before Run. If path is empty, disk persistence is disabled.
func (o *Orchestrator) SetHistoryFile(path string) {
	o.historyMu.Lock()
	o.historyFile = path
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
	o.completedRuns = runs
	slog.Info("orchestrator: loaded history", "path", o.historyFile, "entries", len(runs))
}

// addCompletedRun appends a finished run to the in-memory history ring buffer
// and persists the ring buffer to disk when a history file is configured.
// Safe to call from any goroutine.
func (o *Orchestrator) addCompletedRun(run CompletedRun) {
	o.historyMu.Lock()
	defer o.historyMu.Unlock()
	o.completedRuns = append(o.completedRuns, run)
	if len(o.completedRuns) > maxHistory {
		o.completedRuns = o.completedRuns[len(o.completedRuns)-maxHistory:]
	}
	if o.historyFile != "" {
		data, err := json.Marshal(o.completedRuns)
		if err == nil {
			if err := os.WriteFile(o.historyFile, data, 0644); err != nil {
				slog.Warn("orchestrator: failed to write history file", "path", o.historyFile, "error", err)
			}
		}
	}
}

// SetPausedFile sets the path for persisting PausedIdentifiers across restarts.
// Must be called before Run.
func (o *Orchestrator) SetPausedFile(path string) {
	o.historyMu.Lock()
	o.pausedFile = path
	o.historyMu.Unlock()
}

// loadPausedFromDisk reads the paused file and pre-populates state.PausedIdentifiers.
// Called once at startup. state is the freshly-initialised event-loop State.
// Supports both the new format (map[identifier]issueID) and the legacy format
// ([]string of identifiers), storing an empty UUID for legacy entries.
func (o *Orchestrator) loadPausedFromDisk(state State) State {
	o.historyMu.RLock()
	path := o.pausedFile
	o.historyMu.RUnlock()
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
		for identifier, issueID := range newFmt {
			state.PausedIdentifiers[identifier] = issueID
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
	}
	slog.Info("orchestrator: loaded paused identifiers (legacy format)", "path", path, "count", len(ids))
	return state
}

// copyStringMap returns a copy of a map[string]string.
func copyStringMap(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
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

// copyRunningMap returns a shallow copy of a map[string]*RunEntry.
// The RunEntry pointers themselves are not deep-copied: the event loop owns
// the pointed-to structs and external readers only need a stable map snapshot.
func copyRunningMap(m map[string]*RunEntry) map[string]*RunEntry {
	cp := make(map[string]*RunEntry, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// copyRetryMap returns a shallow copy of a map[string]*RetryEntry.
func copyRetryMap(m map[string]*RetryEntry) map[string]*RetryEntry {
	cp := make(map[string]*RetryEntry, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// cancelRunningWorker looks up a running worker by identifier and calls its cancel
// function if found, returning true. If no running worker is found, cleanupFn is
// called (to clear the caller's side-channel marker) and false is returned.
// Must NOT be called with snapMu held.
func (o *Orchestrator) cancelRunningWorker(identifier string, cleanupFn func()) bool {
	o.snapMu.RLock()
	defer o.snapMu.RUnlock()
	for _, entry := range o.lastSnap.Running {
		if entry.Issue.Identifier == identifier && entry.WorkerCancel != nil {
			entry.WorkerCancel()
			return true
		}
	}
	if cleanupFn != nil {
		cleanupFn()
	}
	return false
}

// savePausedToDisk writes PausedIdentifiers to disk in the new map format
// {"identifier": "issueUUID"}. Must NOT be called with snapMu held.
func (o *Orchestrator) savePausedToDisk(paused map[string]string) {
	o.historyMu.RLock()
	path := o.pausedFile
	o.historyMu.RUnlock()
	if path == "" {
		return
	}
	data, err := json.Marshal(paused)
	if err == nil {
		if err := os.WriteFile(path, data, 0644); err != nil {
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

// Run executes the orchestrator event loop until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) error {
	o.runCtx.Store(&ctx)
	o.loadHistoryFromDisk()
	state := NewState(o.cfg)
	state = o.loadPausedFromDisk(state)
	tick := time.NewTimer(0)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			state = o.onTick(ctx, state)
			o.storeSnap(state)
			tick.Reset(time.Duration(state.PollIntervalMs) * time.Millisecond)
		case <-o.refresh:
			// Immediate re-poll triggered by the web dashboard refresh button.
			state = o.onTick(ctx, state)
			o.storeSnap(state)
			tick.Reset(time.Duration(state.PollIntervalMs) * time.Millisecond)
		case ev := <-o.events:
			state = o.handleEvent(ctx, state, ev)
			o.storeSnap(state)
		}
	}
}

func (o *Orchestrator) onTick(ctx context.Context, state State) State {
	now := time.Now()

	// 1. Fire any retries whose DueAt has passed.
	state = o.fireRetries(ctx, state, now)

	// 2. Stall detection and tracker-state reconciliation.
	state = ReconcileStalls(state, o.cfg, now, o.events, o.logBuf)
	state = ReconcileTrackerStates(ctx, state, o.cfg, o.tracker, o.events, o.logBuf)

	// 3. Fetch candidates and dispatch eligible issues.
	issues, err := o.tracker.FetchCandidateIssues(ctx)
	if err != nil {
		slog.Warn("orchestrator: fetch candidates failed", "error", err)
		return state
	}

	// Build the current active-identifier set for this tick. We compare it
	// against the previous tick's set in the auto-resume guard below, then
	// store it for the next tick.
	currentActive := make(map[string]struct{}, len(issues))
	for i := range issues {
		currentActive[issues[i].Identifier] = struct{}{}
	}

	// Auto-resume any paused issue that the tracker has moved back to an active
	// state (e.g. user manually set it back to "Todo"). A tracker-side state
	// change is treated as an implicit resume — clear the daemon-side pause so
	// the issue can be dispatched on this tick without requiring a manual resume
	// from the TUI.
	//
	// Guard: only auto-resume if the issue was NOT active on the previous tick.
	// If the issue was already active last tick it was active when the user
	// paused it (e.g. GitHub "todo" label stays throughout an agent run).
	// In that case we must not auto-resume — we wait until the issue leaves
	// active_states and then comes back.
	for i := range issues {
		issue := &issues[i]
		if _, paused := state.PausedIdentifiers[issue.Identifier]; paused {
			if _, wasActive := state.PrevActiveIdentifiers[issue.Identifier]; wasActive {
				// Was already active last tick — user paused it while it was
				// in active_states. Don't auto-resume.
				continue
			}
			delete(state.PausedIdentifiers, issue.Identifier)
			delete(state.PausedOpenPRs, issue.Identifier)
			slog.Info("orchestrator: auto-resumed issue re-activated in tracker",
				"identifier", issue.Identifier, "state", issue.State)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLine("INFO",
					fmt.Sprintf("worker: issue moved to %q in tracker — auto-resumed", issue.State)))
			}
		}
	}

	state.PrevActiveIdentifiers = currentActive

	slots := AvailableSlots(state, o.cfg)
	slog.Debug("orchestrator: tick",
		"fetched", len(issues),
		"running", len(state.Running),
		"slots", slots,
		"max_concurrent", o.cfg.Agent.MaxConcurrentAgents,
	)

	dispatched := 0
	for _, issue := range SortForDispatch(issues) {
		if AvailableSlots(state, o.cfg) <= 0 {
			slog.Debug("orchestrator: no slots available, stopping dispatch",
				"running", len(state.Running),
				"max_concurrent", o.cfg.Agent.MaxConcurrentAgents,
			)
			break
		}
		if !IsEligible(issue, state, o.cfg) {
			reason := IneligibleReason(issue, state, o.cfg)
			slog.Info("orchestrator: issue not eligible, skipping",
				"identifier", issue.Identifier,
				"state", issue.State,
				"reason", reason,
			)
			continue
		}
		state = o.dispatch(ctx, state, issue, 0)
		dispatched++
	}
	if dispatched > 0 || len(issues) > 0 {
		slog.Info("orchestrator: dispatch complete",
			"fetched", len(issues),
			"dispatched", dispatched,
			"running", len(state.Running),
			"slots_remaining", AvailableSlots(state, o.cfg),
		)
	}
	return state
}

// fireRetries processes all RetryAttempts whose DueAt <= now.
func (o *Orchestrator) fireRetries(ctx context.Context, state State, now time.Time) State {
	// Collect IDs first to avoid non-deterministic behaviour when ScheduleRetry
	// writes back to the same map key during iteration.
	ids := make([]string, 0, len(state.RetryAttempts))
	for id := range state.RetryAttempts {
		ids = append(ids, id)
	}
	for _, issueID := range ids {
		entry, ok := state.RetryAttempts[issueID]
		if !ok {
			continue // removed by an earlier iteration (e.g. CancelRetry)
		}
		// Skip retries for paused issues and release the claim.
		if _, paused := state.PausedIdentifiers[entry.Identifier]; paused {
			state = CancelRetry(state, issueID)
			continue
		}
		if now.Before(entry.DueAt) {
			continue
		}

		refreshed, err := o.tracker.FetchIssueStatesByIDs(ctx, []string{issueID})
		if err != nil {
			slog.Warn("retry: tracker fetch failed, rescheduling",
				"issue_id", issueID, "error", err)
			state = ScheduleRetry(state, issueID, entry.Attempt+1, entry.Identifier,
				"retry poll failed", now, BackoffMs(entry.Attempt+1, o.cfg.Agent.MaxRetryBackoffMs))
			continue
		}

		if len(refreshed) == 0 || !isActiveState(refreshed[0].State, o.cfg) {
			slog.Info("retry: issue no longer active, releasing claim", "issue_id", issueID)
			state = CancelRetry(state, issueID)
			continue
		}

		if AvailableSlots(state, o.cfg) <= 0 {
			slog.Debug("retry: no slots, rescheduling", "issue_id", issueID)
			state = ScheduleRetry(state, issueID, entry.Attempt, entry.Identifier,
				"no available orchestrator slots", now, 1000)
			continue
		}

		delete(state.RetryAttempts, issueID)
		state = o.dispatch(ctx, state, refreshed[0], entry.Attempt)
	}
	return state
}

func (o *Orchestrator) dispatch(ctx context.Context, state State, issue domain.Issue, attempt int) State {
	workerCtx, workerCancel := context.WithCancel(ctx)

	// Check if this issue has been queued for forced re-analysis (bypasses open-PR guard).
	skipPRCheck := false
	if _, forced := state.ForceReanalyze[issue.Identifier]; forced {
		skipPRCheck = true
		delete(state.ForceReanalyze, issue.Identifier)
		delete(state.PausedOpenPRs, issue.Identifier)
		if o.logBuf != nil {
			o.logBuf.Add(issue.Identifier, makeBufLine("INFO", "worker: forced re-analysis requested"))
		}
	}

	// Round-robin SSH host selection. Empty string = run locally.
	workerHost := ""
	if hosts := o.cfg.Agent.SSHHosts; len(hosts) > 0 {
		workerHost = hosts[o.sshHostIdx%len(hosts)]
		o.sshHostIdx++
	}

	// Resolve agent command: check for a per-issue profile override first.
	agentCommand := o.cfg.Agent.Command
	o.issueProfilesMu.Lock()
	profileName := o.issueProfiles[issue.Identifier]
	o.issueProfilesMu.Unlock()
	if profileName != "" {
		o.cfgMu.RLock()
		profile, ok := o.cfg.Agent.Profiles[profileName]
		o.cfgMu.RUnlock()
		if ok && profile.Command != "" {
			agentCommand = profile.Command
			slog.Info("orchestrator: using profile command",
				"identifier", issue.Identifier, "profile", profileName, "command", agentCommand)
		} else {
			slog.Warn("orchestrator: profile not found or has no command, using default",
				"identifier", issue.Identifier, "profile", profileName)
		}
	}

	// Resolve runner kind: check for per-ticket "agent:<runner>" label override,
	// then fall back to global config.
	runnerKind := agent.RunnerKind(o.cfg.Agent.Runner)
	for _, label := range issue.Labels {
		if kind, ok := agent.RunnerKindFromLabel(label); ok {
			runnerKind = kind
			slog.Info("orchestrator: per-ticket runner override from label",
				"identifier", issue.Identifier, "runner", string(runnerKind), "label", label)
			break
		}
	}

	if o.DryRun {
		workerCancel()
		slog.Info("orchestrator: [DRY-RUN] would dispatch agent",
			"identifier", issue.Identifier, "issue_id", issue.ID,
			"command", agentCommand, "worker_host", workerHost, "runner", string(runnerKind))
		state.Claimed[issue.ID] = struct{}{} // claim so it doesn't re-dispatch this tick
		return state
	}

	state.Claimed[issue.ID] = struct{}{}
	state.Running[issue.ID] = &RunEntry{
		Issue:        issue,
		WorkerHost:   workerHost,
		Backend:      string(runnerKind),
		StartedAt:    time.Now(),
		RetryAttempt: &attempt,
		WorkerCancel: workerCancel,
	}

	if o.OnDispatch != nil {
		o.OnDispatch(issue.ID)
	}

	go o.runWorker(workerCtx, issue, attempt, workerHost, agentCommand, profileName, skipPRCheck, runnerKind)
	return state
}

// transitionToWorking moves the issue to the configured working state (e.g. "In Progress").
// Called in the dispatch goroutine after claiming; errors are logged and ignored.
func (o *Orchestrator) transitionToWorking(ctx context.Context, issue domain.Issue) {
	target := o.cfg.Tracker.WorkingState
	if target == "" || strings.EqualFold(issue.State, target) {
		return
	}
	if err := o.tracker.UpdateIssueState(ctx, issue.ID, target); err != nil {
		slog.Warn("orchestrator: state transition failed (ignored)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier,
			"target_state", target, "error", err)
		return
	}
	slog.Info("orchestrator: issue transitioned",
		"issue_id", issue.ID, "issue_identifier", issue.Identifier,
		"from", issue.State, "to", target)
	if o.logBuf != nil {
		o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: → %s", target)))
	}
}

// runWorker implements the full per-issue lifecycle: workspace, hooks, multi-turn loop.
// Runs in its own goroutine; communicates back only via o.events.
// workerHost is the SSH host to run the agent on; empty string means run locally.
// agentCommand is the resolved agent command to run (may differ from cfg.Agent.Command
// when a per-issue profile override is active).
// profileName is the active named profile for this issue (may be ""); used to
// exclude the current agent from its own sub-agent context in teams mode.
// skipPRCheck bypasses the open-PR guard (used when a forced re-analysis is requested).
// runnerKind specifies which agent backend to use for this worker.
func (o *Orchestrator) runWorker(ctx context.Context, issue domain.Issue, attempt int, workerHost string, agentCommand string, profileName string, skipPRCheck bool, runnerKind agent.RunnerKind) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("worker panic: %v", r)
			slog.Error("worker panicked",
				"issue_id", issue.ID,
				"issue_identifier", issue.Identifier,
				"panic", r,
				"stack", string(debug.Stack()))
			o.sendExit(ctx, issue, attempt, TerminalFailed, err)
		}
	}()
	// --- Workspace ---
	wsPath := ""
	if o.workspace != nil {
		ws, err := o.workspace.EnsureWorkspace(issue.Identifier)
		if err != nil {
			slog.Warn("worker: workspace setup failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLine("ERROR", fmt.Sprintf("worker: workspace setup failed: %v", err)))
			}
			o.sendExit(ctx, issue, attempt, TerminalFailed, err)
			return
		}
		wsPath = ws.Path

		if ws.CreatedNow {
			hookLog := o.hookLogFn(issue.Identifier)
			if err := workspace.RunHook(ctx, o.cfg.Hooks.AfterCreate, wsPath, o.cfg.Hooks.TimeoutMs, hookLog); err != nil {
				slog.Warn("worker: after_create hook failed, removing workspace so next retry re-runs it",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
				if o.logBuf != nil {
					o.logBuf.Add(issue.Identifier, makeBufLine("ERROR", fmt.Sprintf("worker: after_create hook failed: %v", err)))
				}
				_ = o.workspace.RemoveWorkspace(issue.Identifier)
				o.sendExit(ctx, issue, attempt, TerminalFailed, err)
				return
			}
		}
	}

	// --- Check for an existing open PR before resetting the branch ---
	// The before_run hook (e.g. "git checkout main && git reset --hard") will
	// switch branches and make gh pr view blind to the issue branch.  We must
	// therefore check while the workspace still points at the previous run's
	// branch.  If a PR already exists we skip the agent run entirely to break
	// the infinite-loop where an "In Progress" issue keeps being re-dispatched.
	// skipPRCheck bypasses this guard when a forced re-analysis is requested.
	if !skipPRCheck {
		if wsPath != "" {
			if existingPR := workspace.FindOpenPRURL(ctx, wsPath); existingPR != "" {
				slog.Info("worker: open PR already exists, auto-pausing issue",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "pr_url", existingPR)
				if o.logBuf != nil {
					o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: open PR already exists (%s) — pausing until merged", existingPR)))
				}
				o.cfgMu.RLock()
				completionState := o.cfg.Tracker.CompletionState
				o.cfgMu.RUnlock()
				if completionState != "" {
					if err := o.tracker.UpdateIssueState(ctx, issue.ID, completionState); err != nil {
						slog.Warn("worker: completion state transition failed after PR detection",
							"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
					}
				}
				o.prURLsMu.Lock()
				o.prURLsBeforePause[issue.Identifier] = existingPR
				o.prURLsMu.Unlock()
				if o.logBuf != nil {
					o.logBuf.Remove(issue.Identifier)
				}
				o.sendExit(ctx, issue, attempt, TerminalSkippedOpenPR, nil)
				return
			}
		}
	} else {
		slog.Info("worker: skipping PR check (forced re-analysis requested)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
		if o.logBuf != nil {
			o.logBuf.Add(issue.Identifier, makeBufLine("INFO", "worker: forced re-analysis of existing PR"))
		}
	}

	// Transition issue to working state (e.g. Todo → In Progress).
	o.transitionToWorking(ctx, issue)

	// --- Multi-turn loop ---
	// before_run hook runs once per worker invocation (not per turn), so that
	// hooks like "git reset --hard origin/main" set up a clean workspace for the
	// attempt without wiping Claude's work between turns.
	if wsPath != "" {
		hookLog := o.hookLogFn(issue.Identifier)
		if err := workspace.RunHook(ctx, o.cfg.Hooks.BeforeRun, wsPath, o.cfg.Hooks.TimeoutMs, hookLog); err != nil {
			slog.Warn("worker: before_run hook failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLine("ERROR", fmt.Sprintf("worker: before_run hook failed: %v", err)))
			}
			o.sendExit(ctx, issue, attempt, TerminalFailed, err)
			return
		}
	}

	// Checkout the issue's tracked branch so retried workers resume the agent's
	// previous work instead of starting over from the default branch.
	// Runs AFTER before_run (which resets to main) so the branch layering is:
	//   1. before_run: git checkout main && git reset --hard origin/main
	//   2. (here):    git checkout <feature-branch>   ← agent continues from here
	// On a fresh dispatch the branch typically doesn't exist yet — the checkout
	// fails silently and the agent creates it during its first turn.
	if wsPath != "" && issue.BranchName != nil && *issue.BranchName != "" {
		if b := *issue.BranchName; !isDefaultBranch(b) {
			if err := workspace.CheckoutBranch(ctx, wsPath, b); err != nil {
				slog.Warn("worker: branch checkout failed, agent will start from current branch",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier,
					"branch", b, "error", err)
			} else {
				slog.Info("worker: resuming on tracked branch",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "branch", b)
			}
		}
	}

	var sessionID *string
	var allTextBlocks []string // accumulate all Claude text blocks for the final tracker comment
	for turn := 1; turn <= o.cfg.Agent.MaxTurns; turn++ {
		// Enrich issue with comments before rendering the first-turn prompt.
		if turn == 1 {
			if detailed, err := o.tracker.FetchIssueDetail(ctx, issue.ID); err != nil {
				slog.Warn("worker: fetch issue detail failed (using cached issue)",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			} else {
				issue = *detailed
			}
		}

		// Render prompt
		var attemptPtr *int
		if attempt > 0 {
			a := attempt
			attemptPtr = &a
		}
		renderedPrompt, err := prompt.Render(o.cfg.PromptTemplate, issue, attemptPtr)
		if err != nil {
			slog.Warn("worker: prompt render failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLine("ERROR", fmt.Sprintf("worker: prompt render failed: %v", err)))
			}
			o.runAfterHook(ctx, wsPath, issue.ID)
			o.sendExit(ctx, issue, attempt, TerminalFailed, err)
			return
		}
		// Append the active profile's prompt (role context) whenever a named profile
		// is selected, regardless of agent mode. This lets profile prompts work in
		// solo/subagents mode too — not just teams mode.
		// Snapshot the contested cfg fields once under cfgMu to avoid data races
		// with HTTP handler goroutines that may mutate them concurrently.
		o.cfgMu.RLock()
		agentMode := o.cfg.Agent.AgentMode
		profilesSnap := make(map[string]config.AgentProfile, len(o.cfg.Agent.Profiles))
		for k, v := range o.cfg.Agent.Profiles {
			profilesSnap[k] = v
		}
		o.cfgMu.RUnlock()

		if profileName != "" {
			if profile, ok := profilesSnap[profileName]; ok && profile.Prompt != "" {
				renderedPrompt += "\n\n" + profile.Prompt
			}
		}
		// In teams mode, also append sub-agent roster context so Claude knows which
		// specialised agents it can spawn via the Task tool.
		if agentMode == "teams" {
			if subCtx := buildSubAgentContext(profilesSnap, profileName); subCtx != "" {
				renderedPrompt += "\n\n" + subCtx
			}
		}

		// Run agent turn — pass a logger pre-seeded with the issue identifier so
		// Claude's live output appears in the log stream and can be filtered by identifier.
		// onProgress sends a live EventWorkerUpdate each time Claude produces output so
		// the dashboard reflects token counts and session ID mid-turn.
		workerLog := &bufLogger{
			base:       slog.With("issue_id", issue.ID, "issue_identifier", issue.Identifier),
			buf:        o.logBuf,
			identifier: issue.Identifier,
		}
		onProgress := func(partial agent.TurnResult) {
			select {
			case o.events <- OrchestratorEvent{
				Type:    EventWorkerUpdate,
				IssueID: issue.ID,
				RunEntry: &RunEntry{
					TurnCount:    turn,
					TotalTokens:  partial.TotalTokens,
					InputTokens:  partial.InputTokens,
					OutputTokens: partial.OutputTokens,
					SessionID:    partial.SessionID,
					LastMessage:  partial.LastText,
				},
			}:
			default:
			}
		}
		turnStart := time.Now()
		workerRunner := o.runners.Get(runnerKind)
		// Use the runner's default command if config still has "claude" but runner is different.
		effectiveCommand := agentCommand
		if effectiveCommand == "claude" && runnerKind != agent.RunnerClaudeCode {
			effectiveCommand = agent.DefaultCommand(runnerKind)
		}
		result, runErr := workerRunner.RunTurn(ctx, workerLog, onProgress, sessionID, renderedPrompt, wsPath,
			effectiveCommand, workerHost, o.cfg.Agent.ReadTimeoutMs, o.cfg.Agent.TurnTimeoutMs)

		if result.SessionID != "" {
			s := result.SessionID
			sessionID = &s
		}

		// Accumulate all Claude text blocks for the final session comment.
		allTextBlocks = append(allTextBlocks, result.AllTextBlocks...)

		// after_run hook (best-effort, logged and ignored)
		o.runAfterHook(ctx, wsPath, issue.ID)

		// Track the current git branch after each turn so retried workers can
		// resume from the same branch. Only fires when the agent has switched to
		// a non-default branch (i.e. created its feature branch).
		if wsPath != "" {
			if currentBranch := workspace.GetCurrentBranch(ctx, wsPath); !isDefaultBranch(currentBranch) {
				if issue.BranchName == nil || *issue.BranchName != currentBranch {
					if err := o.tracker.SetIssueBranch(ctx, issue.ID, currentBranch); err != nil {
						workerLog.Warn("worker: set branch failed (ignored)",
							"branch", currentBranch, "error", err)
					} else {
						b := currentBranch
						issue.BranchName = &b
						workerLog.Info("worker: branch tracked on issue", "branch", currentBranch)
					}
				}
			}
		}

		if result.Failed {
			// A result error with no failure text and no tokens produced means the
			// claude CLI was asked to --resume a session that had already concluded.
			// Treat this as a clean session end rather than a real failure so the
			// issue does not land in the retry queue.
			if result.FailureText == "" && result.InputTokens == 0 && result.OutputTokens == 0 {
				slog.Info("worker: empty result error on 0-token turn — treating as clean session end",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "turn", turn)
				break
			}
			cause := runErr
			if cause == nil {
				msg := fmt.Sprintf("turn %d: agent reported failure", turn)
				if result.FailureText != "" {
					msg = fmt.Sprintf("turn %d: %s", turn, result.FailureText)
				}
				cause = fmt.Errorf("%s", msg)
			}
			slog.Warn("worker: turn failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier,
				"turn", turn, "error", cause)
			// Emit full error detail to the per-issue log buffer (#7).
			if o.logBuf != nil {
				detail := cause.Error()
				if result.FailureText != "" && !strings.Contains(detail, result.FailureText) {
					detail = detail + " | " + result.FailureText
				}
				o.logBuf.Add(issue.Identifier, formatBufLine("WARN", "worker: turn failed", []any{"detail", detail}))
			}
			o.sendExit(ctx, issue, attempt, TerminalFailed, cause)
			return
		}

		// A turn that produces no tokens means the agent has nothing more to do
		// (the session was already concluded). Break early for a clean exit.
		if result.InputTokens == 0 && result.OutputTokens == 0 {
			slog.Info("worker: 0-token turn — session concluded, exiting loop",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "turn", turn)
			break
		}

		// Emit turn summary line (#3): turn N complete — Δin/Δout tokens, elapsed Xs.
		if o.logBuf != nil {
			elapsed := time.Since(turnStart)
			summary := fmt.Sprintf("turn %d complete — +%d in / +%d out tokens, %.1fs",
				turn, result.InputTokens, result.OutputTokens, elapsed.Seconds())
			o.logBuf.Add(issue.Identifier, formatBufLine("INFO", "worker: turn_summary", []any{"summary", summary}))
		}

		// Send non-blocking progress update so the dashboard shows live turn/token data.
		select {
		case o.events <- OrchestratorEvent{
			Type:    EventWorkerUpdate,
			IssueID: issue.ID,
			RunEntry: &RunEntry{
				TurnCount:    turn,
				LastMessage:  result.ResultText,
				TotalTokens:  result.TotalTokens,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
				SessionID: func() string {
					if sessionID != nil {
						return *sessionID
					}
					return ""
				}(),
			},
		}:
		default:
			// Event loop busy — skip this tick, next turn will send another update.
		}

		// Refresh tracker state to decide whether to continue
		refreshed, err := o.tracker.FetchIssueStatesByIDs(ctx, []string{issue.ID})
		if err != nil {
			slog.Warn("worker: state refresh failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "turn", turn, "error", err)
			o.sendExit(ctx, issue, attempt, TerminalFailed, err)
			return
		}
		if len(refreshed) > 0 {
			savedBranch := issue.BranchName
			issue = refreshed[0]
			// FetchIssueStatesByIDs may not return the branch name (e.g. GitHub's
			// fetchSingleIssue doesn't scan comments). Preserve what we tracked.
			if issue.BranchName == nil {
				issue.BranchName = savedBranch
			}
		}

		if !isActiveState(issue.State, o.cfg) {
			break // issue left active states — clean exit
		}
	}

	// If the agent created a PR during this run, comment its URL on the tracker
	// issue.  This runs before the session summary so the PR link is visible even
	// on trackers that truncate long comments.  Uses the same gh CLI check as the
	// pre-run guard (now the workspace is on the newly-created branch).
	if wsPath != "" {
		if prURL := workspace.FindOpenPRURL(ctx, wsPath); prURL != "" {
			prComment := fmt.Sprintf("🔗 Pull request created: %s", prURL)
			if err := o.tracker.CreateComment(ctx, issue.ID, prComment); err != nil {
				slog.Warn("worker: create PR comment failed (ignored)",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			} else {
				slog.Info("worker: PR link commented on issue",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "pr_url", prURL)
				if o.logBuf != nil {
					// Use pr_opened key so parseLogLine recognises this as a "pr" event (#4).
					o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: pr_opened url=%s", prURL)))
				}
			}
		}
	}

	// Post one comprehensive comment covering the full session narration (best-effort).
	if comment := formatSessionComment(allTextBlocks, issue.Identifier); comment != "" {
		if err := o.tracker.CreateComment(ctx, issue.ID, comment); err != nil {
			slog.Warn("worker: create session comment failed (ignored)",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
		}
	}

	// Move issue to completion_state (e.g. "In Review") so Symphony stops
	// re-dispatching it. Without this, issues stay in active_states after a
	// successful run and get picked up again on the next retry tick.
	// Retried up to 4 times (with exponential backoff) to guard against
	// transient API errors that would otherwise cause an infinite dispatch loop.
	// Skip if the worker context was cancelled (user paused/killed the issue) —
	// transitioning state on a cancelled run would wrongly move a paused issue.
	o.cfgMu.RLock()
	completionState := o.cfg.Tracker.CompletionState
	o.cfgMu.RUnlock()
	if completionState != "" && ctx.Err() == nil {
		slog.Info("worker: transitioning to completion state",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier, "target_state", completionState)
		if o.logBuf != nil {
			o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: moving issue to %q in tracker", completionState)))
		}
		var transitionErr error
		for i := range 4 {
			if i > 0 {
				delay := time.Duration(1<<uint(i-1)) * 2 * time.Second // 2s, 4s, 8s
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					transitionErr = ctx.Err()
					goto doneTransition
				}
			}
			if transitionErr = o.tracker.UpdateIssueState(ctx, issue.ID, completionState); transitionErr == nil {
				break
			}
			slog.Warn("worker: completion state transition attempt failed, retrying",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier,
				"target_state", completionState, "attempt", i+1, "error", transitionErr)
		}
	doneTransition:
		if transitionErr != nil {
			slog.Error("worker: completion state transition failed after retries",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier,
				"target_state", completionState, "error", transitionErr)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLine("ERROR", fmt.Sprintf("worker: state transition to %q failed: %v — issue paused", completionState, transitionErr)))
			}
			// Pause the issue so it doesn't re-enter the dispatch loop and cause
			// an infinite retry cycle. The user can resume it manually.
			o.userCancelledMu.Lock()
			o.userCancelledIDs[issue.Identifier] = struct{}{}
			o.userCancelledMu.Unlock()
		} else {
			slog.Info("worker: issue moved to completion state",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "state", completionState)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: → %s", completionState)))
				o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: ✓ issue moved to %q", completionState)))
			}
		}
	}

	// Release the log buffer for this issue to free memory (after the completion
	// state transition so any transition errors are visible in the TUI log pane).
	if o.logBuf != nil {
		o.logBuf.Remove(issue.Identifier)
	}

	o.sendExit(ctx, issue, attempt, TerminalSucceeded, nil)
}

// hookLogFn returns a function suitable for workspace.RunHook's logFn parameter.
// Each hook output line is forwarded to the per-issue log buffer as an info entry.
// Returns nil when logBuf is not configured (no-op in RunHook).
func (o *Orchestrator) hookLogFn(identifier string) func(string) {
	if o.logBuf == nil {
		return nil
	}
	return func(line string) {
		o.logBuf.Add(identifier, makeBufLine("INFO", "hook: "+line))
	}
}

func (o *Orchestrator) runAfterHook(ctx context.Context, wsPath, issueID string) {
	if wsPath == "" {
		return
	}
	if err := workspace.RunHook(ctx, o.cfg.Hooks.AfterRun, wsPath, o.cfg.Hooks.TimeoutMs); err != nil {
		slog.Warn("worker: after_run hook failed (ignored)", "issue_id", issueID, "error", err)
	}
}

func (o *Orchestrator) sendExit(ctx context.Context, issue domain.Issue, attempt int, reason TerminalReason, err error) {
	ev := OrchestratorEvent{
		Type:    EventWorkerExited,
		IssueID: issue.ID,
		RunEntry: &RunEntry{
			Issue:          issue,
			TerminalReason: reason,
			RetryAttempt:   &attempt,
		},
		Error: err,
	}
	// If the worker context is already cancelled (e.g. user-triggered pause via
	// CancelIssue), the exit event must still reach the event loop so that
	// PausedIdentifiers is set correctly.  Fall back to a background-derived
	// context so we never drop the exit notification just because the worker's
	// own context was cancelled.
	sendCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
	}
	select {
	case o.events <- ev:
	case <-(*o.runCtx.Load()).Done():
		slog.Warn("worker: exit event dropped (orchestrator exited)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
	case <-sendCtx.Done():
		slog.Warn("worker: exit event not delivered (orchestrator shutting down)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
	}
}

func (o *Orchestrator) handleEvent(_ context.Context, state State, ev OrchestratorEvent) State {
	switch ev.Type {
	case EventWorkerUpdate:
		if entry, ok := state.Running[ev.IssueID]; ok && ev.RunEntry != nil {
			now := time.Now()
			entry.LastEventAt = &now
			if ev.RunEntry.TurnCount > 0 {
				entry.TurnCount = ev.RunEntry.TurnCount
			}
			if ev.RunEntry.TotalTokens > 0 {
				entry.TotalTokens = ev.RunEntry.TotalTokens
				entry.InputTokens = ev.RunEntry.InputTokens
				entry.OutputTokens = ev.RunEntry.OutputTokens
			}
			if ev.RunEntry.LastMessage != "" {
				entry.LastMessage = ev.RunEntry.LastMessage
			}
			if ev.RunEntry.SessionID != "" {
				entry.SessionID = ev.RunEntry.SessionID
			}
		}

	case EventForceReanalyze:
		// Runs in the event loop goroutine — safe to mutate state maps directly.
		if _, isPaused := state.PausedIdentifiers[ev.Identifier]; isPaused {
			delete(state.PausedIdentifiers, ev.Identifier)
			state.ForceReanalyze[ev.Identifier] = struct{}{}
			// Persist immediately so a crash between ticks doesn't re-pause the issue.
			o.savePausedToDisk(copyStringMap(state.PausedIdentifiers))
			slog.Info("orchestrator: issue un-paused for forced re-analysis",
				"identifier", ev.Identifier)
			if o.OnStateChange != nil {
				o.OnStateChange()
			}
		}

	case EventResumeIssue:
		if _, isPaused := state.PausedIdentifiers[ev.Identifier]; isPaused {
			delete(state.PausedIdentifiers, ev.Identifier)
			o.savePausedToDisk(copyStringMap(state.PausedIdentifiers))
			slog.Info("orchestrator: issue resumed", "identifier", ev.Identifier)
			if o.OnStateChange != nil {
				o.OnStateChange()
			}
		}

	case EventTerminatePaused:
		if _, isPaused := state.PausedIdentifiers[ev.Identifier]; isPaused {
			delete(state.PausedIdentifiers, ev.Identifier)
			o.savePausedToDisk(copyStringMap(state.PausedIdentifiers))
			slog.Info("orchestrator: paused issue terminated (claim released)", "identifier", ev.Identifier)
			// Move the issue back to Backlog (or first active state if no backlog
			// is configured) to remove the in-progress label and prevent it from
			// being immediately re-dispatched or left with a stale working label.
			// Skip if we don't have the issue UUID (legacy disk entry).
			if ev.IssueID != "" {
				backlogStates := o.cfg.Tracker.BacklogStates
				activeStates := o.cfg.Tracker.ActiveStates
				var targetState string
				if len(backlogStates) > 0 {
					targetState = backlogStates[0]
				} else if len(activeStates) > 0 {
					// No backlog configured — revert to the first active state so the
					// working-state label (e.g. "in-progress") is removed.
					targetState = activeStates[0]
				}
				if targetState != "" {
					// Hold the issue in DiscardingIdentifiers until the label update
					// completes. The TUI's background TriggerPoll fires every ~30s and
					// would re-dispatch the issue if it still has its "In Progress" label
					// during the async window. DiscardingIdentifiers blocks IsEligible.
					state.DiscardingIdentifiers[ev.Identifier] = struct{}{}
					issueID := ev.IssueID
					identifier := ev.Identifier
					go func() {
						ctx, cancel := context.WithTimeout(*o.runCtx.Load(), 15*time.Second)
						defer cancel()
						if err := o.tracker.UpdateIssueState(ctx, issueID, targetState); err != nil {
							slog.Warn("orchestrator: failed to transition discarded issue",
								"identifier", identifier, "target_state", targetState, "error", err)
						} else {
							slog.Info("orchestrator: discarded issue transitioned",
								"identifier", identifier, "state", targetState)
						}
						// Signal that the label update is complete (success or failure) so
						// DiscardingIdentifiers can be cleared.
						select {
						case o.events <- OrchestratorEvent{Type: EventDiscardComplete, Identifier: identifier}:
						default:
						}
					}()
				}
			}
			if o.OnStateChange != nil {
				o.OnStateChange()
			}
		}

	case EventDiscardComplete:
		delete(state.DiscardingIdentifiers, ev.Identifier)
		slog.Info("orchestrator: discard complete, issue released", "identifier", ev.Identifier)

	case EventWorkerExited:
		// Capture the live entry before deletion so we can record history.
		liveEntry := state.Running[ev.IssueID]
		delete(state.Running, ev.IssueID)

		if ev.RunEntry == nil {
			// Emitted by ReconcileStalls or ReconcileTrackerStates (not-found path);
			// claim and retry already managed by the reconcile function.
			return state
		}

		now := time.Now()
		issue := ev.RunEntry.Issue
		attempt := 0
		if ev.RunEntry.RetryAttempt != nil {
			attempt = *ev.RunEntry.RetryAttempt
		}

		// Check if this exit was caused by a user kill (CancelIssue → pause)
		// or a hard terminate (TerminateIssue → release claim, no pause).
		o.userCancelledMu.Lock()
		_, wasCancelledByUser := o.userCancelledIDs[issue.Identifier]
		if wasCancelledByUser {
			delete(o.userCancelledIDs, issue.Identifier)
		}
		o.userCancelledMu.Unlock()

		o.userTerminatedMu.Lock()
		_, wasTerminatedByUser := o.userTerminatedIDs[issue.Identifier]
		if wasTerminatedByUser {
			delete(o.userTerminatedIDs, issue.Identifier)
		}
		o.userTerminatedMu.Unlock()

		if wasCancelledByUser {
			state.PausedIdentifiers[issue.Identifier] = issue.ID
			delete(state.Claimed, ev.IssueID)
			slog.Info("orchestrator: issue paused by user kill",
				"issue_id", ev.IssueID, "identifier", issue.Identifier)
			o.recordHistory(liveEntry, issue, now, "cancelled")
			return state
		}

		if wasTerminatedByUser {
			// Hard terminate: release claim without pausing — issue will be
			// re-dispatched on the next poll cycle.
			delete(state.Claimed, ev.IssueID)
			slog.Info("orchestrator: issue terminated by user (claim released)",
				"issue_id", ev.IssueID, "identifier", issue.Identifier)
			o.recordHistory(liveEntry, issue, now, "cancelled")
			return state
		}

		switch ev.RunEntry.TerminalReason {
		case TerminalCanceledByReconciliation:
			// Reconcile already released the claim; just log.
			delete(state.Claimed, ev.IssueID)
			slog.Info("orchestrator: worker canceled by reconciliation",
				"issue_id", ev.IssueID, "issue_identifier", issue.Identifier)
			// Not recorded — the issue will be re-dispatched.

		case TerminalSkippedOpenPR:
			// Open PR detected — auto-pause so this issue isn't re-dispatched
			// every poll cycle. The user can resume it from the dashboard if needed,
			// or it will be cleaned up once the PR merges and Linear moves the
			// issue to a terminal state.
			delete(state.Claimed, ev.IssueID)
			state.PausedIdentifiers[issue.Identifier] = issue.ID
			// Record the PR URL so the dashboard can show a link and offer re-analysis.
			o.prURLsMu.Lock()
			prURL := o.prURLsBeforePause[issue.Identifier]
			delete(o.prURLsBeforePause, issue.Identifier)
			o.prURLsMu.Unlock()
			if prURL != "" {
				state.PausedOpenPRs[issue.Identifier] = prURL
			}
			slog.Info("orchestrator: issue auto-paused (open PR exists)",
				"issue_id", ev.IssueID, "issue_identifier", issue.Identifier)
			// Not recorded — the issue is paused, not finished.

		case TerminalSucceeded:
			// Release the claim — the issue completed successfully.
			// Do NOT schedule a retry; successful completions must not appear in
			// the retry queue and must not cause infinite re-dispatch loops.
			delete(state.Claimed, ev.IssueID)
			slog.Info("orchestrator: worker succeeded, claim released",
				"issue_id", ev.IssueID, "issue_identifier", issue.Identifier)
			o.recordHistory(liveEntry, issue, now, "succeeded")

		default: // TerminalFailed, TerminalTimedOut
			// context.Canceled means the worker was stopped by the orchestrator
			// (stall timeout, reload, shutdown) — not a real failure. Release the
			// claim so the issue can be dispatched fresh on the next poll cycle.
			if ev.Error != nil && errors.Is(ev.Error, context.Canceled) {
				delete(state.Claimed, ev.IssueID)
				slog.Info("orchestrator: worker context cancelled, claim released for re-dispatch",
					"issue_id", ev.IssueID, "issue_identifier", issue.Identifier)
				// Not recorded — the issue will be re-dispatched.
			} else {
				backoff := BackoffMs(attempt+1, o.cfg.Agent.MaxRetryBackoffMs)
				errMsg := ""
				if ev.Error != nil {
					errMsg = ev.Error.Error()
				}
				state = ScheduleRetry(state, ev.IssueID, attempt+1, issue.Identifier, errMsg, now, backoff)
				slog.Info("orchestrator: worker failed, retry scheduled",
					"issue_id", ev.IssueID, "issue_identifier", issue.Identifier,
					"attempt", attempt+1, "backoff_ms", backoff)
				o.recordHistory(liveEntry, issue, now, "failed")
			}
		}
	}
	return state
}

// recordHistory appends a completed run to the history ring buffer.
// liveEntry may be nil if the worker exited before the first update.
func (o *Orchestrator) recordHistory(liveEntry *RunEntry, issue domain.Issue, finishedAt time.Time, status string) {
	run := CompletedRun{
		Identifier: issue.Identifier,
		Title:      issue.Title,
		FinishedAt: finishedAt,
		Status:     status,
	}
	if liveEntry != nil {
		run.StartedAt = liveEntry.StartedAt
		run.ElapsedMs = finishedAt.Sub(liveEntry.StartedAt).Milliseconds()
		run.TurnCount = liveEntry.TurnCount
		run.TotalTokens = liveEntry.TotalTokens
		run.InputTokens = liveEntry.InputTokens
		run.OutputTokens = liveEntry.OutputTokens
		run.WorkerHost = liveEntry.WorkerHost
		run.Backend = liveEntry.Backend
		run.SessionID = liveEntry.SessionID
	} else {
		run.StartedAt = finishedAt
	}
	o.addCompletedRun(run)
}

// buildSubAgentContext generates a "## Available Sub-Agents" section that is
// appended to the rendered prompt when agent teams mode is active.
// activeProfile is excluded from the list so the agent doesn't try to spawn itself.
// Returns an empty string when there are no other profiles to list.
func buildSubAgentContext(profiles map[string]config.AgentProfile, activeProfile string) string {
	if len(profiles) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Available Sub-Agents\n\n")
	b.WriteString("You can spawn the following specialised sub-agents using the Task tool:\n\n")
	for name, p := range profiles {
		if name == activeProfile {
			continue
		}
		if p.Prompt != "" {
			b.WriteString("- **" + name + "**: " + p.Prompt + "\n")
		} else {
			b.WriteString("- **" + name + "**\n")
		}
	}
	b.WriteString("\nUse the Task tool with the sub-agent description when you need specialised help.")
	return b.String()
}

// isDefaultBranch returns true for the standard default branch names and for
// detached HEAD / empty — branches we never want to track or checkout as a
// feature branch resume target.
func isDefaultBranch(branch string) bool {
	return branch == "" || branch == "main" || branch == "master" || branch == "HEAD"
}

// StartupTerminalCleanup fetches terminal issues and removes their workspaces.
// Fetch failure logs a warning and continues startup.
func StartupTerminalCleanup(ctx context.Context, tr tracker.Tracker, terminalStates []string, removeWorkspace func(string) error) {
	issues, err := tr.FetchIssuesByStates(ctx, terminalStates)
	if err != nil {
		slog.Warn("startup: terminal workspace cleanup fetch failed, continuing", "error", err)
		return
	}
	for _, issue := range issues {
		if err := removeWorkspace(issue.Identifier); err != nil {
			slog.Warn("startup: failed to remove workspace",
				"identifier", issue.Identifier, "error", err)
		}
	}
}
