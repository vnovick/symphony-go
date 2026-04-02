package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/vnovick/symphony-go/internal/agent"
	"github.com/vnovick/symphony-go/internal/domain"
	"github.com/vnovick/symphony-go/internal/prdetector"
	"github.com/vnovick/symphony-go/internal/prompt"
	"github.com/vnovick/symphony-go/internal/workspace"
)

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
	if o.cfg.Tracker.CompletionState != "" {
		allStates = append(allStates, o.cfg.Tracker.CompletionState)
	}
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
	o.reviewerWg.Add(1)
	go o.runReviewerWorker(ctx, *found)
	return nil
}

// runReviewerWorker runs a single reviewer agent session for the given issue.
// Unlike runWorker it does not register in the orchestrator state machine or
// attempt retries — it is a fire-and-forget background job.
func (o *Orchestrator) runReviewerWorker(ctx context.Context, issue domain.Issue) {
	defer o.reviewerWg.Done()
	startedAt := time.Now()
	workerLog := &bufLogger{
		base:       slog.With("issue_id", issue.ID, "issue_identifier", issue.Identifier, "kind", "reviewer"),
		buf:        o.logBuf,
		identifier: issue.Identifier,
	}
	workerLog.Info("worker: reviewer started", "identifier", issue.Identifier)
	if o.logBuf != nil {
		o.logBuf.Add(issue.Identifier, makeBufLine("INFO", fmt.Sprintf("worker: AI reviewer dispatched for %s", issue.Identifier)))
	}

	// Detect open PR to use the PR branch for the workspace (same logic as runWorker).
	branchName := workspace.ResolveWorktreeBranch(issue.BranchName, issue.Identifier)
	if prCtx, _ := prdetector.Detect(ctx, issue); prCtx != nil && prCtx.Branch != "" {
		branchName = prCtx.Branch
		slog.Info("reviewer: open PR detected, using PR branch",
			"issue_identifier", issue.Identifier, "branch", prCtx.Branch, "pr_url", prCtx.URL)
	}

	// Workspace: use the same workspace as the issue so the reviewer can read the PR branch.
	wsPath := ""
	if o.workspace != nil {
		ws, err := o.workspace.EnsureWorkspace(ctx, issue.Identifier, branchName)
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

	// Snapshot mutable cfg fields under cfgMu to avoid data races with HTTP
	// handler goroutines — mirroring the pattern in runWorker (GO-R10-4).
	o.cfgMu.RLock()
	reviewerPrompt := o.cfg.Agent.ReviewerPrompt
	reviewerCommand := o.cfg.Agent.Command
	reviewerBackend := o.cfg.Agent.Backend
	reviewerReadTimeoutMs := o.cfg.Agent.ReadTimeoutMs
	reviewerTurnTimeoutMs := o.cfg.Agent.TurnTimeoutMs
	reviewerSSHHosts := append([]string{}, o.cfg.Agent.SSHHosts...)
	dispatchStrategy := o.cfg.Agent.DispatchStrategy
	profiles := o.cfg.Agent.Profiles
	o.cfgMu.RUnlock()

	// Resolve per-issue profile overrides — same logic as dispatch() in
	// event_loop.go so the reviewer runs with the same backend as the worker.
	o.issueProfilesMu.Lock()
	profileName := o.issueProfiles[issue.Identifier]
	o.issueProfilesMu.Unlock()
	if profileName != "" {
		if profile, ok := profiles[profileName]; ok {
			if profile.Command != "" {
				reviewerCommand = profile.Command
			}
			if profile.Backend != "" {
				reviewerBackend = profile.Backend
			}
		}
	}
	// Resolve the final backend and embed the hint in the command so
	// MultiRunner dispatches to the correct runner (same as event_loop.go).
	resolvedBackend := agent.BackendFromCommand(reviewerCommand)
	if reviewerBackend != "" {
		resolvedBackend = reviewerBackend
		reviewerCommand = agent.CommandWithBackendHint(reviewerCommand, reviewerBackend)
	}

	renderedPrompt, err := prompt.Render(reviewerPrompt, issue, nil)
	if err != nil {
		workerLog.Warn("reviewer: prompt render failed", "error", err)
		return
	}

	// Apply the configured dispatch strategy for SSH host selection, matching
	// the logic in event_loop.go:dispatch(). The reviewer goroutine cannot
	// safely access o.sshHostIdx (owned by the event loop), so round-robin
	// uses a random offset for fair distribution.
	workerHost := ""
	if len(reviewerSSHHosts) > 0 {
		if dispatchStrategy == "least-loaded" {
			snap := o.Snapshot()
			workerHost = selectLeastLoadedHost(reviewerSSHHosts, snap.Running)
		} else {
			workerHost = reviewerSSHHosts[rand.IntN(len(reviewerSSHHosts))]
		}
	}

	const maxReviewerAttempts = 2
	var runErr error
	var result agent.TurnResult
	for attempt := range maxReviewerAttempts {
		result, runErr = o.runner.RunTurn(ctx, workerLog, nil, nil, renderedPrompt, wsPath,
			reviewerCommand, workerHost, "", reviewerReadTimeoutMs, reviewerTurnTimeoutMs)
		if runErr == nil && !result.Failed {
			break
		}
		workerLog.Warn("reviewer: agent turn failed", "attempt", attempt+1, "error", runErr, "failure", result.FailureText)
		if attempt+1 < maxReviewerAttempts {
			workerLog.Info("reviewer: retrying", "attempt", attempt+2)
		}
	}

	if runErr != nil || result.Failed {
		msg := "worker: reviewer failed after retries"
		workerLog.Warn(msg, "error", runErr, "failure", result.FailureText)
		if o.logBuf != nil {
			o.logBuf.Add(issue.Identifier, makeBufLine("WARN", msg))
		}
		// Record failure in history.
		o.historyMu.RLock()
		projectKey := o.historyKey
		o.historyMu.RUnlock()
		finishedAt := time.Now()
		select {
		case o.events <- OrchestratorEvent{
			Type: EventReviewerCompleted,
			CompletedRun: &CompletedRun{
				Identifier:   issue.Identifier,
				Title:        issue.Title,
				StartedAt:    startedAt,
				FinishedAt:   finishedAt,
				ElapsedMs:    finishedAt.Sub(startedAt).Milliseconds(),
				TotalTokens:  result.TotalTokens,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
				TurnCount:    1,
				WorkerHost:   workerHost,
				Backend:      resolvedBackend,
				Status:       "failed",
				ProjectKey:   projectKey,
				AppSessionID: o.appSessionID,
			},
		}:
		default:
			slog.Warn("reviewer: history channel full, run not recorded", "identifier", issue.Identifier)
		}
		return
	}

	workerLog.Info("worker: reviewer completed", "identifier", issue.Identifier, "tokens", result.TotalTokens)
	if o.logBuf != nil {
		o.logBuf.Add(issue.Identifier, makeBufLine("INFO", "worker: AI reviewer completed"))
	}
	// Record reviewer completion in history so it appears in the run-history ring
	// buffer alongside regular worker runs. Route through the event loop to
	// preserve the single-writer invariant on completedRuns.
	o.historyMu.RLock()
	projectKey := o.historyKey
	o.historyMu.RUnlock()
	finishedAt := time.Now()
	select {
	case o.events <- OrchestratorEvent{
		Type: EventReviewerCompleted,
		CompletedRun: &CompletedRun{
			Identifier:   issue.Identifier,
			Title:        issue.Title,
			StartedAt:    startedAt,
			FinishedAt:   finishedAt,
			ElapsedMs:    finishedAt.Sub(startedAt).Milliseconds(),
			TotalTokens:  result.TotalTokens,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
			TurnCount:    1, // reviewer runs one turn (up to maxReviewerAttempts retries)
			WorkerHost:   workerHost,
			Backend:      resolvedBackend,
			Status:       "succeeded",
			ProjectKey:   projectKey,
			AppSessionID: o.appSessionID,
		},
	}:
	default:
		// Channel full: history lost. Non-critical.
		slog.Warn("reviewer: history channel full, run not recorded", "identifier", issue.Identifier)
	}
}
