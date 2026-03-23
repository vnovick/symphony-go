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
)

// OrchestratorEvent is sent over the event channel to the orchestrator loop.
type OrchestratorEvent struct { //nolint:revive
	Type       EventType
	IssueID    string // tracker UUID (e.g. "abc123"); used by WorkerExited/Update events
	Identifier string // human identifier (e.g. "ENG-1"); used by ForceReanalyze events
	RunEntry   *RunEntry
	RetryEntry *RetryEntry
	Error      error
}

// TerminalReason classifies why a worker stopped.
type TerminalReason string

// Terminal reason constants for RunEntry.TerminalReason.
const (
	TerminalSucceeded                TerminalReason = "succeeded"
	TerminalFailed                   TerminalReason = "failed"
	TerminalCanceledByReconciliation TerminalReason = "canceled_by_reconciliation"
	TerminalCanceledByUser           TerminalReason = "canceled_by_user"
	// TerminalSkippedOpenPR is set when a worker detects an existing open PR and
	// skips the agent run. The issue is auto-paused so it isn't re-dispatched
	// every poll cycle until the PR is merged or the user resumes it.
	TerminalSkippedOpenPR TerminalReason = "skipped_open_pr"
)

// RunEntry tracks a live agent worker goroutine.
type RunEntry struct {
	Issue          domain.Issue
	SessionID      string
	WorkerHost     string // SSH host used for this worker, empty = local
	Backend        string // e.g. "claude", "codex", or "" when unknown
	Kind           string // "worker" (default) | "reviewer"
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
	Status       string // "succeeded" | "failed" | "cancelled"
	WorkerHost   string
	Backend      string
	SessionID    string
	// ProjectKey scopes this run to a specific project so that a shared
	// history file does not leak runs across projects. Format: "<kind>:<slug>".
	// Empty string means "unscoped" (legacy entries written before this field
	// was added); these are retained so existing history is not silently dropped.
	ProjectKey string
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
}

// NewState initialises a State from a config snapshot.
func NewState(cfg *config.Config) State {
	return State{
		PollIntervalMs:        cfg.Polling.IntervalMs,
		MaxConcurrentAgents:   cfg.Agent.MaxConcurrentAgents,
		Running:               make(map[string]*RunEntry),
		Claimed:               make(map[string]struct{}),
		RetryAttempts:         make(map[string]*RetryEntry),
		PausedIdentifiers:     make(map[string]string),
		IssueProfiles:         make(map[string]string),
		PausedOpenPRs:         make(map[string]string),
		ForceReanalyze:        make(map[string]struct{}),
		PrevActiveIdentifiers: make(map[string]struct{}),
		DiscardingIdentifiers: make(map[string]struct{}),
	}
}
