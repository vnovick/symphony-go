package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/prdetector"
	"github.com/vnovick/itervox/internal/prompt"
	"github.com/vnovick/itervox/internal/workspace"
)

// Worker-level timeout and retry constants.
const (
	// hookFallbackTimeout is used when no explicit hook timeout is configured.
	hookFallbackTimeout = 30 * time.Second
	// postRunTimeout bounds post-run actions: branch push, PR comment, state transition.
	postRunTimeout = 60 * time.Second
	// maxTransitionAttempts is how many times the worker retries moving the issue
	// to the completion state before giving up.
	maxTransitionAttempts = 4
)

// runWorker implements the full per-issue lifecycle: workspace, hooks, multi-turn loop.
// Runs in its own goroutine; communicates back only via o.events.
// workerHost is the SSH host to run the agent on; empty string means run locally.
// agentCommand is the resolved agent command to run (may differ from cfg.Agent.Command
// when a per-issue profile override is active).
// backend is the resolved runner backend for this worker after applying any
// explicit backend overrides.
// profileName is the active named profile for this issue (may be ""); used to
// exclude the current agent from its own sub-agent context in teams mode.
// skipPRCheck bypasses the open-PR guard (used when a forced re-analysis is requested).
// resumeSessionID, if non-empty, instructs the runner to use --resume <id> on
// turn 1 so the agent continues an existing session (set when an issue is
// resumed from manual pause and we have a captured session ID).
func (o *Orchestrator) runWorker(ctx context.Context, issue domain.Issue, attempt int, workerHost string, agentCommand string, backend string, profileName string, skipPRCheck bool, resumeSessionID string) {
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

	// Assign a run-level log session ID immediately so every log entry — including
	// early hook/worker messages written before the agent subprocess starts — shares
	// the same session ID. This ID is used purely for log correlation in the Timeline;
	// the real Claude Code session ID (claudeSessionID below) is kept separately for
	// --resume continuity within the same run.
	runLogID := generateRunID()
	// Notify the event loop right away so the Timeline run record has a session ID
	// from the start, before the first agent turn completes.
	select {
	case o.events <- OrchestratorEvent{
		Type:     EventWorkerUpdate,
		IssueID:  issue.ID,
		RunEntry: &RunEntry{SessionID: runLogID},
	}:
	default:
		slog.Debug("orchestrator: worker update event dropped (channel full)", "issue_id", issue.ID)
	}

	// --- Workspace ---
	wsPath := ""
	branchName := workspace.ResolveWorktreeBranch(issue.BranchName, issue.Identifier)

	// Detect open PR (best-effort). On success, use the PR branch so the worktree
	// checks out the existing branch instead of creating a new one.
	var prCtx *prdetector.PRContext
	var detectedPRURL string // PR URL discovered during this run (pre-existing or newly created)
	if !skipPRCheck {
		prCtx, _ = prdetector.Detect(ctx, issue)
		if prCtx != nil && prCtx.Branch == "" {
			// Treat a PR with no head branch as "not found" — an empty branch name
			// passed to EnsureWorkspace produces an unnamed worktree.
			slog.Warn("worker: open PR has empty branch, ignoring",
				"issue_identifier", issue.Identifier, "pr_url", prCtx.URL)
			prCtx = nil
		}
		if prCtx != nil {
			branchName = prCtx.Branch
			detectedPRURL = prCtx.URL
			slog.Info("worker: open PR detected, using PR branch",
				"issue_identifier", issue.Identifier, "branch", prCtx.Branch, "pr_url", prCtx.URL)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLine("INFO",
					fmt.Sprintf("worker: pr_context url=%s branch=%s", prCtx.URL, prCtx.Branch)))
			}
		}
	} else {
		slog.Info("worker: skipping PR check (forced re-analysis requested)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
		if o.logBuf != nil {
			o.logBuf.Add(issue.Identifier, makeBufLineWithSession("INFO", "worker: forced re-analysis of existing PR", runLogID))
		}
	}

	if o.workspace != nil {
		ws, err := o.workspace.EnsureWorkspace(ctx, issue.Identifier, branchName)
		if err != nil {
			slog.Warn("worker: workspace setup failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("ERROR", fmt.Sprintf("worker: workspace setup failed: %v", err), runLogID))
			}
			o.sendExit(ctx, issue, attempt, TerminalFailed, err)
			return
		}
		wsPath = ws.Path

		if ws.CreatedNow {
			hookLog := o.hookLogFn(issue.Identifier, runLogID)
			if err := workspace.RunHook(ctx, o.cfg.Hooks.AfterCreate, wsPath, o.cfg.Hooks.TimeoutMs, hookLog); err != nil {
				slog.Warn("worker: after_create hook failed, removing workspace so next retry re-runs it",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
				if o.logBuf != nil {
					o.logBuf.Add(issue.Identifier, makeBufLineWithSession("ERROR", fmt.Sprintf("worker: after_create hook failed: %v", err), runLogID))
				}
				_ = o.workspace.RemoveWorkspace(ctx, issue.Identifier, "")
				o.sendExit(ctx, issue, attempt, TerminalFailed, err)
				return
			}
		}
	}

	// Enrich PR context with diffs now that wsPath is known (best-effort).
	// BaseBranch is read-only after startup — no lock required.
	if prCtx != nil && wsPath != "" {
		prdetector.FetchPRContext(ctx, prCtx, wsPath, o.cfg.Agent.BaseBranch)
	}

	// Transition issue to working state (e.g. Todo → In Progress).
	o.transitionToWorking(ctx, issue)

	// Log the backend being used for this worker.
	displayBackend := backend
	if displayBackend == "" {
		displayBackend = "claude"
	}
	slog.Info("worker: starting",
		"issue_id", issue.ID, "issue_identifier", issue.Identifier,
		"attempt", attempt, "backend", displayBackend, "profile", profileName)
	if o.logBuf != nil {
		o.logBuf.Add(issue.Identifier, makeBufLineWithSession("INFO", fmt.Sprintf("worker: starting (backend=%s)", displayBackend), runLogID))
	}

	// Snapshot mutable config fields once under cfgMu so the entire worker
	// lifecycle uses a consistent view without holding the lock.
	o.cfgMu.RLock()
	maxTurns := o.cfg.Agent.MaxTurns
	readTimeoutMs := o.cfg.Agent.ReadTimeoutMs
	turnTimeoutMs := o.cfg.Agent.TurnTimeoutMs
	beforeRunHook := o.cfg.Hooks.BeforeRun
	afterRunHook := o.cfg.Hooks.AfterRun
	hookTimeoutMs := o.cfg.Hooks.TimeoutMs
	o.cfgMu.RUnlock()

	// --- Multi-turn loop ---
	// before_run hook runs once per worker invocation (not per turn), so that
	// hooks like "git reset --hard origin/main" set up a clean workspace for the
	// attempt without wiping Claude's work between turns.
	if wsPath != "" {
		hookLog := o.hookLogFn(issue.Identifier, runLogID)
		if err := workspace.RunHook(ctx, beforeRunHook, wsPath, hookTimeoutMs, hookLog); err != nil {
			slog.Warn("worker: before_run hook failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("ERROR", fmt.Sprintf("worker: before_run hook failed: %v", err), runLogID))
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
	// In worktree mode the branch is already checked out by EnsureWorkspace.
	// CheckoutBranch is only needed in legacy directory mode.
	o.cfgMu.RLock()
	worktreeMode := o.cfg.Workspace.Worktree
	o.cfgMu.RUnlock()
	if wsPath != "" && !worktreeMode && issue.BranchName != nil && *issue.BranchName != "" {
		if b := *issue.BranchName; !workspace.IsDefaultBranch(b) {
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

	var claudeSessionID *string
	// If the orchestrator passed in a resume session ID (set when an issue is
	// dispatched after manual pause), pre-populate claudeSessionID so the first
	// turn uses --resume / `exec resume` and continues the existing session.
	if resumeSessionID != "" {
		sid := resumeSessionID
		claudeSessionID = &sid
		slog.Info("worker: resuming from manual pause",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier, "session_id", resumeSessionID)
	}
	var allTextBlocks []string                                  // accumulate all Claude text blocks for the final tracker comment
	var cumulativeInput, cumulativeCached, cumulativeOutput int // accumulate tokens across turns for dashboard display
	var prevResultText string                                   // detect repeated identical responses (Codex resume loop)
	startedAt := time.Now()
	turn := 1
	for ; turn <= maxTurns; turn++ {
		// Enrich issue with comments before rendering the first-turn prompt.
		if turn == 1 {
			if detailed, err := o.tracker.FetchIssueDetail(ctx, issue.ID); err != nil {
				slog.Warn("worker: fetch issue detail failed (using cached issue)",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			} else {
				issue = *detailed
			}
		}

		// Render prompt — reviewer workers use the reviewer_prompt template
		// instead of the main WORKFLOW.md body.
		var attemptPtr *int
		if attempt > 0 {
			a := attempt
			attemptPtr = &a
		}
		o.cfgMu.RLock()
		isReviewer := profileName != "" && profileName == o.cfg.Agent.ReviewerProfile
		reviewerTmpl := o.cfg.Agent.ReviewerPrompt
		o.cfgMu.RUnlock()

		promptTemplate := o.cfg.PromptTemplate
		if isReviewer && reviewerTmpl != "" {
			promptTemplate = reviewerTmpl
		}
		renderedPrompt, err := prompt.Render(promptTemplate, issue, attemptPtr)
		if err != nil {
			slog.Warn("worker: prompt render failed",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("ERROR", fmt.Sprintf("worker: prompt render failed: %v", err), runLogID))
			}
			// Use a fresh background context for the after_run hook on the failure
			// path: the worker context may already be cancelled.
			// Use an explicit cancel call (not defer) because this block is inside
			// a loop — defer would not fire until runWorker returns (GO-R10-2).
			if wsPath != "" {
				hookCtx, hookCancel := context.WithTimeout(context.Background(), hookFallbackTimeout)
				if hookErr := workspace.RunHook(hookCtx, afterRunHook, wsPath, hookTimeoutMs, o.hookLogFn(issue.Identifier, runLogID)); hookErr != nil {
					slog.Warn("worker: after_run hook failed (ignored)", "issue_id", issue.ID, "error", hookErr)
				}
				hookCancel()
			}
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
		maps.Copy(profilesSnap, o.cfg.Agent.Profiles)
		o.cfgMu.RUnlock()

		var firstResumeContext []string
		if profileName != "" {
			if profile, ok := profilesSnap[profileName]; ok && profile.Prompt != "" {
				profilePrompt := prompt.RenderProfilePrompt(profile.Prompt, issue, attemptPtr)
				renderedPrompt += "\n\n" + profilePrompt
				if turn == 1 {
					firstResumeContext = append(firstResumeContext, profilePrompt)
				}
			}
		}
		// In teams mode, also append sub-agent roster context so the active backend
		// knows which specialised agents it can spawn via its delegation tool.
		if agentMode == "teams" {
			if subCtx := buildSubAgentContext(profilesSnap, profileName, backend); subCtx != "" {
				renderedPrompt += "\n\n" + subCtx
				if turn == 1 {
					firstResumeContext = append(firstResumeContext, subCtx)
				}
			}
		}

		// On the first turn, inject open PR context if detected.
		if turn == 1 {
			if prBlock := prdetector.FormatPRContext(prCtx); prBlock != "" {
				renderedPrompt += "\n\n" + prBlock
				firstResumeContext = append(firstResumeContext, prBlock)
			}
		}

		turnPrompt := renderedPrompt
		if shouldUseClaudeResumePrompt(backend, claudeSessionID) {
			resumePrompt, err := renderClaudeResumePrompt(o.cfg.Agent.ResumePrompt, issue, attemptPtr)
			if err != nil {
				slog.Warn("worker: resume prompt render failed",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			} else {
				turnPrompt = appendPromptSections(resumePrompt, firstResumeContext...)
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
			sessionID:  runLogID,
		}
		onProgress := func(partial agent.TurnResult) {
			select {
			case o.events <- OrchestratorEvent{
				Type:    EventWorkerUpdate,
				IssueID: issue.ID,
				RunEntry: &RunEntry{
					TurnCount:    turn,
					TotalTokens:  cumulativeInput + cumulativeCached + cumulativeOutput + partial.TotalTokens,
					InputTokens:  cumulativeInput + partial.InputTokens,
					OutputTokens: cumulativeOutput + partial.OutputTokens,
					SessionID:    runLogID,
					LastMessage:  partial.LastText,
				},
			}:
			default:
				slog.Debug("orchestrator: worker update event dropped (channel full)", "issue_id", issue.ID)
			}
		}
		turnStart := time.Now()
		logDir := ""
		if o.agentLogDir != "" {
			logDir = filepath.Join(o.agentLogDir, workspace.SanitizeKey(issue.Identifier))
		}
		result, runErr := o.runner.RunTurn(ctx, workerLog, onProgress, claudeSessionID, turnPrompt, wsPath,
			agentCommand, workerHost, logDir, readTimeoutMs, turnTimeoutMs)

		if result.SessionID != "" {
			s := result.SessionID
			claudeSessionID = &s
			// Propagate the agent's real session ID to state.Running so that
			// manual pause/resume can capture it. Non-blocking — drop on busy
			// channel; the next progress update will retry.
			select {
			case o.events <- OrchestratorEvent{
				Type:     EventWorkerUpdate,
				IssueID:  issue.ID,
				RunEntry: &RunEntry{AgentSessionID: s},
			}:
			default:
			}
		}

		// Accumulate all Claude text blocks for the final session comment.
		allTextBlocks = append(allTextBlocks, result.AllTextBlocks...)

		// after_run hook (best-effort, logged and ignored)
		o.runAfterHook(ctx, afterRunHook, hookTimeoutMs, wsPath, issue.ID, issue.Identifier, runLogID)

		// Track the current git branch after each turn so retried workers can
		// resume from the same branch. Only fires when the agent has switched to
		// a non-default branch (i.e. created its feature branch).
		if wsPath != "" {
			if currentBranch := workspace.GetCurrentBranch(ctx, wsPath); !workspace.IsDefaultBranch(currentBranch) {
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
				cause = errors.New(msg)
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
				o.logBuf.Add(issue.Identifier, formatBufLine("WARN", "worker: turn failed", []any{"detail", detail, "session_id", runLogID}))
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

		// Codex runs to completion in a single turn — "codex exec" produces
		// the full result and exits. Resuming a completed Codex session just
		// replays the same answer (burning tokens). Exit after the first
		// successful turn for Codex backends.
		if backend == "codex" && !result.Failed {
			slog.Info("worker: codex turn completed — exiting loop (single-turn backend)",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "turn", turn)
			break
		}

		// InputRequired means the agent needs human input to continue (e.g.
		// permission prompt, API key). Send TerminalInputRequired so the event
		// loop queues the issue for user input instead of retrying or completing.
		if result.InputRequired {
			inputContext := result.FailureText
			if inputContext == "" {
				inputContext = result.ResultText
			}
			if inputContext == "" {
				inputContext = "Agent requires human input to continue"
			}
			slog.Info("worker: agent requires input — queuing for user input",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "turn", turn,
				"context", inputContext)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("WARN",
					fmt.Sprintf("worker: agent requires input — %s", inputContext), runLogID))
			}
			var sid string
			if claudeSessionID != nil {
				sid = *claudeSessionID
			}
			o.sendExitWithInputRequired(ctx, issue, attempt, &InputRequiredEntry{
				IssueID:     issue.ID,
				Identifier:  issue.Identifier,
				SessionID:   sid,
				Context:     inputContext,
				Backend:     backend,
				Command:     agentCommand,
				WorkerHost:  workerHost,
				ProfileName: profileName,
				QueuedAt:    time.Now(),
			})
			return
		}

		// Detect repeated identical responses — a sign the session has
		// concluded but the agent is replaying its last answer on each
		// resume (common with Codex). Two consecutive identical result
		// texts trigger an early exit to avoid burning tokens.
		if result.ResultText != "" && result.ResultText == prevResultText {
			slog.Info("worker: duplicate result text detected — session concluded, exiting loop",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "turn", turn)
			break
		}
		prevResultText = result.ResultText

		// Emit turn summary line (#3): turn N complete — Δin/Δout tokens, elapsed Xs.
		if o.logBuf != nil {
			elapsed := time.Since(turnStart)
			summary := fmt.Sprintf("turn %d complete — +%d in / +%d out tokens, %.1fs",
				turn, result.InputTokens, result.OutputTokens, elapsed.Seconds())
			o.logBuf.Add(issue.Identifier, formatBufLine("INFO", "worker: turn_summary", []any{"summary", summary, "session_id", runLogID}))
		}

		// Accumulate tokens from this turn before the end-of-turn update so the
		// dashboard always shows the true running total, not just the per-turn count.
		cumulativeInput += result.InputTokens
		cumulativeCached += result.CachedInputTokens
		cumulativeOutput += result.OutputTokens

		// Send non-blocking progress update so the dashboard shows live turn/token data.
		select {
		case o.events <- OrchestratorEvent{
			Type:    EventWorkerUpdate,
			IssueID: issue.ID,
			RunEntry: &RunEntry{
				TurnCount:    turn,
				LastMessage:  result.ResultText,
				TotalTokens:  cumulativeInput + cumulativeCached + cumulativeOutput,
				InputTokens:  cumulativeInput,
				OutputTokens: cumulativeOutput,
				SessionID:    runLogID,
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

		o.cfgMu.RLock()
		activeStates := append([]string{}, o.cfg.Tracker.ActiveStates...)
		o.cfgMu.RUnlock()
		if !isActiveState(issue.State, State{ActiveStates: activeStates}) {
			break // issue left active states — clean exit
		}
	}

	// Content-based input detection: the agent succeeded but its output contains
	// questions for the user (e.g. "Questions for you:", "How would you like to
	// proceed?"). Route to input_required instead of the normal success path so
	// the dashboard shows the "Needs Input" badge.
	//
	// Questions appear in assistant text blocks (the full conversational output),
	// not in result text (a brief summary). Scan the tail of allTextBlocks —
	// the last few blocks are where concluding questions land.
	if ctx.Err() == nil {
		// Build the check text from the last text blocks (where questions appear).
		// Cap at 3 blocks to keep the scan focused on the conclusion, not the
		// entire multi-turn conversation.
		var checkText string
		if n := len(allTextBlocks); n > 0 {
			start := max(0, n-3)
			checkText = strings.Join(allTextBlocks[start:], "\n")
		}
		// Also include ResultText — some agents put the summary there.
		if prevResultText != "" {
			checkText = checkText + "\n" + prevResultText
		}
		if agent.IsSentinelInputRequired(checkText) {
			// Use the last text block as context — it contains the actual questions
			// the agent asked, which gets posted as the tracker comment in inline mode.
			inputContext := checkText
			if len(inputContext) > 4000 {
				inputContext = inputContext[len(inputContext)-4000:]
			}
			slog.Info("worker: agent output contains questions — queuing for user input",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("WARN",
					fmt.Sprintf("worker: agent requires input — %s", inputContext), runLogID))
			}
			var sid string
			if claudeSessionID != nil {
				sid = *claudeSessionID
			}
			o.sendExitWithInputRequired(ctx, issue, attempt, &InputRequiredEntry{
				IssueID:     issue.ID,
				Identifier:  issue.Identifier,
				SessionID:   sid,
				Context:     inputContext,
				Backend:     backend,
				Command:     agentCommand,
				WorkerHost:  workerHost,
				ProfileName: profileName,
				QueuedAt:    time.Now(),
			})
			return
		}
	}

	// If the agent created a PR during this run, comment its URL on the tracker
	// issue.  This runs before the session summary so the PR link is visible even
	// on trackers that truncate long comments.  Uses the same gh CLI check as the
	// pre-run guard (now the workspace is on the newly-created branch).
	if wsPath != "" && prCtx == nil {
		if prURL := workspace.FindOpenPRURL(ctx, wsPath); prURL != "" {
			detectedPRURL = prURL
			// Dedup: check if we already posted a PR comment for this URL.
			prComment := fmt.Sprintf("🔗 Pull request created: %s", prURL)
			alreadyPosted := false
			for _, c := range issue.Comments {
				if strings.Contains(c.Body, prURL) {
					alreadyPosted = true
					break
				}
			}
			if alreadyPosted {
				slog.Info("worker: PR comment already posted, skipping",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "pr_url", prURL)
			} else if err := o.tracker.CreateComment(ctx, issue.ID, prComment); err != nil {
				slog.Warn("worker: create PR comment failed (ignored)",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
			} else {
				slog.Info("worker: PR link commented on issue",
					"issue_id", issue.ID, "issue_identifier", issue.Identifier, "pr_url", prURL)
				if o.logBuf != nil {
					// parseLogLine detects "pr" events by substring-matching "pr_opened" in the
					// message text, not by the log level or a separate event-type field.
					o.logBuf.Add(issue.Identifier, makeBufLineWithSession("INFO", fmt.Sprintf("worker: pr_opened url=%s", prURL), runLogID))
				}
			}
		}
	}

	// Build session summary once — reused for both PR comment and tracker comment.
	sessionComment := formatSessionComment(allTextBlocks, issue.Identifier)

	// Push the PR branch and post a summary comment on the existing PR (best-effort).
	// Use a fresh background-derived context with a timeout so that a cancellation
	// of the worker context (e.g. user pause) between the ctx.Err() guard and
	// command execution does not silently skip the post-run cleanup.
	if prCtx != nil && ctx.Err() == nil {
		postRunCtx, postRunCancel := context.WithTimeout(context.Background(), postRunTimeout)
		defer postRunCancel()
		// Push so the remote branch reflects the agent's changes.
		if wsPath != "" {
			pushCmd := exec.CommandContext(postRunCtx, "git", "push", "origin", prCtx.Branch)
			pushCmd.Dir = wsPath
			if err := pushCmd.Run(); err != nil {
				slog.Warn("worker: git push failed (non-fatal)",
					"issue_identifier", issue.Identifier, "branch", prCtx.Branch, "error", err)
			}
		}

		if sessionComment != "" {
			// gh pr comment <url> uses the GitHub API directly and does not need a
			// working directory; omitting Dir avoids misleading readers.
			ghCommentCmd := exec.CommandContext(postRunCtx, "gh", "pr", "comment", prCtx.URL,
				"--body", sessionComment)
			if err := ghCommentCmd.Run(); err != nil {
				slog.Warn("worker: gh pr comment failed (non-fatal)",
					"issue_identifier", issue.Identifier, "pr_url", prCtx.URL, "error", err)
			} else {
				slog.Info("worker: posted session summary to PR",
					"issue_identifier", issue.Identifier, "pr_url", prCtx.URL)
			}
		}
	}

	// Post one comprehensive comment covering the full session narration (best-effort).
	// Skip when there is an open PR: the summary was already posted as a PR comment
	// above, so posting it again on the tracker issue would create a duplicate (GO-R10-3).
	if sessionComment != "" && prCtx == nil {
		if err := o.tracker.CreateComment(ctx, issue.ID, sessionComment); err != nil {
			slog.Warn("worker: create session comment failed (ignored)",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier, "error", err)
		}
	}

	// Move issue to completion_state (e.g. "In Review") so Itervox stops
	// re-dispatching it. Without this, issues stay in active_states after a
	// successful run and get picked up again on the next retry tick.
	// Up to 4 attempts total (1 immediate + 3 retries with 2s/4s/8s backoff) to guard against
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
			o.logBuf.Add(issue.Identifier, makeBufLineWithSession("INFO", fmt.Sprintf("worker: moving issue to %q in tracker", completionState), runLogID))
		}
		// Use a fresh context for the tracker API calls so that a cancelled worker
		// context (user paused/terminated mid-retry) does not immediately fail the
		// request — the individual API call should complete or time out on its own.
		// The retry select still watches ctx.Done() to stop between attempts if the
		// user terminates while we're waiting for the next backoff window.
		transCtx, cancelTrans := context.WithTimeout(context.Background(), postRunTimeout)
		defer cancelTrans()
		var transitionErr error
	transitionLoop:
		for i := range maxTransitionAttempts {
			if i > 0 {
				delay := time.Duration(1<<uint(i-1)) * 2 * time.Second // 2s, 4s, 8s
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					transitionErr = ctx.Err()
					break transitionLoop
				}
			}
			if transitionErr = o.tracker.UpdateIssueState(transCtx, issue.ID, completionState); transitionErr == nil {
				break transitionLoop
			}
			slog.Warn("worker: completion state transition attempt failed, retrying",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier,
				"target_state", completionState, "attempt", i+1, "error", transitionErr)
		}
		if transitionErr != nil {
			slog.Error("worker: completion state transition failed after retries",
				"issue_id", issue.ID, "issue_identifier", issue.Identifier,
				"target_state", completionState, "error", transitionErr)
			if o.logBuf != nil {
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("ERROR", fmt.Sprintf("worker: state transition to %q failed: %v — issue paused", completionState, transitionErr), runLogID))
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
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("INFO", fmt.Sprintf("worker: → %s", completionState), runLogID))
				o.logBuf.Add(issue.Identifier, makeBufLineWithSession("INFO", fmt.Sprintf("worker: ✓ issue moved to %q", completionState), runLogID))
			}
		}
	}

	// Log a completion summary with token usage so it appears in the log file
	// and stderr (charmbracelet/log). This is the main human-visible record of
	// a successful run — includes turns, tokens, and elapsed time.
	elapsed := time.Since(startedAt)
	completionArgs := []any{
		"issue_identifier", issue.Identifier,
		"status", "succeeded",
		"turns", turn - 1,
		"input_tokens", cumulativeInput,
		"output_tokens", cumulativeOutput,
		"elapsed", elapsed.Round(time.Second).String(),
	}
	if detectedPRURL != "" {
		completionArgs = append(completionArgs, "pr_url", detectedPRURL)
	}
	slog.Info("worker: completed", completionArgs...)

	// Release the log buffer for this issue to free memory (after the completion
	// state transition so any transition errors are visible in the TUI log pane).
	if o.logBuf != nil {
		o.logBuf.Remove(issue.Identifier)
	}

	// Pass branchName so the auto-clear handler uses the actual worktree branch
	// (which may be prCtx.Branch on a PR-continuation run, not issue.BranchName).
	o.sendExitWithBranch(ctx, issue, attempt, TerminalSucceeded, nil, branchName, detectedPRURL)
}

// hookLogFn returns a function suitable for workspace.RunHook's logFn parameter.
// Each hook output line is forwarded to the per-issue log buffer as an info entry
// tagged with sessionID so hook messages are attributable to the correct Timeline run.
// Returns nil when logBuf is not configured (no-op in RunHook).
func (o *Orchestrator) hookLogFn(identifier, sessionID string) func(string) {
	if o.logBuf == nil {
		return nil
	}
	return func(line string) {
		o.logBuf.Add(identifier, makeBufLineWithSession("INFO", "hook: "+line, sessionID))
	}
}

func (o *Orchestrator) runAfterHook(ctx context.Context, hook string, timeoutMs int, wsPath, issueID, identifier, sessionID string) {
	if wsPath == "" {
		return
	}
	if err := workspace.RunHook(ctx, hook, wsPath, timeoutMs, o.hookLogFn(identifier, sessionID)); err != nil {
		slog.Warn("worker: after_run hook failed (ignored)", "issue_id", issueID, "error", err)
	}
}

func shouldUseClaudeResumePrompt(backend string, sessionID *string) bool {
	if sessionID == nil || *sessionID == "" {
		return false
	}
	return backend == "" || backend == "claude"
}

func renderClaudeResumePrompt(template string, issue domain.Issue, attempt *int) (string, error) {
	if strings.TrimSpace(template) == "" {
		template = config.DefaultResumePrompt
	}
	return prompt.Render(template, issue, attempt)
}

func appendPromptSections(base string, sections ...string) string {
	out := base
	for _, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}
		out += "\n\n" + section
	}
	return out
}

// generateRunID returns a short random ID that is assigned to a worker run
// before the agent subprocess starts, enabling all log entries — including
// early hook/worker messages — to be tagged with the same session ID.
func generateRunID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return "run-" + hex.EncodeToString(b)
}

// runWorkerWithResume dispatches a resumed worker for an input-required issue.
// The user's message is used as the prompt and the session ID enables --resume.
// Runs a single turn; on success proceeds to completion state transition.
func (o *Orchestrator) runWorkerWithResume(ctx context.Context, issue domain.Issue, workerHost, agentCommand, backend, profileName string, sessionID *string, userMessage string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("resumed worker panicked",
				"issue_identifier", issue.Identifier, "panic", r)
			o.sendExit(ctx, issue, 0, TerminalFailed, fmt.Errorf("resumed worker panic: %v", r))
		}
	}()

	workerLog := &bufLogger{
		base:       slog.With("issue_id", issue.ID, "issue_identifier", issue.Identifier, "kind", "resumed"),
		buf:        o.logBuf,
		identifier: issue.Identifier,
	}
	workerLog.Info("worker: resuming with user input", "session_id", sessionID)

	o.cfgMu.RLock()
	readTimeoutMs := o.cfg.Agent.ReadTimeoutMs
	turnTimeoutMs := o.cfg.Agent.TurnTimeoutMs
	o.cfgMu.RUnlock()

	logDir := ""
	if o.agentLogDir != "" {
		logDir = filepath.Join(o.agentLogDir, workspace.SanitizeKey(issue.Identifier))
	}

	result, runErr := o.runner.RunTurn(ctx, workerLog, nil, sessionID, userMessage, "",
		agentCommand, workerHost, logDir, readTimeoutMs, turnTimeoutMs)

	if runErr != nil || result.Failed {
		cause := runErr
		if cause == nil {
			cause = fmt.Errorf("resumed turn failed: %s", result.FailureText)
		}
		workerLog.Warn("worker: resumed turn failed", "error", cause)
		o.sendExit(ctx, issue, 0, TerminalFailed, cause)
		return
	}

	workerLog.Info("worker: resumed turn succeeded")

	// Transition to completion state.
	o.cfgMu.RLock()
	completionState := o.cfg.Tracker.CompletionState
	o.cfgMu.RUnlock()
	if completionState != "" && ctx.Err() == nil {
		transCtx, cancelTrans := context.WithTimeout(context.Background(), postRunTimeout)
		defer cancelTrans()
		if err := o.tracker.UpdateIssueState(transCtx, issue.ID, completionState); err != nil {
			workerLog.Warn("worker: completion state transition failed after resume", "error", err)
		}
	}

	o.sendExit(ctx, issue, 0, TerminalSucceeded, nil)
}

func (o *Orchestrator) sendExit(ctx context.Context, issue domain.Issue, attempt int, reason TerminalReason, err error) {
	o.sendExitWithBranch(ctx, issue, attempt, reason, err, "", "")
}

func (o *Orchestrator) sendExitWithBranch(ctx context.Context, issue domain.Issue, attempt int, reason TerminalReason, err error, branchName string, prURL string) {
	ev := OrchestratorEvent{
		Type:    EventWorkerExited,
		IssueID: issue.ID,
		RunEntry: &RunEntry{
			Issue:          issue,
			BranchName:     branchName,
			PRURL:          prURL,
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
		sendCtx, cancel = context.WithTimeout(context.Background(), hookFallbackTimeout)
		defer cancel()
	}
	// Nil-receive channel blocks forever — safe fallback when Run hasn't started.
	var orchDone <-chan struct{}
	if p := o.runCtx.Load(); p != nil {
		orchDone = (*p).Done()
	}
	select {
	case o.events <- ev:
	case <-orchDone:
		slog.Warn("worker: exit event dropped (orchestrator exited)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
	case <-sendCtx.Done():
		slog.Warn("worker: exit event not delivered (orchestrator shutting down)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
	}
}

func (o *Orchestrator) sendExitWithInputRequired(ctx context.Context, issue domain.Issue, attempt int, entry *InputRequiredEntry) {
	ev := OrchestratorEvent{
		Type:    EventWorkerExited,
		IssueID: issue.ID,
		RunEntry: &RunEntry{
			Issue:          issue,
			TerminalReason: TerminalInputRequired,
			RetryAttempt:   &attempt,
		},
		InputRequiredEntry: entry,
	}
	sendCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(context.Background(), hookFallbackTimeout)
		defer cancel()
	}
	var orchDone <-chan struct{}
	if p := o.runCtx.Load(); p != nil {
		orchDone = (*p).Done()
	}
	select {
	case o.events <- ev:
	case <-orchDone:
		slog.Warn("worker: input-required event dropped (orchestrator exited)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
	case <-sendCtx.Done():
		slog.Warn("worker: input-required event not delivered (orchestrator shutting down)",
			"issue_id", issue.ID, "issue_identifier", issue.Identifier)
	}
}
