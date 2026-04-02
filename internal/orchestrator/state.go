package orchestrator

import (
	"context"
	"time"

	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/domain"
)

// EventType identifies the kind of OrchestratorEvent.
type EventType string

// OrchestratorEvent type constants sent over the events channel.
const (
	EventWorkerExited    EventType = "WorkerExited"
	EventWorkerUpdate    EventType = "WorkerUpdate"
	EventForceReanalyze  EventType = "ForceReanalyze"
	EventResumeIssue     EventType = "ResumeIssue"
	EventTerminatePaused EventType = "TerminatePaused"
	// EventDiscardComplete is sent by the UpdateIssueState goroutine (spawned by
	// EventTerminatePaused) once the label transition is confirmed. Until this
	// event is processed, the issue stays in DiscardingIdentifiers which blocks
	// dispatch — preventing the TUI's background TriggerPoll from re-picking the
	// issue before the "In Progress" label has been removed.
	EventDiscardComplete EventType = "DiscardComplete"
	// EventTerminateRunning is sent by TerminateIssue when the target issue has
	// a live worker. Processing it inside the event loop linearises the terminate
	// with EventWorkerExited, closing the TOCTOU window where a natural worker
	// exit races with a user-initiated cancel (GO-R5-3).
	EventTerminateRunning EventType = "TerminateRunning"
	// EventReviewerCompleted is sent by runReviewerWorker when it finishes
	// (success or failure) so the event loop can append a CompletedRun record
	// to the history ring buffer without requiring the reviewer goroutine to
	// call addCompletedRun directly (which would violate the single-writer invariant).
	EventReviewerCompleted EventType = "ReviewerCompleted"
	// EventCancelRetry is sent by CancelIssue when the target issue is in the
	// retry queue (no live worker). The event loop removes the retry entry,
	// releases the claim, and moves the issue to PausedIdentifiers.
	EventCancelRetry EventType = "CancelRetry"
	// EventProvideInput is sent by ProvideInput when the user provides a message
	// for an input-required issue. The event loop removes the entry from
	// InputRequiredIssues and dispatches a resumed worker.
	EventProvideInput EventType = "ProvideInput"
	// EventDismissInput is sent by DismissInput when the user dismisses an
	// input-required issue without providing input. Moves to PausedIdentifiers.
	EventDismissInput EventType = "DismissInput"
)

// OrchestratorEvent is sent over the event channel to the orchestrator loop.
type OrchestratorEvent struct { //nolint:revive
	Type         EventType
	IssueID      string // tracker UUID (e.g. "abc123"); used by WorkerExited/Update events
	Identifier   string // human identifier (e.g. "ENG-1"); used by ForceReanalyze events
	RunEntry     *RunEntry
	RetryEntry   *RetryEntry
	Error              error
	Message            string              // user-provided text for EventProvideInput
	CompletedRun       *CompletedRun       // used by EventReviewerCompleted
	InputRequiredEntry *InputRequiredEntry  // used by TerminalInputRequired
}

// TerminalReason classifies why a worker stopped.
type TerminalReason string

// Terminal reason constants for RunEntry.TerminalReason.
const (
	TerminalSucceeded                TerminalReason = "succeeded"
	TerminalFailed                   TerminalReason = "failed"
	TerminalCanceledByReconciliation TerminalReason = "canceled_by_reconciliation"
	// TerminalStalled is used when a worker is killed by stall detection.
	// Claim and retry management are handled inline by ReconcileStalls; the
	// event loop only records history when it sees this reason.
	TerminalStalled TerminalReason = "stalled"
	// TerminalInputRequired is used when the agent signals it needs human input
	// (permission prompt, missing API key, etc.). The issue is moved to the
	// InputRequiredIssues queue instead of being retried or marked as succeeded.
	TerminalInputRequired TerminalReason = "input_required"
)

// InputRequiredEntry holds context for an issue whose agent is blocked waiting
// for human input. Stored in State.InputRequiredIssues until the user provides
// input (via ProvideInput) or dismisses it (via DismissInput).
type InputRequiredEntry struct {
	IssueID     string
	Identifier  string
	SessionID   string    // for --resume
	Context     string    // what the agent was waiting for (from FailureText/ResultText)
	Backend     string    // which runner was used
	Command     string    // agent command (for resume on same runner)
	WorkerHost  string    // SSH host (for resume on same host)
	ProfileName string    // active profile
	QueuedAt    time.Time
}

// RunEntry tracks a live agent worker goroutine.
type RunEntry struct {
	Issue          domain.Issue
	SessionID      string
	WorkerHost     string // SSH host used for this worker, empty = local
	Backend        string // e.g. "claude", "codex", or "" when unknown
	Kind           string // "worker" (default) | "reviewer"
	BranchName     string // actual resolved branch used for the worktree (may differ from issue.BranchName when a PR branch was used)
	TerminalReason TerminalReason
	LastEventAt    *time.Time // when last EventWorkerUpdate was received
	LastMessage    string
	InputTokens    int
	OutputTokens   int
	TotalTokens    int
	TurnCount      int
	RetryAttempt   *int
	StartedAt      time.Time
	WorkerCancel   context.CancelFunc
}

// CompletedRun is a snapshot of a finished worker session, kept in the history ring buffer.
type CompletedRun struct {
	Identifier   string
	Title        string
	StartedAt    time.Time
	FinishedAt   time.Time
	ElapsedMs    int64
	TurnCount    int
	TotalTokens  int
	InputTokens  int
	OutputTokens int
	Status       string // "succeeded" | "failed" | "cancelled" | "stalled" | "input_required"
	WorkerHost   string
	Backend      string
	SessionID    string
	// ProjectKey scopes this run to a specific project so that a shared
	// history file does not leak runs across projects. Format: "<kind>:<slug>".
	// Empty string means "unscoped" (legacy entries written before this field
	// was added); these are retained so existing history is not silently dropped.
	ProjectKey   string
	AppSessionID string // daemon-invocation grouping key; empty for legacy entries
}

// RetryEntry represents a scheduled retry for an issue.
type RetryEntry struct {
	IssueID    string
	Identifier string
	Attempt    int
	DueAt      time.Time
	Error      *string
}

// State is the single in-memory authority for all orchestrator runtime data.
type State struct {
	PollIntervalMs      int
	MaxConcurrentAgents int
	// ActiveStates and TerminalStates are snapshotted from cfg at the start of
	// each tick under cfgMu so the event loop can compare issue states lock-free
	// throughout a tick. These are the cfg fields governed by cfgMu.
	ActiveStates   []string
	TerminalStates []string
	Running             map[string]*RunEntry
	Claimed             map[string]struct{}
	RetryAttempts       map[string]*RetryEntry
	// PausedIdentifiers tracks issues paused by user kill.
	// Key: identifier (e.g. "TIPRD-25"), Value: issue UUID (empty when loaded
	// from an old disk snapshot that predates UUID persistence).
	// Paused issues are not re-dispatched until explicitly resumed.
	PausedIdentifiers map[string]string
	// IssueProfiles maps issue identifier to an agent profile name override.
	// When set for an issue, the named profile's Command is used instead of
	// the default cfg.Agent.Command when dispatching that issue.
	IssueProfiles map[string]string
	// PausedOpenPRs tracks issues that were auto-paused because an open PR was detected.
	// Key: issue identifier, Value: open PR URL.
	PausedOpenPRs map[string]string
	// ForceReanalyze holds identifiers queued for forced PR re-analysis.
	// These bypass the "existing open PR = skip" guard on next dispatch.
	ForceReanalyze map[string]struct{}
	// PrevActiveIdentifiers is the set of issue identifiers that were fetched
	// as active on the previous tick. Used by the auto-resume guard to
	// distinguish "issue came back to active after being absent" (safe to
	// auto-resume) from "issue was already active when user paused it"
	// (must not auto-resume — wait until it leaves active and returns).
	PrevActiveIdentifiers map[string]struct{}
	// DiscardingIdentifiers holds identifiers of issues whose EventTerminatePaused
	// has been processed but whose UpdateIssueState goroutine has not yet
	// completed. Issues in this set are ineligible for dispatch, preventing the
	// TUI's background TriggerPoll from re-picking them before the label update
	// (e.g. removing "In Progress") finishes. Cleared by EventDiscardComplete.
	DiscardingIdentifiers map[string]struct{}
	// InputRequiredIssues tracks issues whose agent signalled that it needs
	// human input to continue. Key: identifier. These issues are not dispatched
	// until the user provides input or dismisses.
	InputRequiredIssues map[string]*InputRequiredEntry
	// InlineInputIssues is deprecated but kept for snapshot copy compatibility.
	// All input-required handling now uses InputRequiredIssues.
	InlineInputIssues map[string]*InlineInputEntry
}

// InlineInputEntry holds context for an input-required issue that was
// delegated to the tracker via a comment (inlineInput mode).
type InlineInputEntry struct {
	IssueID          string
	Identifier       string
	SessionID        string
	Context          string // agent's question
	Backend          string
	Command          string
	WorkerHost       string
	ProfileName      string
	PostedAt         time.Time
	LastCommentCount int // comment count at time of posting, to detect new ones
}

// NewState initialises a State from a config snapshot.
func NewState(cfg *config.Config) State {
	return State{
		PollIntervalMs:        cfg.Polling.IntervalMs,
		MaxConcurrentAgents:   cfg.Agent.MaxConcurrentAgents,
		ActiveStates:          append([]string{}, cfg.Tracker.ActiveStates...),
		TerminalStates:        append([]string{}, cfg.Tracker.TerminalStates...),
		Running:               make(map[string]*RunEntry),
		Claimed:               make(map[string]struct{}),
		RetryAttempts:         make(map[string]*RetryEntry),
		PausedIdentifiers:     make(map[string]string),
		IssueProfiles:         make(map[string]string),
		PausedOpenPRs:         make(map[string]string),
		ForceReanalyze:        make(map[string]struct{}),
		PrevActiveIdentifiers: make(map[string]struct{}),
		DiscardingIdentifiers: make(map[string]struct{}),
		InputRequiredIssues:   make(map[string]*InputRequiredEntry),
		InlineInputIssues:    make(map[string]*InlineInputEntry),
	}
}
