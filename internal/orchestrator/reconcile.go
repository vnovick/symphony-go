package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/logbuffer"
	"github.com/vnovick/symphony-go/internal/tracker"
)

// ReconcileStalls checks all running sessions for stall timeout violations.
// If stall_timeout_ms <= 0, stall detection is disabled and this is a no-op.
// The optional logBuf receives a stall-warning entry before the worker is killed.
func ReconcileStalls(state State, cfg *config.Config, now time.Time, events chan OrchestratorEvent, logBuf ...*logbuffer.Buffer) State {
	if cfg.Agent.StallTimeoutMs <= 0 {
		return state
	}
	stallDur := time.Duration(cfg.Agent.StallTimeoutMs) * time.Millisecond

	var buf *logbuffer.Buffer
	if len(logBuf) > 0 {
		buf = logBuf[0]
	}

	for id, entry := range state.Running {
		var elapsed time.Duration
		if entry.LastEventAt != nil {
			elapsed = now.Sub(*entry.LastEventAt)
		} else {
			elapsed = now.Sub(entry.StartedAt)
		}
		if elapsed > stallDur {
			slog.Warn("stall detected: killing worker",
				"issue_id", id,
				"issue_identifier", entry.Issue.Identifier,
				"elapsed_ms", elapsed.Milliseconds(),
			)
			// Emit stall warning to per-issue log buffer before killing (#6).
			if buf != nil {
				msg := fmt.Sprintf("worker: ⚠ stall detected — no output for %.0fs, killing worker", elapsed.Seconds())
				buf.Add(entry.Issue.Identifier, makeBufLine("WARN", msg))
			}
			if entry.WorkerCancel != nil {
				entry.WorkerCancel()
			}
			delete(state.Running, id)
			delete(state.Claimed, id)
			state = ScheduleRetry(state, id, 1, entry.Issue.Identifier, "stall_timeout", now, 1000)
			// Non-blocking send: state is already updated inline; the event loop
			// will persist the snapshot after onTick returns regardless of whether
			// this notification is received.
			select {
			case events <- OrchestratorEvent{Type: EventWorkerExited, IssueID: id}:
			default:
			}
		}
	}
	return state
}

// ReconcileTrackerStates fetches current states for all running issues and
// reconciles: terminal→cleanup, active→update snapshot, neither→stop no cleanup.
// If the fetch fails, workers are kept and the error is logged.
// The optional logBuf receives per-issue explanatory messages when workers are stopped.
func ReconcileTrackerStates(ctx context.Context, state State, cfg *config.Config, tr tracker.Tracker, events chan OrchestratorEvent, logBuf ...*logbuffer.Buffer) State {
	var buf *logbuffer.Buffer
	if len(logBuf) > 0 {
		buf = logBuf[0]
	}
	if len(state.Running) == 0 {
		return state
	}

	ids := make([]string, 0, len(state.Running))
	for id := range state.Running {
		ids = append(ids, id)
	}

	refreshed, err := tr.FetchIssueStatesByIDs(ctx, ids)
	if err != nil {
		slog.Warn("reconciliation: tracker refresh failed, keeping workers", "error", err)
		return state
	}

	byID := make(map[string]string, len(refreshed))
	for _, issue := range refreshed {
		byID[issue.ID] = issue.State
	}

	now := time.Now()
	for id, entry := range state.Running {
		refreshedState, found := byID[id]
		if !found {
			slog.Info("reconciliation: issue not found in tracker, stopping worker",
				"issue_id", id, "issue_identifier", entry.Issue.Identifier)
			if buf != nil {
				buf.Add(entry.Issue.Identifier, makeBufLine("WARN", "worker: ⚠ issue no longer found in tracker — worker stopped"))
			}
			if entry.WorkerCancel != nil {
				entry.WorkerCancel()
			}
			delete(state.Running, id)
			delete(state.Claimed, id)
			select {
			case events <- OrchestratorEvent{Type: EventWorkerExited, IssueID: id}:
			default:
			}
			continue
		}

		if isTermState(refreshedState, cfg) {
			slog.Info("reconciliation: terminal state, stopping worker",
				"issue_id", id, "issue_identifier", entry.Issue.Identifier, "state", refreshedState)
			if buf != nil {
				buf.Add(entry.Issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: issue moved to terminal state %q — worker stopped", refreshedState)))
			}
			if entry.WorkerCancel != nil {
				entry.WorkerCancel()
			}
			delete(state.Running, id)
			delete(state.Claimed, id)
			select {
			case events <- OrchestratorEvent{
				Type:    EventWorkerExited,
				IssueID: id,
				RunEntry: &RunEntry{
					Issue:          entry.Issue,
					TerminalReason: TerminalCanceledByReconciliation,
				},
			}:
			default:
			}
		} else if isActState(refreshedState, cfg) {
			entry.Issue.State = refreshedState
			entry.LastEventAt = &now
		} else {
			slog.Info("reconciliation: non-active state, stopping worker without cleanup",
				"issue_id", id, "issue_identifier", entry.Issue.Identifier, "state", refreshedState)
			if buf != nil {
				buf.Add(entry.Issue.Identifier, makeBufLine("WARN", fmt.Sprintf("worker: ⚠ issue state changed to %q (not in active_states) — worker stopped", refreshedState)))
			}
			if entry.WorkerCancel != nil {
				entry.WorkerCancel()
			}
			delete(state.Running, id)
			delete(state.Claimed, id)
			select {
			case events <- OrchestratorEvent{Type: EventWorkerExited, IssueID: id}:
			default:
			}
		}
	}
	return state
}

func isTermState(s string, cfg *config.Config) bool {
	for _, t := range cfg.Tracker.TerminalStates {
		if strings.EqualFold(s, t) {
			return true
		}
	}
	return false
}

func isActState(s string, cfg *config.Config) bool {
	for _, a := range cfg.Tracker.ActiveStates {
		if strings.EqualFold(s, a) {
			return true
		}
	}
	return false
}
