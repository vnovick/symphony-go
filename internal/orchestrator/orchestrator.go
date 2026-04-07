package orchestrator

import (
	"context"
	"log/slog"
	"maps"
	"os"
	"sync"
	"sync/atomic"

	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/logbuffer"
	"github.com/vnovick/itervox/internal/tracker"
	"github.com/vnovick/itervox/internal/workspace"
)

// maxWorkersCap is the absolute upper bound on MaxConcurrentAgents.
// Time-based worker constants (hookFallbackTimeout, postRunTimeout,
// maxTransitionAttempts) are defined in worker.go alongside their call sites.
const maxWorkersCap = 50

// Orchestrator is the single-goroutine state machine that owns all dispatch state.
type Orchestrator struct {
	// DryRun disables actual agent execution: issues are claimed but no worker
	// subprocess is started. Set ITERVOX_DRY_RUN=1 or assign before calling Run.
	DryRun bool

	cfg       *config.Config
	tracker   tracker.Tracker
	runner    agent.Runner
	workspace workspace.Provider // nil is safe — workspace ops skipped (useful in tests)
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

	// cfgMu guards cfg fields mutated at runtime from HTTP handler goroutines:
	// cfg.Agent.AgentMode, cfg.Agent.MaxConcurrentAgents, cfg.Agent.Profiles,
	// cfg.Tracker.ActiveStates, cfg.Tracker.TerminalStates, cfg.Tracker.CompletionState,
	// cfg.Workspace.AutoClearWorkspace, cfg.Agent.SSHHosts, cfg.Agent.DispatchStrategy,
	// cfg.Agent.ReviewerProfile, cfg.Agent.AutoReview.
	// All other cfg fields are read-only after startup and need no lock.
	cfgMu sync.RWMutex

	// sshHostDescs maps SSH host address → optional human label.
	// Managed at runtime via AddSSHHostCfg / RemoveSSHHostCfg.
	// Protected by cfgMu alongside cfg.Agent.SSHHosts.
	sshHostDescs map[string]string

	// historyMu guards completedRuns, historyFile, and historyKey only.
	historyMu     sync.RWMutex
	completedRuns []CompletedRun
	historyFile   string // optional path for persisting completedRuns to disk
	historyKey    string // project key used to scope history entries; format "<kind>:<slug>"

	// pausedMu guards pausedFile, which is an unrelated concern from history.
	pausedMu   sync.RWMutex
	pausedFile string // optional path for persisting PausedIdentifiers across restarts

	// inputRequiredMu guards inputRequiredFile.
	inputRequiredMu   sync.RWMutex
	inputRequiredFile string // optional path for persisting InputRequiredIssues across restarts

	// workerCancelsMu guards workerCancels, which is written by dispatch (event
	// loop goroutine) and read by cancelRunningWorker (any goroutine).
	// This is separate from lastSnap.Running because snapshot copies intentionally
	// omit WorkerCancel to avoid sharing cancel funcs across goroutines unsafely.
	workerCancelsMu sync.Mutex
	workerCancels   map[string]context.CancelFunc // identifier → cancel func

	// userCancelledMu guards userCancelledIDs, which is written by CancelIssue
	// (any goroutine) and read by handleEvent (event loop goroutine).
	userCancelledMu  sync.Mutex
	userCancelledIDs map[string]struct{} // keyed by identifier (e.g. "TIPRD-25")

	// userTerminatedMu guards userTerminatedIDs, which is written by TerminateIssue
	// (any goroutine) and read by handleEvent (event loop goroutine).
	userTerminatedMu  sync.Mutex
	userTerminatedIDs map[string]struct{} // like userCancelledIDs but releases claim without pausing

	// issueProfilesMu guards issueProfiles, which is written by SetIssueProfile
	// (any goroutine) and read by dispatch (event loop goroutine) and Snapshot.
	// RWMutex allows concurrent Snapshot() calls without serialising each other.
	issueProfilesMu sync.RWMutex
	issueProfiles   map[string]string // identifier → profile name

	// issueBackendsMu guards issueBackends, which is written by SetIssueBackend
	// (any goroutine) and read by dispatch (event loop goroutine) and Snapshot.
	issueBackendsMu sync.RWMutex
	issueBackends   map[string]string // identifier → "claude"|"codex"

	// agentLogDir, when non-empty, is passed to RunTurn as CLAUDE_CODE_LOG_DIR
	// so Claude Code writes full session logs (including sub-agents) to disk.
	// Set via SetAgentLogDir before calling Run.
	agentLogDir string

	// appSessionID is a unique ID for this daemon invocation, used to group
	// all history entries produced during a single run of the binary.
	// Set via SetAppSessionID before calling Run.
	appSessionID string

	// autoClearWg tracks in-flight auto-clear workspace goroutines so Run can
	// wait for them before returning.
	autoClearWg sync.WaitGroup

	// discardWg tracks in-flight asyncDiscardAndTransition / asyncDiscardAndTransitionTo
	// goroutines so Run can wait for them before returning.
	discardWg sync.WaitGroup

	// runCtx is the context passed to Run. Stored atomically so DispatchReviewer
	// can read it safely from any goroutine without a mutex.
	runCtx atomic.Pointer[context.Context]

	// started is set to true at the beginning of Run. It guards SetHistoryFile
	// and SetHistoryKey: calling either after Run starts is a programming error
	// (those fields are only read under historyMu from the event-loop goroutine,
	// and loadHistoryFromDisk has already consumed them by the time Run returns).
	started atomic.Bool
}

// New constructs an Orchestrator ready to Run. wm may be nil (workspace ops skipped).
func New(cfg *config.Config, tr tracker.Tracker, runner agent.Runner, wm workspace.Provider) *Orchestrator {
	return &Orchestrator{
		cfg:               cfg,
		tracker:           tr,
		runner:            runner,
		workspace:         wm,
		events:            make(chan OrchestratorEvent, 64),
		refresh:           make(chan struct{}, 1),
		workerCancels:     make(map[string]context.CancelFunc),
		userCancelledIDs:  make(map[string]struct{}),
		userTerminatedIDs: make(map[string]struct{}),
		issueProfiles:     make(map[string]string),
		issueBackends:     make(map[string]string),
		sshHostDescs:      make(map[string]string),
	}
}

// SetAgentLogDir configures the directory where agent session logs are written
// via CLAUDE_CODE_LOG_DIR. Per-issue logs are stored in {dir}/{identifier}/.
// Must be called before Run.
func (o *Orchestrator) SetAgentLogDir(dir string) {
	o.agentLogDir = dir
}

// SetAppSessionID sets the unique ID for this daemon invocation.
// Must be called before Run.
func (o *Orchestrator) SetAppSessionID(id string) {
	o.appSessionID = id
}

// AgentLogDir returns the configured agent log directory (empty = disabled).
func (o *Orchestrator) AgentLogDir() string {
	return o.agentLogDir
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
// The value is clamped to [1, maxWorkersCap]. Safe to call from any goroutine.
func (o *Orchestrator) SetMaxWorkers(n int) {
	if n < 1 {
		n = 1
	}
	if n > maxWorkersCap {
		n = maxWorkersCap
	}
	o.cfgMu.Lock()
	o.cfg.Agent.MaxConcurrentAgents = n
	o.cfgMu.Unlock()
	slog.Info("orchestrator: max workers updated", "max_concurrent_agents", n)
	if o.OnStateChange != nil {
		o.OnStateChange()
	}
}

// BumpMaxWorkers atomically applies a delta to MaxConcurrentAgents under cfgMu,
// clamping the result to [1, maxWorkersCap]. Returns the new value.
// Safe to call from any goroutine.
func (o *Orchestrator) BumpMaxWorkers(delta int) int {
	o.cfgMu.Lock()
	next := o.cfg.Agent.MaxConcurrentAgents + delta
	if next < 1 {
		next = 1
	}
	if next > maxWorkersCap {
		next = maxWorkersCap
	}
	o.cfg.Agent.MaxConcurrentAgents = next
	o.cfgMu.Unlock()
	slog.Info("orchestrator: max workers bumped", "delta", delta, "max_concurrent_agents", next)
	if o.OnStateChange != nil {
		o.OnStateChange()
	}
	return next
}

// MaxWorkers returns the current maximum concurrent agents setting.
// Safe to call from any goroutine.
func (o *Orchestrator) MaxWorkers() int {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
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

// SetAutoClearWorkspaceCfg toggles automatic workspace removal after a task succeeds.
// Safe to call from any goroutine.
func (o *Orchestrator) SetAutoClearWorkspaceCfg(enabled bool) {
	o.cfgMu.Lock()
	o.cfg.Workspace.AutoClearWorkspace = enabled
	o.cfgMu.Unlock()
}

// ClearHistory wipes the in-memory completed-run ring buffer and deletes the
// on-disk history file. Safe to call from any goroutine.
func (o *Orchestrator) ClearHistory() {
	o.historyMu.Lock()
	o.completedRuns = nil
	path := o.historyFile
	o.historyMu.Unlock()
	if path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			slog.Warn("orchestrator: failed to remove history file", "path", path, "error", err)
		}
	}
}

// AutoClearWorkspaceCfg returns the current auto-clear workspace setting.
// Safe to call from any goroutine.
func (o *Orchestrator) AutoClearWorkspaceCfg() bool {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	return o.cfg.Workspace.AutoClearWorkspace
}

func (o *Orchestrator) SetInlineInputCfg(enabled bool) {
	o.cfgMu.Lock()
	o.cfg.Agent.InlineInput = enabled
	o.cfgMu.Unlock()
}

func (o *Orchestrator) InlineInputCfg() bool {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	return o.cfg.Agent.InlineInput
}

// AvailableModelsCfg returns the available models from the config.
// Read-only after startup — no lock needed.
func (o *Orchestrator) AvailableModelsCfg() map[string][]config.ModelOption {
	return o.cfg.Agent.AvailableModels
}

// ReviewerCfg returns the reviewer profile name and auto-review flag under cfgMu.
func (o *Orchestrator) ReviewerCfg() (profile string, autoReview bool) {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	return o.cfg.Agent.ReviewerProfile, o.cfg.Agent.AutoReview
}

// SetReviewerCfg sets the reviewer profile name and auto-review flag under cfgMu.
func (o *Orchestrator) SetReviewerCfg(profile string, autoReview bool) {
	o.cfgMu.Lock()
	o.cfg.Agent.ReviewerProfile = profile
	o.cfg.Agent.AutoReview = autoReview
	o.cfgMu.Unlock()
}

// ProfilesCfg returns a shallow copy of cfg.Agent.Profiles under cfgMu.
// Safe to call from any goroutine.
func (o *Orchestrator) ProfilesCfg() map[string]config.AgentProfile {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	cp := make(map[string]config.AgentProfile, len(o.cfg.Agent.Profiles))
	maps.Copy(cp, o.cfg.Agent.Profiles)
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

// SSHHostsCfg returns a copy of the current SSH host list and descriptions map.
// Safe to call from any goroutine.
func (o *Orchestrator) SSHHostsCfg() (hosts []string, descs map[string]string) {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	return append([]string{}, o.cfg.Agent.SSHHosts...), maps.Clone(o.sshHostDescs)
}

// AddSSHHostCfg adds a host to the SSH pool at runtime. If the host already
// exists, its description is updated. Safe to call from any goroutine.
func (o *Orchestrator) AddSSHHostCfg(host, description string) {
	o.cfgMu.Lock()
	defer o.cfgMu.Unlock()
	for _, h := range o.cfg.Agent.SSHHosts {
		if h == host {
			o.sshHostDescs[host] = description
			return
		}
	}
	o.cfg.Agent.SSHHosts = append(o.cfg.Agent.SSHHosts, host)
	o.sshHostDescs[host] = description
}

// RemoveSSHHostCfg removes a host from the SSH pool at runtime.
// Safe to call from any goroutine.
func (o *Orchestrator) RemoveSSHHostCfg(host string) {
	o.cfgMu.Lock()
	defer o.cfgMu.Unlock()
	result := o.cfg.Agent.SSHHosts[:0:0]
	for _, h := range o.cfg.Agent.SSHHosts {
		if h != host {
			result = append(result, h)
		}
	}
	o.cfg.Agent.SSHHosts = result
	delete(o.sshHostDescs, host)
}

// DispatchStrategyCfg returns the active dispatch strategy.
// Safe to call from any goroutine.
func (o *Orchestrator) DispatchStrategyCfg() string {
	o.cfgMu.RLock()
	defer o.cfgMu.RUnlock()
	return o.cfg.Agent.DispatchStrategy
}

// SetDispatchStrategyCfg sets the dispatch strategy at runtime.
// Safe to call from any goroutine.
func (o *Orchestrator) SetDispatchStrategyCfg(strategy string) {
	o.cfgMu.Lock()
	defer o.cfgMu.Unlock()
	o.cfg.Agent.DispatchStrategy = strategy
}

// SetLogBuffer attaches a log buffer so worker output is captured per-identifier
// for display in the interactive TUI.
func (o *Orchestrator) SetLogBuffer(buf *logbuffer.Buffer) {
	o.logBuf = buf
}
