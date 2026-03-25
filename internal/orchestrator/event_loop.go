package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vnovick/symphony-go/internal/agent"
	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/domain"
	"github.com/vnovick/symphony-go/internal/tracker"
	"github.com/vnovick/symphony-go/internal/workspace"
)

// Run executes the orchestrator event loop until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) error {
	o.started.Store(true) // guard SetHistoryFile / SetHistoryKey against post-Run calls
	o.runCtx.Store(&ctx)
	o.loadHistoryFromDisk()
	state := NewState(o.cfg)
	state = o.loadPausedFromDisk(state)
	tick := time.NewTimer(0)
	defer tick.Stop()

	var loopErr error
	for {
		select {
		case <-ctx.Done():
			loopErr = ctx.Err()
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
		if loopErr != nil {
			break
		}
	}
	o.reviewerWg.Wait()
	o.autoClearWg.Wait()
	return loopErr
}

func (o *Orchestrator) onTick(ctx context.Context, state State) State {
	now := time.Now()

	// Snapshot runtime-mutable cfg fields into the event-loop State so that
	// AvailableSlots and dispatch helpers read a stable, lock-free copy for
	// the entire tick (no need to hold cfgMu inside the hot dispatch path).
	o.cfgMu.RLock()
	state.MaxConcurrentAgents = o.cfg.Agent.MaxConcurrentAgents
	state.ActiveStates = append([]string{}, o.cfg.Tracker.ActiveStates...)
	state.TerminalStates = append([]string{}, o.cfg.Tracker.TerminalStates...)
	o.cfgMu.RUnlock()

	// 1. Fire any retries whose DueAt has passed.
	state = o.fireRetries(ctx, state, now)

	// 2. Stall detection and tracker-state reconciliation.
	state = ReconcileStalls(state, o.cfg, now, o.events, o.logBuf)
	state = ReconcileTrackerStates(ctx, state, o.tracker, o.events, o.logBuf)

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

	slots := AvailableSlots(state)
	slog.Debug("orchestrator: tick",
		"fetched", len(issues),
		"running", len(state.Running),
		"slots", slots,
		"max_concurrent", o.cfg.Agent.MaxConcurrentAgents,
	)

	dispatched := 0
	for _, issue := range SortForDispatch(issues) {
		if AvailableSlots(state) <= 0 {
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
			"slots_remaining", AvailableSlots(state),
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

		if len(refreshed) == 0 || !isActiveState(refreshed[0].State, state) {
			slog.Info("retry: issue no longer active, releasing claim", "issue_id", issueID)
			state = CancelRetry(state, issueID)
			continue
		}

		if AvailableSlots(state) <= 0 {
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

	// SSH host selection. Empty string = run locally.
	// SSHHosts and DispatchStrategy are now runtime-mutable so they must be
	// read under cfgMu, same as the other runtime-mutable cfg fields below.
	o.cfgMu.RLock()
	hosts := append([]string{}, o.cfg.Agent.SSHHosts...)
	dispatchStrategy := o.cfg.Agent.DispatchStrategy
	agentCommand := o.cfg.Agent.Command
	defaultBackend := o.cfg.Agent.Backend
	o.cfgMu.RUnlock()

	workerHost := ""
	if len(hosts) > 0 {
		if dispatchStrategy == "least-loaded" {
			hostCount := make(map[string]int, len(hosts))
			for _, h := range hosts {
				hostCount[h] = 0
			}
			for _, entry := range state.Running {
				if _, ok := hostCount[entry.WorkerHost]; ok {
					hostCount[entry.WorkerHost]++
				}
			}
			minCount := int(^uint(0) >> 1)
			for _, h := range hosts {
				if c := hostCount[h]; c < minCount {
					minCount = c
					workerHost = h
				}
			}
		} else {
			// Default: round-robin.
			workerHost = hosts[o.sshHostIdx%len(hosts)]
			o.sshHostIdx++
		}
	}
	runnerCommand := agentCommand
	backend := agent.BackendFromCommand(agentCommand)
	if defaultBackend != "" {
		backend = defaultBackend
		runnerCommand = agent.CommandWithBackendHint(agentCommand, defaultBackend)
	}
	o.issueProfilesMu.Lock()
	profileName := o.issueProfiles[issue.Identifier]
	o.issueProfilesMu.Unlock()
	if profileName != "" {
		o.cfgMu.RLock()
		profile, ok := o.cfg.Agent.Profiles[profileName]
		o.cfgMu.RUnlock()
		if ok && profile.Command != "" {
			agentCommand = profile.Command
			runnerCommand = agentCommand
			backend = agent.BackendFromCommand(agentCommand)
			if profile.Backend != "" {
				backend = profile.Backend
				runnerCommand = agent.CommandWithBackendHint(agentCommand, profile.Backend)
			}
			slog.Info("orchestrator: using profile command",
				"identifier", issue.Identifier, "profile", profileName, "command", agentCommand, "backend", backend)
		} else {
			slog.Warn("orchestrator: profile not found or has no command, using default",
				"identifier", issue.Identifier, "profile", profileName)
			profileName = "" // clear so the worker does not reference a missing profile
		}
	}

	if o.DryRun {
		workerCancel()
		slog.Info("orchestrator: [DRY-RUN] would dispatch agent",
			"identifier", issue.Identifier, "issue_id", issue.ID,
			"command", agentCommand, "worker_host", workerHost, "backend", backend)
		state.Claimed[issue.ID] = struct{}{} // claim so it doesn't re-dispatch this tick
		return state
	}

	state.Claimed[issue.ID] = struct{}{}
	state.Running[issue.ID] = &RunEntry{
		Issue:        issue,
		WorkerHost:   workerHost,
		Backend:      backend,
		StartedAt:    time.Now(),
		RetryAttempt: &attempt,
		WorkerCancel: workerCancel,
	}

	// Register the cancel func in the concurrent-safe map so CancelIssue (called
	// from HTTP handler goroutines) can reach it without going through the snapshot,
	// which intentionally omits WorkerCancel to avoid unsafe cross-goroutine sharing.
	o.workerCancelsMu.Lock()
	o.workerCancels[issue.Identifier] = workerCancel
	o.workerCancelsMu.Unlock()

	if o.OnDispatch != nil {
		o.OnDispatch(issue.ID)
	}

	// Note: we intentionally do NOT clear the log buffer here. Every log entry is
	// tagged with a per-run runLogID (set by runWorker), so the client can filter
	// by session ID to isolate each run's logs. Clearing here would destroy the
	// log history of cancelled/terminated runs before users can inspect them.

	go o.runWorker(workerCtx, issue, attempt, workerHost, runnerCommand, backend, profileName, skipPRCheck)
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
				o.cfgMu.RLock()
				// Copy the slices so the spawned goroutine does not alias the live
				// cfg arrays — a concurrent HTTP write could replace those slices
				// while the goroutine is still reading them (GO-R10-1).
				backlogStates := append([]string{}, o.cfg.Tracker.BacklogStates...)
				activeStates := append([]string{}, o.cfg.Tracker.ActiveStates...)
				o.cfgMu.RUnlock()
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
						// Use a fresh background context so this goroutine can finish
						// its tracker update and send EventDiscardComplete even when
						// Run() has already exited. Deriving from the run context
						// collapses the 15-second window to zero on shutdown, leaving
						// the identifier permanently stuck in DiscardingIdentifiers.
						updateCtx, updateCancel := context.WithTimeout(context.Background(), 15*time.Second)
						if err := o.tracker.UpdateIssueState(updateCtx, issueID, targetState); err != nil {
							slog.Warn("orchestrator: failed to transition discarded issue",
								"identifier", identifier, "target_state", targetState, "error", err)
						} else {
							slog.Info("orchestrator: discarded issue transitioned",
								"identifier", identifier, "state", targetState)
						}
						updateCancel()
						// The send gets its own window. The events channel holds 64 entries and
						// the event loop processes them continuously; 30s is ample even under load.
						sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer sendCancel()
						select {
						case o.events <- OrchestratorEvent{Type: EventDiscardComplete, Identifier: identifier}:
						case <-sendCtx.Done():
							slog.Warn("orchestrator: discard complete event lost, identifier may be stuck",
								"identifier", identifier)
						}
					}()
				}
			}
			if o.OnStateChange != nil {
				o.OnStateChange()
			}
		}

	case EventTerminateRunning:
		// Find the running worker and atomically mark it as user-terminated
		// before cancelling its context. Because this executes in the same
		// event-loop goroutine as EventWorkerExited, it is impossible for the
		// exit event to arrive and be processed between the state.Running check
		// and the userTerminatedIDs write — the TOCTOU window from GO-R5-3 is
		// gone. If the worker already exited naturally and EventWorkerExited was
		// queued before this event, state.Running will no longer contain the
		// entry and we take the no-op path below.
		for _, entry := range state.Running {
			if entry.Issue.Identifier == ev.Identifier && entry.WorkerCancel != nil {
				o.userTerminatedMu.Lock()
				o.userTerminatedIDs[ev.Identifier] = struct{}{}
				o.userTerminatedMu.Unlock()
				entry.WorkerCancel()
				slog.Info("orchestrator: running worker terminated by user",
					"identifier", ev.Identifier)
				return state
			}
		}
		// Worker already exited before this event was processed — natural exit
		// won the race; no flag to set, no cancel needed.
		slog.Debug("orchestrator: EventTerminateRunning — worker already exited",
			"identifier", ev.Identifier)

	case EventCancelRetry:
		// Remove the retry entry and claim, then pause the issue so it won't be
		// automatically re-dispatched until the user explicitly resumes it.
		state = CancelRetry(state, ev.IssueID)
		state.PausedIdentifiers[ev.Identifier] = ev.IssueID
		o.savePausedToDisk(copyStringMap(state.PausedIdentifiers))
		slog.Info("orchestrator: retry-queue issue cancelled and paused", "identifier", ev.Identifier)
		if o.OnStateChange != nil {
			o.OnStateChange()
		}

	case EventDiscardComplete:
		delete(state.DiscardingIdentifiers, ev.Identifier)
		slog.Info("orchestrator: discard complete, issue released", "identifier", ev.Identifier)

	case EventReviewerCompleted:
		if ev.CompletedRun != nil {
			o.addCompletedRun(*ev.CompletedRun)
		}

	case EventWorkerExited:
		// Capture the live entry before deletion so we can record history.
		liveEntry := state.Running[ev.IssueID]
		delete(state.Running, ev.IssueID)

		if ev.RunEntry == nil {
			// Emitted by ReconcileTrackerStates (not-found/non-active path);
			// claim and retry already managed by the reconcile function.
			return state
		}

		// Remove the cancel func from the concurrent-safe map now that the worker
		// has exited — CancelIssue will no longer find a cancel to invoke.
		o.workerCancelsMu.Lock()
		delete(o.workerCancels, ev.RunEntry.Issue.Identifier)
		o.workerCancelsMu.Unlock()

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
			delete(state.Claimed, ev.IssueID)
			slog.Info("orchestrator: issue terminated by user (claim released)",
				"issue_id", ev.IssueID, "identifier", issue.Identifier)
			o.recordHistory(liveEntry, issue, now, "cancelled")

			// Move the issue to backlog so the working-state label is cleared and
			// the issue is not immediately re-dispatched on the next poll cycle.
			// Uses the same pattern as EventTerminatePaused: block dispatch with
			// DiscardingIdentifiers until the async tracker update completes.
			o.cfgMu.RLock()
			backlogStates := append([]string{}, o.cfg.Tracker.BacklogStates...)
			activeStates := append([]string{}, o.cfg.Tracker.ActiveStates...)
			o.cfgMu.RUnlock()
			var targetState string
			if len(backlogStates) > 0 {
				targetState = backlogStates[0]
			} else if len(activeStates) > 0 {
				targetState = activeStates[0]
			}
			if targetState != "" && ev.IssueID != "" {
				state.DiscardingIdentifiers[issue.Identifier] = struct{}{}
				issueID := ev.IssueID
				identifier := issue.Identifier
				go func() {
					updateCtx, updateCancel := context.WithTimeout(context.Background(), 15*time.Second)
					if err := o.tracker.UpdateIssueState(updateCtx, issueID, targetState); err != nil {
						slog.Warn("orchestrator: failed to transition terminated issue",
							"identifier", identifier, "target_state", targetState, "error", err)
					} else {
						slog.Info("orchestrator: terminated issue transitioned",
							"identifier", identifier, "state", targetState)
					}
					updateCancel()
					sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer sendCancel()
					select {
					case o.events <- OrchestratorEvent{Type: EventDiscardComplete, Identifier: identifier}:
					case <-sendCtx.Done():
						slog.Warn("orchestrator: discard complete event lost, identifier may be stuck",
							"identifier", identifier)
					}
				}()
			}
			return state
		}

		switch ev.RunEntry.TerminalReason {
		case TerminalCanceledByReconciliation:
			// Reconcile already released the claim; just log.
			delete(state.Claimed, ev.IssueID)
			slog.Info("orchestrator: worker canceled by reconciliation",
				"issue_id", ev.IssueID, "issue_identifier", issue.Identifier)
			// Not recorded — the issue will be re-dispatched.

		case TerminalSucceeded:
			// Release the claim — the issue completed successfully.
			// Do NOT schedule a retry; successful completions must not appear in
			// the retry queue and must not cause infinite re-dispatch loops.
			delete(state.Claimed, ev.IssueID)
			slog.Info("orchestrator: worker succeeded, claim released",
				"issue_id", ev.IssueID, "issue_identifier", issue.Identifier)
			o.recordHistory(liveEntry, issue, now, "succeeded")
			// Auto-clear workspace if configured — removes the cloned directory
			// but leaves logs intact (they live under the logs dir, not here).
			o.cfgMu.RLock()
			autoClear := o.cfg.Workspace.AutoClearWorkspace
			o.cfgMu.RUnlock()
			if autoClear && o.workspace != nil {
				// Run in a goroutine — os.RemoveAll can be slow on large workspaces
				// and must not block the event loop (which would stall all workers).
				wm := o.workspace
				id := issue.Identifier
				// Use the actual worktree branch propagated via sendExitWithBranch.
				// PR-continuation runs use prCtx.Branch, which differs from
				// issue.BranchName; re-deriving the branch here would delete the
				// wrong branch and leak the PR branch permanently (GO-H2).
				bn := ev.RunEntry.BranchName
				if bn == "" {
					bn = workspace.ResolveWorktreeBranch(issue.BranchName, issue.Identifier)
				}
				o.autoClearWg.Add(1)
				go func() {
					defer o.autoClearWg.Done()
					rmCtx, rmCancel := context.WithTimeout(context.Background(), hookFallbackTimeout)
					defer rmCancel()
					if err := wm.RemoveWorkspace(rmCtx, id, bn); err != nil {
						slog.Warn("orchestrator: auto-clear workspace failed",
							"identifier", id, "error", err)
					} else {
						slog.Info("orchestrator: workspace auto-cleared",
							"identifier", id)
					}
				}()
			}

		case TerminalStalled:
			// ReconcileStalls already handled claim deletion and retry scheduling
			// inline. All we need to do here is record history so stall kills appear
			// in the run-history ring buffer. Use ev.RunEntry (not liveEntry, which
			// is nil because ReconcileStalls already deleted it from state.Running).
			o.recordHistory(ev.RunEntry, issue, now, "stalled")

		default: // TerminalFailed (and any other unhandled terminal reasons)
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
	o.historyMu.RLock()
	key := o.historyKey
	o.historyMu.RUnlock()
	run := CompletedRun{
		Identifier:   issue.Identifier,
		Title:        issue.Title,
		FinishedAt:   finishedAt,
		Status:       status,
		ProjectKey:   key,
		AppSessionID: o.appSessionID,
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
func buildSubAgentContext(profiles map[string]config.AgentProfile, activeProfile string, backend string) string {
	if len(profiles) == 0 {
		return ""
	}
	toolName := "Task"
	if backend == "codex" {
		toolName = "spawn_agent"
	}
	var b strings.Builder
	b.WriteString("## Available Sub-Agents\n\n")
	b.WriteString("You can spawn the following specialised sub-agents using the ")
	b.WriteString(toolName)
	b.WriteString(" tool:\n\n")
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
	b.WriteString("\nUse the ")
	b.WriteString(toolName)
	b.WriteString(" tool with the sub-agent description when you need specialised help.")
	return b.String()
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
