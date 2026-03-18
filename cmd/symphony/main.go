package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/vnovick/symphony-go/internal/agent"
	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/logbuffer"
	"github.com/vnovick/symphony-go/internal/orchestrator"
	"github.com/vnovick/symphony-go/internal/server"
	"github.com/vnovick/symphony-go/internal/statusui"
	"github.com/vnovick/symphony-go/internal/tracker"
	"github.com/vnovick/symphony-go/internal/tracker/github"
	"github.com/vnovick/symphony-go/internal/tracker/linear"
	"github.com/vnovick/symphony-go/internal/workflow"
	"github.com/vnovick/symphony-go/internal/workspace"
	"gopkg.in/lumberjack.v2"
)

// Set by GoReleaser via ldflags — empty when built with `go build`
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: symphony [command] [flags]

Commands:
  init    Scan a repository and generate a WORKFLOW.md starter file
            --tracker  linear|github  (required)
            --output   output file path (default: WORKFLOW.md)
            --dir      directory to scan (default: .)
            --force    overwrite existing output file

  clear   Remove workspace directories created by symphony
            --workflow path to WORKFLOW.md (default: WORKFLOW.md)
            [identifier ...]  specific issues to clear; omit for all

  --version  Print version information

Run mode (default when no command given):
`)
	flag.PrintDefaults()
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			runInit(os.Args[2:])
			return
		case "clear":
			runClear(os.Args[2:])
			return
		case "--version", "-version":
			fmt.Printf("symphony %s (commit: %s, built: %s)\n", version, commit, date)
			return
		case "help", "--help", "-help", "-h":
			printUsage()
			return
		}
	}

	flag.Usage = printUsage
	workflowPath := flag.String("workflow", "WORKFLOW.md", "path to WORKFLOW.md")
	logsDir := flag.String("logs-dir", "log", "directory for rotating log files")
	verbose := flag.Bool("verbose", false, "enable DEBUG-level logging (includes Claude output)")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	// Tee logs to stderr and a rotating file under <logs-dir>/symphony.log.
	if err := os.MkdirAll(*logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logs dir %s: %v\n", *logsDir, err)
		os.Exit(1)
	}
	rotatingFile := &lumberjack.Logger{
		Filename:   filepath.Join(*logsDir, "symphony.log"),
		MaxSize:    10, // MB
		MaxBackups: 5,
		Compress:   true,
	}
	logWriter := io.MultiWriter(os.Stderr, rotatingFile)
	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: logLevel,
	})))
	slog.Info("symphony starting", "version", version, "commit", commit, "date", date)
	slog.Info("logging to file", "path", rotatingFile.Filename)

	// Top-level context cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Outer loop: restart when WORKFLOW.md changes.
	for {
		cfg, err := config.Load(*workflowPath)
		if err != nil {
			slog.Error("failed to load config", "path", *workflowPath, "error", err)
			os.Exit(1)
		}
		if err := config.ValidateDispatch(cfg); err != nil {
			slog.Error("config validation failed", "path", *workflowPath, "error", err)
			os.Exit(1)
		}

		runCtx, cancel := context.WithCancel(ctx)

		// Watch WORKFLOW.md; cancel runCtx to trigger reload on change.
		go func() {
			if err := workflow.Watch(runCtx, *workflowPath, cancel); err != nil && runCtx.Err() == nil {
				slog.Warn("workflow watcher stopped", "error", err)
			}
		}()

		if err := run(runCtx, cfg, *workflowPath, rotatingFile.Filename, rotatingFile, logLevel); err != nil && ctx.Err() == nil {
			slog.Warn("run returned, restarting", "error", err)
		}
		cancel()

		if ctx.Err() != nil {
			return // top-level shutdown
		}

		slog.Info("WORKFLOW.md changed — reloading config")
		time.Sleep(200 * time.Millisecond)
	}
}

// run starts the orchestrator (and optionally the HTTP server) and blocks until
// runCtx is cancelled. logFile is passed to the HTTP server for the /api/v1/logs endpoint.
// fileWriter is the rotating log file writer; logLevel is the configured log level.
// Both are used to redirect slog away from stderr once the TUI takes the terminal.
func run(ctx context.Context, cfg *config.Config, workflowPath string, logFile string, fileWriter io.Writer, logLevel slog.Level) error {
	tr, err := buildTracker(cfg)
	if err != nil {
		return fmt.Errorf("build tracker: %w", err)
	}

	cfg.Agent.Command = resolveAgentCommand(cfg.Agent.Command)
	runner := agent.NewClaudeRunner()
	wm := workspace.NewManager(cfg)

	// Remove workspaces for issues that were terminal when we last shut down.
	orchestrator.StartupTerminalCleanup(ctx, tr, cfg.Tracker.TerminalStates, func(id string) error {
		return wm.RemoveWorkspace(id)
	})

	refreshChan := make(chan struct{}, 1)
	logBuf := logbuffer.New()
	// Persist per-issue logs to disk alongside the main log file so they
	// survive restarts and remain viewable after an issue completes.
	if logFile != "" {
		logBuf.SetLogDir(filepath.Join(filepath.Dir(logFile), "issues"))
	}
	orch := orchestrator.New(cfg, tr, runner, wm)
	if os.Getenv("SYMPHONY_DRY_RUN") == "1" {
		orch.DryRun = true
		slog.Info("symphony: dry-run mode enabled — agents will not be dispatched")
	}
	orch.SetLogBuffer(logBuf)
	if logFile != "" {
		orch.SetHistoryFile(filepath.Join(filepath.Dir(logFile), "history.json"))
		orch.SetPausedFile(filepath.Join(filepath.Dir(logFile), "paused.json"))
	}

	snap := func() server.StateSnapshot {
		s := orch.Snapshot()
		now := time.Now()

		running := make([]server.RunningRow, 0, len(s.Running))
		for _, r := range s.Running {
			msg := r.LastMessage
			if len(msg) > 120 {
				msg = msg[:120] + "…"
			}
			var lastEvAt string
			if r.LastEventAt != nil {
				lastEvAt = r.LastEventAt.Format(time.RFC3339)
			}
			running = append(running, server.RunningRow{
				Identifier:   r.Issue.Identifier,
				State:        r.Issue.State,
				TurnCount:    r.TurnCount,
				Tokens:       r.TotalTokens,
				InputTokens:  r.InputTokens,
				OutputTokens: r.OutputTokens,
				LastEvent:    msg,
				LastEventAt:  lastEvAt,
				SessionID:    r.SessionID,
				WorkerHost:   r.WorkerHost,
				Backend:      r.Backend,
				ElapsedMs:    now.Sub(r.StartedAt).Milliseconds(),
				StartedAt:    r.StartedAt,
			})
		}
		// Sort by start time (oldest first) so order is stable across ticks.
		sort.Slice(running, func(i, j int) bool {
			return running[i].StartedAt.Before(running[j].StartedAt)
		})

		retrying := make([]server.RetryRow, 0, len(s.RetryAttempts))
		for _, r := range s.RetryAttempts {
			row := server.RetryRow{
				Identifier: r.Identifier,
				Attempt:    r.Attempt,
				DueAt:      r.DueAt,
			}
			if r.Error != nil {
				row.Error = *r.Error
			}
			retrying = append(retrying, row)
		}

		paused := make([]string, 0, len(s.PausedIdentifiers))
		for identifier := range s.PausedIdentifiers {
			paused = append(paused, identifier)
		}

		var rateLimits *server.RateLimitInfo
		var activeProjectFilter []string
		if lc, ok := tr.(*linear.Client); ok {
			reqLim, reqRem, reset, cplxLim, cplxRem := lc.RateLimits()
			if reqLim > 0 || cplxLim > 0 {
				rateLimits = &server.RateLimitInfo{
					RequestsLimit:       reqLim,
					RequestsRemaining:   reqRem,
					RequestsReset:       reset,
					ComplexityLimit:     cplxLim,
					ComplexityRemaining: cplxRem,
				}
			}
			activeProjectFilter = lc.GetProjectFilter()
		} else if gc, ok := tr.(*github.Client); ok {
			limit, remaining, reset := gc.RateLimits()
			if limit > 0 {
				rateLimits = &server.RateLimitInfo{
					RequestsLimit:     limit,
					RequestsRemaining: remaining,
					RequestsReset:     reset,
				}
			}
		}
		// Read the mutable cfg fields once, under cfgMu, for a consistent snapshot.
		profiles := orch.ProfilesCfg()
		agentMode := orch.AgentModeCfg()
		activeStates, terminalStates, completionState := orch.TrackerStatesCfg()

		// Collect available profile names (sorted for stable output).
		var availableProfiles []string
		for name := range profiles {
			availableProfiles = append(availableProfiles, name)
		}
		sort.Strings(availableProfiles)

		// Build profileDefs map.
		var profileDefs map[string]server.ProfileDef
		if len(profiles) > 0 {
			profileDefs = make(map[string]server.ProfileDef, len(profiles))
			for n, p := range profiles {
				profileDefs[n] = server.ProfileDef{
					Command: p.Command,
					Prompt:  p.Prompt,
				}
			}
		}

		// Build history rows from the orchestrator's completed-run ring buffer.
		completedRuns := orch.RunHistory()
		history := make([]server.HistoryRow, 0, len(completedRuns))
		for _, r := range completedRuns {
			history = append(history, server.HistoryRow{
				Identifier:   r.Identifier,
				Title:        r.Title,
				StartedAt:    r.StartedAt,
				FinishedAt:   r.FinishedAt,
				ElapsedMs:    r.ElapsedMs,
				TurnCount:    r.TurnCount,
				TotalTokens:  r.TotalTokens,
				InputTokens:  r.InputTokens,
				OutputTokens: r.OutputTokens,
				Status:       r.Status,
				WorkerHost:   r.WorkerHost,
				Backend:      r.Backend,
				SessionID:    r.SessionID,
			})
		}

		pausedWithPR := orch.GetPausedOpenPRs()
		return server.StateSnapshot{
			GeneratedAt:         now,
			Counts:              server.Counts{Running: len(running), Retrying: len(retrying), Paused: len(paused)},
			Running:             running,
			History:             history,
			Retrying:            retrying,
			Paused:              paused,
			PausedWithPR:        pausedWithPR,
			MaxConcurrentAgents: orch.MaxWorkers(), // always read from orchestrator, not stale cfg copy
			RateLimits:          rateLimits,
			TrackerKind:         cfg.Tracker.Kind,
			ActiveProjectFilter: activeProjectFilter,
			AvailableProfiles:   availableProfiles,
			ProfileDefs:         profileDefs,
			AgentMode:           agentMode,
			ActiveStates:        activeStates,
			TerminalStates:      terminalStates,
			CompletionState:     completionState,
			BacklogStates:       cfg.Tracker.BacklogStates,
			PollIntervalMs:      cfg.Polling.IntervalMs,
		}
	}

	// Terminal status UI — full-screen ANSI panel, refreshes every second.
	tuiCfg := statusui.Config{MaxAgents: cfg.Agent.MaxConcurrentAgents}
	if cfg.Server.Port != nil {
		tuiCfg.DashboardURL = fmt.Sprintf("http://%s:%d/", cfg.Server.Host, *cfg.Server.Port)
	}
	// Wire project picker to TUI when the tracker is Linear.
	if lc, ok := tr.(*linear.Client); ok {
		tuiCfg.FetchProjects = func() ([]statusui.ProjectItem, error) {
			fetchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			lps, err := lc.FetchProjects(fetchCtx)
			if err != nil {
				return nil, err
			}
			items := make([]statusui.ProjectItem, len(lps))
			for i, lp := range lps {
				items[i] = statusui.ProjectItem{ID: lp.ID, Name: lp.Name, Slug: lp.Slug}
			}
			return items, nil
		}
		tuiCfg.SetProjectFilter = func(slugs []string) {
			lc.SetProjectFilter(slugs)
			updateWorkflowProjectSlug(workflowPath, slugs)
		}
	}
	// Wire + / - worker adjustment into TUI.
	tuiCfg.AdjustWorkers = func(delta int) {
		next := orch.MaxWorkers() + delta
		orch.SetMaxWorkers(next)
		if err := workflow.PatchIntField(workflowPath, "max_concurrent_agents", orch.MaxWorkers()); err != nil {
			slog.Warn("failed to persist max_concurrent_agents to WORKFLOW.md", "error", err)
		}
	}
	// Wire backlog panel: fetch issues in backlog states.
	if len(cfg.Tracker.BacklogStates) > 0 {
		tuiCfg.FetchBacklog = func() ([]statusui.BacklogIssueItem, error) {
			fetchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			issues, err := tr.FetchIssuesByStates(fetchCtx, cfg.Tracker.BacklogStates)
			if err != nil {
				return nil, err
			}
			items := make([]statusui.BacklogIssueItem, len(issues))
			for i, iss := range issues {
				pri := 0
				if iss.Priority != nil {
					pri = *iss.Priority
				}
				items[i] = statusui.BacklogIssueItem{
					Identifier: iss.Identifier,
					Title:      iss.Title,
					State:      iss.State,
					Priority:   pri,
				}
			}
			return items, nil
		}
		// Wire dispatch: move a backlog issue into the first active state.
		if len(cfg.Tracker.ActiveStates) > 0 {
			targetState := cfg.Tracker.ActiveStates[0]
			tuiCfg.DispatchIssue = func(identifier string) error {
				dispCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				allStates := append(append([]string{}, cfg.Tracker.BacklogStates...), cfg.Tracker.ActiveStates...)
				issues, err := tr.FetchIssuesByStates(dispCtx, allStates)
				if err != nil {
					return err
				}
				for _, iss := range issues {
					if iss.Identifier == identifier {
						return tr.UpdateIssueState(dispCtx, iss.ID, targetState)
					}
				}
				return fmt.Errorf("issue %s not found", identifier)
			}
		}
	}
	// Redirect slog to file-only before the TUI takes the alt-screen.
	// Without this, concurrent slog writes to stderr corrupt the bubbletea display.
	// The TUI log pane reads directly from logBuf instead of stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(fileWriter, &slog.HandlerOptions{Level: logLevel})))
	// tuiCancel lets the TUI kill an agent via the 'x' key; it's the same logic as the HTTP cancel.
	tuiCancel := func(identifier string) bool {
		return orch.CancelIssue(identifier)
	}
	// tuiResume lets the TUI resume a paused agent via the 'r' key.
	tuiResume := func(identifier string) bool {
		ok := orch.ResumeIssue(identifier)
		if ok {
			orch.Refresh()
		}
		return ok
	}
	tuiCfg.ResumeIssue = tuiResume
	tuiCfg.TerminateIssue = func(identifier string) bool {
		ok := orch.TerminateIssue(identifier)
		if ok {
			// Safe to Refresh here: DiscardingIdentifiers blocks re-dispatch until
			// the async UpdateIssueState goroutine completes (EventDiscardComplete),
			// so a poll triggered immediately after discard cannot re-pick the issue.
			orch.Refresh()
		}
		return ok
	}
	tuiCfg.TriggerPoll = orch.Refresh
	go statusui.Run(ctx, snap, logBuf, tuiCfg, tuiCancel)

	// fetchIssues enriches tracker issues with live orchestrator state for the dashboard.
	fetchIssues := func(ctx context.Context) ([]server.TrackerIssue, error) {
		seen := map[string]bool{}
		var allStates []string
		base := append(append(append([]string{}, cfg.Tracker.BacklogStates...), cfg.Tracker.ActiveStates...), cfg.Tracker.TerminalStates...)
		if cfg.Tracker.CompletionState != "" {
			base = append(base, cfg.Tracker.CompletionState)
		}
		for _, s := range base {
			if !seen[s] {
				seen[s] = true
				allStates = append(allStates, s)
			}
		}
		issues, err := tr.FetchIssuesByStates(ctx, allStates)
		if err != nil {
			return nil, err
		}
		snap := orch.Snapshot()
		now := time.Now()
		result := make([]server.TrackerIssue, len(issues))
		for i, issue := range issues {
			ti := server.TrackerIssue{
				Identifier: issue.Identifier,
				Title:      issue.Title,
				State:      issue.State,
				Labels:     issue.Labels,
				Priority:   issue.Priority,
				BranchName: issue.BranchName,
			}
			if issue.Description != nil {
				ti.Description = *issue.Description
			}
			if issue.URL != nil {
				ti.URL = *issue.URL
			}
			// BlockedBy: collect non-nil identifiers from []domain.BlockerRef
			for _, b := range issue.BlockedBy {
				if b.Identifier != nil && *b.Identifier != "" {
					ti.BlockedBy = append(ti.BlockedBy, *b.Identifier)
				}
			}
			// Comments: map domain.Comment to server.CommentRow
			for _, c := range issue.Comments {
				row := server.CommentRow{
					Author: c.AuthorName,
					Body:   c.Body,
				}
				if c.CreatedAt != nil {
					row.CreatedAt = c.CreatedAt.Format(time.RFC3339)
				}
				ti.Comments = append(ti.Comments, row)
			}
			// Per-issue agent profile override.
			if profileName, ok := snap.IssueProfiles[issue.Identifier]; ok && profileName != "" {
				ti.AgentProfile = profileName
			}
			// Orchestrator state
			if re, ok := snap.Running[issue.ID]; ok {
				ti.OrchestratorState = "running"
				ti.TurnCount = re.TurnCount
				ti.Tokens = re.TotalTokens
				ti.ElapsedMs = now.Sub(re.StartedAt).Milliseconds()
				if re.LastMessage != "" {
					ti.LastMessage = re.LastMessage
				}
			} else if re, ok := snap.RetryAttempts[issue.ID]; ok {
				ti.OrchestratorState = "retrying"
				if re.Error != nil {
					ti.Error = *re.Error
				}
			} else if _, paused := snap.PausedIdentifiers[issue.Identifier]; paused {
				ti.OrchestratorState = "paused"
			} else {
				ti.OrchestratorState = "idle"
				// IneligibleReason: only for active-state idle issues.
				// Calling IneligibleReason on terminal/inactive issues returns
				// "not_active_state" which is noise — filter those out.
				reason := orchestrator.IneligibleReason(issue, snap, cfg)
				if reason != "" && reason != "not_active_state" && reason != "terminal_state" {
					ti.IneligibleReason = reason
				}
			}
			result[i] = ti
		}
		return result, nil
	}

	// HTTP server — only started when server.port is configured.
	var srvDone <-chan error
	if cfg.Server.Port != nil {
		// pauseIssue cancels the running worker and marks it paused (no auto-retry).
		pauseIssue := func(identifier string) bool {
			issue := orch.GetRunningIssue(identifier)
			if issue == nil {
				return false
			}
			if !orch.CancelIssue(identifier) {
				return false
			}
			go func() {
				pauseCtx, pauseFn := context.WithTimeout(ctx, 30*time.Second)
				defer pauseFn()
				if err := tr.CreateComment(pauseCtx, issue.ID, "⏸ Agent paused via Symphony dashboard. Click Resume to continue."); err != nil {
					slog.Warn("pause: create comment failed", "identifier", identifier, "error", err)
				}
			}()
			return true
		}
		resumeIssue := tuiResume
		terminateIssue := func(identifier string) bool {
			ok := orch.TerminateIssue(identifier)
			if ok {
				// Safe to Refresh here: the web server has no TriggerPoll background
				// ticker, so there's no race with the async UpdateIssueState goroutine
				// in EventTerminatePaused (unlike the TUI path).
				orch.Refresh()
			}
			return ok
		}
		fetchLogs := func(identifier string) []string {
			return logBuf.Get(identifier)
		}
		clearLogs := func(identifier string) error {
			return logBuf.Clear(identifier)
		}
		srv := server.New(snap, refreshChan, logFile, fetchIssues, pauseIssue, resumeIssue, fetchLogs)
		srv.SetTerminateIssue(terminateIssue)
		srv.SetReanalyzeIssue(orch.ReanalyzeIssue)
		srv.SetClearLogs(clearLogs)
		if lc, ok := tr.(*linear.Client); ok {
			srv.SetProjectManager(&linearProjectManager{client: lc, workflowPath: workflowPath})
		}
		srv.SetDispatchReviewer(orch.DispatchReviewer)
		srv.SetWorkerSetter(func(n int) {
			orch.SetMaxWorkers(n)
			if err := workflow.PatchIntField(workflowPath, "max_concurrent_agents", n); err != nil {
				slog.Warn("failed to persist max_concurrent_agents to WORKFLOW.md", "error", err)
			}
		})
		srv.SetIssueProfileSetter(orch.SetIssueProfile)
		srv.SetProfileReader(func() map[string]server.ProfileDef {
			profiles := orch.ProfilesCfg()
			defs := make(map[string]server.ProfileDef, len(profiles))
			for name, p := range profiles {
				defs[name] = server.ProfileDef{
					Command: p.Command,
					Prompt:  p.Prompt,
				}
			}
			return defs
		})
		srv.SetProfileUpserter(func(name string, def server.ProfileDef) error {
			profiles := orch.ProfilesCfg()
			if profiles == nil {
				profiles = make(map[string]config.AgentProfile)
			}
			profiles[name] = config.AgentProfile{
				Command: def.Command,
				Prompt:  def.Prompt,
			}
			orch.SetProfilesCfg(profiles)
			entries := profilesToEntries(profiles)
			if err := workflow.PatchProfilesBlock(workflowPath, entries); err != nil {
				return err
			}
			srv.Notify()
			return nil
		})
		srv.SetProfileDeleter(func(name string) error {
			profiles := orch.ProfilesCfg()
			delete(profiles, name)
			orch.SetProfilesCfg(profiles)
			entries := profilesToEntries(profiles)
			if err := workflow.PatchProfilesBlock(workflowPath, entries); err != nil {
				return err
			}
			srv.Notify()
			return nil
		})
		srv.SetAgentModeSetter(func(mode string) error {
			orch.SetAgentModeCfg(mode)
			if err := workflow.PatchAgentStringField(workflowPath, "agent_mode", mode); err != nil {
				slog.Warn("failed to persist agent_mode to WORKFLOW.md", "error", err)
			}
			srv.Notify()
			return nil
		})
		srv.SetTrackerStateUpdater(func(active, terminal []string, completion string) error {
			orch.SetTrackerStatesCfg(active, terminal, completion)
			// Best-effort WORKFLOW.md persistence: keys may not exist if the user
			// hasn't configured them yet. In-memory update above is authoritative.
			if err := workflow.PatchStringSliceField(workflowPath, "active_states", active); err != nil {
				slog.Warn("could not patch active_states in WORKFLOW.md", "error", err)
			}
			if err := workflow.PatchStringSliceField(workflowPath, "terminal_states", terminal); err != nil {
				slog.Warn("could not patch terminal_states in WORKFLOW.md", "error", err)
			}
			if err := workflow.PatchStringField(workflowPath, "completion_state", completion); err != nil {
				slog.Warn("could not patch completion_state in WORKFLOW.md", "error", err)
			}
			srv.Notify()
			return nil
		})
		srv.SetUpdateIssueState(func(ctx context.Context, identifier, stateName string) error {
			seen := map[string]bool{}
			var allStates []string
			active, terminal, completion := orch.TrackerStatesCfg()
			base := append(append(append([]string{}, cfg.Tracker.BacklogStates...), active...), terminal...)
			if completion != "" {
				base = append(base, completion)
			}
			for _, s := range base {
				if !seen[s] {
					seen[s] = true
					allStates = append(allStates, s)
				}
			}
			issues, err := tr.FetchIssuesByStates(ctx, allStates)
			if err != nil {
				return fmt.Errorf("fetch issues: %w", err)
			}
			for _, iss := range issues {
				if iss.Identifier == identifier {
					return tr.UpdateIssueState(ctx, iss.ID, stateName)
				}
			}
			return fmt.Errorf("issue %s not found", identifier)
		})
		orch.OnStateChange = srv.Notify
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, *cfg.Server.Port)
		srvDone = serveHTTP(ctx, addr, srv)
		slog.Info("HTTP server listening", "addr", addr)
	}

	// Forward web dashboard refresh signals to the orchestrator for an immediate re-poll.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-refreshChan:
				slog.Debug("manual refresh requested")
				orch.Refresh()
			}
		}
	}()

	orchDone := make(chan error, 1)
	go func() { orchDone <- orch.Run(ctx) }()

	if srvDone != nil {
		select {
		case err := <-orchDone:
			return err
		case err := <-srvDone:
			return err
		}
	}
	return <-orchDone
}

// resolveAgentCommand resolves a bare command name (e.g. "claude") to its full
// absolute path using the user's interactive login shell, which sources .zshrc
// and therefore picks up PATH additions from nvm, volta, homebrew, etc.
// If the command is already absolute, or resolution fails, the original value
// is returned unchanged.
// profilesToEntries converts config.AgentProfile map to workflow.ProfileEntry map
// for persistence to WORKFLOW.md.
func profilesToEntries(profiles map[string]config.AgentProfile) map[string]workflow.ProfileEntry {
	entries := make(map[string]workflow.ProfileEntry, len(profiles))
	for name, p := range profiles {
		entries[name] = workflow.ProfileEntry{
			Command: p.Command,
			Prompt:  p.Prompt,
		}
	}
	return entries
}

func resolveAgentCommand(command string) string {
	if filepath.IsAbs(command) {
		return command
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	// -ilc: interactive (-i, sources .zshrc) + login (-l, sources .zprofile) + command (-c)
	out, err := exec.Command(shell, "-ilc", "command -v "+command).Output()
	if err != nil {
		slog.Warn("agent command resolution failed — using bare name; set agent.command to the full path if it fails",
			"command", command, "shell", shell, "error", err)
		return command
	}
	// Interactive shells may print init messages. Scan every line for either:
	//   /absolute/path          (binary on PATH)
	//   alias name=/abs/path    (shell alias — Claude Code installs this way)
	//   alias name='/abs/path'
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		l := strings.TrimSpace(line)
		if filepath.IsAbs(l) {
			slog.Info("agent command resolved", "command", l)
			return l
		}
		// alias foo=/path  or  alias foo='/path'  or  alias foo="/path"
		if strings.HasPrefix(l, "alias ") {
			if _, val, ok := strings.Cut(l, "="); ok {
				val = strings.Trim(val, `'"`)
				if filepath.IsAbs(val) {
					slog.Info("agent command resolved from alias", "command", val)
					return val
				}
			}
		}
	}
	slog.Warn("could not resolve agent command; using bare name — set agent.command to the full path if this fails",
		"command", command, "shell_output", strings.TrimSpace(string(out)))
	return command
}

// linearProjectManager adapts *linear.Client to server.ProjectManager.
// It translates linear.LinearProject → server.Project on the fly.
type linearProjectManager struct {
	client       *linear.Client
	workflowPath string
}

// FetchProjects implements server.ProjectManager by delegating to the Linear client.
func (m *linearProjectManager) FetchProjects(ctx context.Context) ([]server.Project, error) {
	lps, err := m.client.FetchProjects(ctx)
	if err != nil {
		return nil, err
	}
	projects := make([]server.Project, len(lps))
	for i, lp := range lps {
		projects[i] = server.Project{ID: lp.ID, Name: lp.Name, Slug: lp.Slug}
	}
	return projects, nil
}

// SetProjectFilter implements server.ProjectManager and persists the filter to WORKFLOW.md.
func (m *linearProjectManager) SetProjectFilter(slugs []string) {
	m.client.SetProjectFilter(slugs)
	if m.workflowPath != "" {
		updateWorkflowProjectSlug(m.workflowPath, slugs)
	}
}

// GetProjectFilter implements server.ProjectManager.
func (m *linearProjectManager) GetProjectFilter() []string { return m.client.GetProjectFilter() }

// buildTracker constructs the correct tracker adapter from config.
func buildTracker(cfg *config.Config) (tracker.Tracker, error) {
	switch cfg.Tracker.Kind {
	case "linear":
		return linear.NewClient(linear.ClientConfig{
			APIKey:         cfg.Tracker.APIKey,
			ProjectSlug:    cfg.Tracker.ProjectSlug,
			ActiveStates:   cfg.Tracker.ActiveStates,
			TerminalStates: cfg.Tracker.TerminalStates,
			Endpoint:       cfg.Tracker.Endpoint,
		}), nil
	case "github":
		return github.NewClient(github.ClientConfig{
			APIKey:         cfg.Tracker.APIKey,
			ProjectSlug:    cfg.Tracker.ProjectSlug,
			ActiveStates:   cfg.Tracker.ActiveStates,
			TerminalStates: cfg.Tracker.TerminalStates,
			BacklogStates:  cfg.Tracker.BacklogStates,
			Endpoint:       cfg.Tracker.Endpoint,
		}), nil
	default:
		return nil, fmt.Errorf("unknown tracker kind %q (supported: linear, github)", cfg.Tracker.Kind)
	}
}

// runClear removes workspace directories for one or more issues, or all workspaces
// under workspace.root when no identifiers are given.
//
// Usage:
//
//	symphony clear [--workflow WORKFLOW.md] [identifier ...]
//
// With no identifiers, all subdirectories under workspace.root are removed.
// With identifiers, only those specific workspace directories are removed.
func runClear(args []string) {
	fs := flag.NewFlagSet("clear", flag.ExitOnError)
	workflowPath := fs.String("workflow", "WORKFLOW.md", "path to WORKFLOW.md (to read workspace.root)")
	_ = fs.Parse(args)
	identifiers := fs.Args()

	cfg, err := config.Load(*workflowPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "symphony clear: load config %s: %v\n", *workflowPath, err)
		os.Exit(1)
	}

	root := cfg.Workspace.Root

	if len(identifiers) == 0 {
		// Remove all entries under workspace.root.
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("symphony clear: workspace root %s does not exist — nothing to clear\n", root)
				return
			}
			fmt.Fprintf(os.Stderr, "symphony clear: read dir %s: %v\n", root, err)
			os.Exit(1)
		}
		removed := 0
		for _, e := range entries {
			path := filepath.Join(root, e.Name())
			if err := os.RemoveAll(path); err != nil {
				fmt.Fprintf(os.Stderr, "symphony clear: remove %s: %v\n", path, err)
			} else {
				fmt.Printf("  removed %s\n", path)
				removed++
			}
		}
		fmt.Printf("symphony clear: removed %d workspace(s) from %s\n", removed, root)
		return
	}

	// Remove only the specified identifiers.
	wm := workspace.NewManager(cfg)
	for _, id := range identifiers {
		path := workspace.WorkspacePath(root, id)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("  skip %s (not found)\n", path)
			continue
		}
		if err := wm.RemoveWorkspace(id); err != nil {
			fmt.Fprintf(os.Stderr, "symphony clear: remove %s: %v\n", path, err)
		} else {
			fmt.Printf("  removed %s\n", path)
		}
	}
}

// repoInfo holds values discovered by scanning the current directory.
type repoInfo struct {
	RemoteURL     string // raw git remote URL
	Owner         string // e.g. "vnovick"
	Repo          string // e.g. "simphony"
	CloneURL      string // SSH clone URL reconstructed for after_create hook
	DefaultBranch string // "main" or "master"
	ProjectName   string // repo name, used for workspace.root
	HasClaudeMD   bool   // CLAUDE.md present in dir
	Stacks        []detectedStack
}

type detectedStack struct {
	Name     string
	Commands []string
}

// scanRepo inspects dir (typically ".") for git remote, branch, CLAUDE.md, and
// language/framework indicators. All fields fall back to sensible placeholders
// so the output is always valid even in a non-git directory.
func scanRepo(dir string) repoInfo {
	info := repoInfo{DefaultBranch: "main", ProjectName: "my-project"}

	// ── git remote ────────────────────────────────────────────────────────────
	if out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output(); err == nil {
		info.RemoteURL = strings.TrimSpace(string(out))
		info.Owner, info.Repo = parseGitRemote(info.RemoteURL)
		if info.Repo != "" {
			info.ProjectName = info.Repo
		}
		// Normalise to SSH clone URL for the after_create hook.
		if info.Owner != "" && info.Repo != "" {
			info.CloneURL = fmt.Sprintf("git@github.com:%s/%s.git", info.Owner, info.Repo)
		}
	}

	// ── default branch ────────────────────────────────────────────────────────
	if out, err := exec.Command("git", "-C", dir, "symbolic-ref", "refs/remotes/origin/HEAD").Output(); err == nil {
		ref := strings.TrimSpace(string(out)) // refs/remotes/origin/main
		if parts := strings.Split(ref, "/"); len(parts) > 0 {
			info.DefaultBranch = parts[len(parts)-1]
		}
	}

	// ── CLAUDE.md ─────────────────────────────────────────────────────────────
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
		info.HasClaudeMD = true
	}

	// ── tech stack ────────────────────────────────────────────────────────────
	info.Stacks = detectStacks(dir)

	return info
}

// parseGitRemote extracts owner and repo from an SSH or HTTPS git remote URL.
func parseGitRemote(remote string) (owner, repo string) {
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	// SSH: git@github.com:owner/repo
	if strings.HasPrefix(remote, "git@") {
		if _, path, ok := strings.Cut(remote, ":"); ok {
			owner, repo, _ = strings.Cut(path, "/")
			return
		}
	}
	// HTTPS: https://github.com/owner/repo
	parts := strings.Split(remote, "/")
	if len(parts) >= 2 {
		repo = parts[len(parts)-1]
		owner = parts[len(parts)-2]
	}
	return
}

// detectStacks scans dir for language/framework indicator files and returns
// the detected stacks with their suggested check commands.
func detectStacks(dir string) []detectedStack {
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	var stacks []detectedStack

	if has("go.mod") {
		stacks = append(stacks, detectedStack{
			Name:     "Go",
			Commands: []string{"go test ./...", "go vet ./..."},
		})
	}

	if has("package.json") {
		stacks = append(stacks, detectedStack{
			Name:     "Node.js",
			Commands: detectNodeCommands(dir),
		})
	}

	if has("Cargo.toml") {
		stacks = append(stacks, detectedStack{
			Name:     "Rust",
			Commands: []string{"cargo test", "cargo clippy -- -D warnings"},
		})
	}

	if has("pyproject.toml") || has("setup.py") || has("requirements.txt") {
		stacks = append(stacks, detectedStack{
			Name:     "Python",
			Commands: []string{"python -m pytest", "python -m mypy ."},
		})
	}

	if has("mix.exs") {
		stacks = append(stacks, detectedStack{
			Name:     "Elixir",
			Commands: []string{"mix test", "mix credo"},
		})
	}

	if has("Gemfile") {
		stacks = append(stacks, detectedStack{
			Name:     "Ruby",
			Commands: []string{"bundle exec rspec", "bundle exec rubocop"},
		})
	}

	return stacks
}

// detectNodeCommands reads package.json scripts to suggest the right test/lint commands.
func detectNodeCommands(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return []string{"npm test"}
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return []string{"npm test"}
	}

	// Detect package manager from lock files.
	pm := "npm"
	if _, err := os.Stat(filepath.Join(dir, "pnpm-lock.yaml")); err == nil {
		pm = "pnpm"
	} else if _, err := os.Stat(filepath.Join(dir, "yarn.lock")); err == nil {
		pm = "yarn"
	}

	var cmds []string
	for _, script := range []string{"test", "lint", "typecheck", "check", "build"} {
		if _, ok := pkg.Scripts[script]; ok {
			cmds = append(cmds, pm+" run "+script)
		}
	}
	if len(cmds) == 0 {
		cmds = []string{pm + " test"}
	}
	return cmds
}

// generateWorkflow builds the WORKFLOW.md content from scanned repo info.
func generateWorkflow(trackerKind string, info repoInfo) string {
	var b strings.Builder

	// ── frontmatter ───────────────────────────────────────────────────────────
	b.WriteString("---\n")
	b.WriteString("tracker:\n")
	b.WriteString("  kind: " + trackerKind + "\n")

	switch trackerKind {
	case "linear":
		b.WriteString("  api_key: $LINEAR_API_KEY          # export LINEAR_API_KEY=lin_api_...\n")
	case "github":
		b.WriteString("  api_key: $GITHUB_TOKEN            # export GITHUB_TOKEN=ghp_...\n")
	}

	slug := "owner/repo"
	if info.Owner != "" && info.Repo != "" {
		if trackerKind == "linear" {
			slug = info.Repo + "-<slug>"
		} else {
			slug = info.Owner + "/" + info.Repo
		}
	}
	if trackerKind == "linear" {
		b.WriteString("  # project_slug: <slug>  # Optional — filter to one project.\n")
		b.WriteString("  #                        Select interactively via TUI (p) or web dashboard instead.\n")
		b.WriteString("  active_states: [\"Todo\", \"In Progress\"]\n")
		b.WriteString("  terminal_states: [\"Done\", \"Cancelled\", \"Duplicate\"]\n")
		b.WriteString("  working_state: \"In Progress\"     # State applied when an agent starts working.\n")
		b.WriteString("  #                                  # Set to \"\" to disable auto-transition.\n")
		b.WriteString("  completion_state: \"In Review\"     # State applied when the agent finishes.\n")
		b.WriteString("  backlog_states: [\"Backlog\"]        # Discard target; shown in TUI (b) and Kanban; not auto-dispatched.\n")
	} else {
		b.WriteString("  project_slug: " + slug + "\n")
		b.WriteString("  # GitHub uses labels to map states. Labels must exist in your repo.\n")
		b.WriteString("  # NOTE: GitHub Projects v2 'Status' field is separate from labels — Symphony\n")
		b.WriteString("  #       only reads labels. See README for Projects automation setup.\n")
		b.WriteString("  # Create them with: gh label create \"todo\" --color \"0075ca\" --repo " + slug + "\n")
		b.WriteString("  #                   gh label create \"in-progress\" --color \"e4e669\" --repo " + slug + "\n")
		b.WriteString("  #                   gh label create \"in-review\" --color \"d93f0b\" --repo " + slug + "\n")
		b.WriteString("  #                   gh label create \"done\" --color \"0e8a16\" --repo " + slug + "\n")
		b.WriteString("  #                   gh label create \"cancelled\" --color \"cccccc\" --repo " + slug + "\n")
		b.WriteString("  #                   gh label create \"backlog\" --color \"f9f9f9\" --repo " + slug + "\n")
		b.WriteString("  active_states: [\"todo\", \"in-progress\"]\n")
		b.WriteString("  terminal_states: [\"done\", \"cancelled\"]\n")
		b.WriteString("  working_state: \"in-progress\"  # Label applied when an agent starts.\n")
		b.WriteString("  #                               # MUST exist as a label in your repo.\n")
		b.WriteString("  #                               # Set to \"\" to disable, or reuse an active label.\n")
		b.WriteString("  completion_state: \"in-review\"  # Label applied when the agent finishes.\n")
		b.WriteString("  # backlog_states: [\"backlog\"]  # Shown in TUI (b) and Kanban; not auto-dispatched.\n")
		b.WriteString("  #                               # Must be an array — not a bare string.\n")
		b.WriteString("  backlog_states: [\"backlog\"]\n")
	}

	b.WriteString("\npolling:\n  interval_ms: 60000\n")
	b.WriteString("\nagent:\n  max_turns: 60\n  max_concurrent_agents: 3\n  turn_timeout_ms: 3600000\n  read_timeout_ms: 120000\n  stall_timeout_ms: 300000\n")

	b.WriteString("\nworkspace:\n  root: ~/.simphony/workspaces/" + info.ProjectName + "\n")

	cloneURL := info.CloneURL
	if cloneURL == "" {
		cloneURL = "git@github.com:owner/" + info.ProjectName + ".git"
	}
	branch := info.DefaultBranch
	b.WriteString("\nhooks:\n")
	b.WriteString("  after_create: |\n")
	b.WriteString("    git clone " + cloneURL + " .\n")
	b.WriteString("  before_run: |\n")
	b.WriteString("    git fetch origin\n")
	b.WriteString("    git checkout -B " + branch + " origin/" + branch + "\n")
	b.WriteString("    git reset --hard origin/" + branch + "\n")

	b.WriteString("\nserver:\n  port: 8090\n")
	b.WriteString("---\n\n")

	// ── prompt body ───────────────────────────────────────────────────────────
	b.WriteString("You are an expert engineer working on **" + info.ProjectName + "**.\n\n")

	b.WriteString("## Your issue\n\n")
	b.WriteString("**{{ issue.identifier }}: {{ issue.title }}**\n\n")
	b.WriteString("{% if issue.description %}\n{{ issue.description }}\n{% endif %}\n\n")
	b.WriteString("Issue URL: {{ issue.url }}\n\n")
	b.WriteString("{% if issue.comments %}\n## Comments\n\n")
	b.WriteString("{% for comment in issue.comments %}\n**{{ comment.author_name }}**: {{ comment.body }}\n\n{% endfor %}\n{% endif %}\n\n")
	b.WriteString("---\n\n")

	// CLAUDE.md or conventions placeholder
	if info.HasClaudeMD {
		b.WriteString("## Project Conventions\n\n")
		b.WriteString("This project has a `CLAUDE.md`. Read it before touching any code:\n\n")
		b.WriteString("```bash\ncat CLAUDE.md\n```\n\n")
		b.WriteString("Follow all conventions, architecture rules, and preferences documented there.\n\n")
		b.WriteString("---\n\n")
	}

	b.WriteString("## Step 1 — Explore before touching anything\n\n")
	b.WriteString("Read the issue. Explore the relevant code before making changes.\n\n")
	if info.HasClaudeMD {
		b.WriteString("Re-read `CLAUDE.md` if you are unsure about conventions.\n\n")
	}
	b.WriteString("---\n\n")

	b.WriteString("## Step 2 — Create a branch\n\n")
	b.WriteString("```bash\n")
	if trackerKind == "linear" {
		b.WriteString("git checkout -b {{ issue.branch_name | default: issue.identifier | downcase }}\n")
	} else {
		b.WriteString("git checkout -b {{ issue.branch_name | default: issue.identifier | replace: \"#\", \"\" | downcase }}\n")
	}
	b.WriteString("```\n\n---\n\n")

	b.WriteString("## Step 3 — Implement\n\n")
	b.WriteString("Read `CLAUDE.md` to understand project conventions before writing any code:\n\n")
	b.WriteString("```bash\ncat CLAUDE.md\n```\n\n")
	b.WriteString("If `CLAUDE.md` does not exist, explore the repository structure, identify the dominant patterns and conventions, create `CLAUDE.md` documenting them, and then implement.\n\n")
	if len(info.Stacks) > 0 {
		b.WriteString("Detected stacks: ")
		for i, s := range info.Stacks {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(s.Name)
		}
		b.WriteString(". Follow their conventions as documented in `CLAUDE.md`.\n\n")
	}
	b.WriteString("---\n\n")

	b.WriteString("## Step 4 — Run checks\n\n")
	b.WriteString("Read `CLAUDE.md` for the project's test and lint commands. If `CLAUDE.md` does not exist, discover the check commands by exploring the repository (look for `Makefile`, `package.json` scripts, CI config, etc.).\n\n")
	b.WriteString("```bash\n")
	if len(info.Stacks) > 0 {
		for _, s := range info.Stacks {
			b.WriteString("# " + s.Name + "\n")
			for _, cmd := range s.Commands {
				b.WriteString(cmd + "\n")
			}
		}
	} else {
		b.WriteString("# Run the project's test and lint commands (check CLAUDE.md or discover from repo)\n")
	}
	b.WriteString("```\n\n---\n\n")

	b.WriteString("## Step 5 — Commit and open PR\n\n")
	b.WriteString("```bash\n")
	b.WriteString("git add <specific files>\n")
	b.WriteString("git commit -m \"feat: <description> ({{ issue.identifier }})\"\n")
	b.WriteString("git push -u origin HEAD\n")
	b.WriteString("gh pr create --title \"<title> ({{ issue.identifier }})\" --body \"Closes {{ issue.url }}\"\n")
	b.WriteString("```\n\n---\n\n")

	b.WriteString("## Step 6 — Post PR link to tracker\n\n")
	b.WriteString("After the PR is open, post its URL as a comment on the tracker issue so it is visible in ")
	if trackerKind == "linear" {
		b.WriteString("Linear:\n\n")
		b.WriteString("```bash\n")
		b.WriteString("PR_URL=$(gh pr view --json url -q .url)\n")
		b.WriteString("curl -s -X POST https://api.linear.app/graphql \\\n")
		b.WriteString("  -H \"Authorization: $LINEAR_API_KEY\" \\\n")
		b.WriteString("  -H \"Content-Type: application/json\" \\\n")
		b.WriteString("  -d \"{\\\"query\\\":\\\"mutation { commentCreate(input: { issueId: \\\\\\\"{{ issue.id }}\\\\\\\", body: \\\\\\\"PR: ${PR_URL}\\\\\\\" }) { success } }\\\"}\"\n")
		b.WriteString("```\n\n---\n\n")
	} else {
		b.WriteString("GitHub:\n\n")
		b.WriteString("```bash\n")
		b.WriteString("PR_URL=$(gh pr view --json url -q .url)\n")
		b.WriteString("gh issue comment {{ issue.identifier | remove: \"#\" }} --body \"🤖 Opened PR: ${PR_URL}\"\n")
		b.WriteString("```\n\n---\n\n")
	}

	b.WriteString("## Rules\n\n")
	b.WriteString("- Complete the issue fully before stopping.\n")
	b.WriteString("- Never commit `.env` files or secrets.\n")
	if info.HasClaudeMD {
		b.WriteString("- All conventions in `CLAUDE.md` apply — do not deviate without a documented reason.\n")
	}
	b.WriteString("\n")

	return b.String()
}

// updateWorkflowProjectSlug rewrites the project_slug line in the YAML frontmatter
// of the given WORKFLOW.md path. If slugs is nil or empty, the line is commented out.
// Silently ignores errors (the filter is applied in-memory regardless).
func updateWorkflowProjectSlug(path string, slugs []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	inFrontmatter := false
	fmCount := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			fmCount++
			if fmCount == 1 {
				inFrontmatter = true
				continue
			}
			break // second --- ends frontmatter
		}
		if !inFrontmatter {
			continue
		}
		// Match both commented and uncommented project_slug lines.
		stripped := strings.TrimLeft(line, " #")
		if !strings.HasPrefix(stripped, "project_slug:") {
			continue
		}
		// Determine indentation (spaces before # or p).
		indent := strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " ")))
		if len(slugs) == 0 {
			lines[i] = indent + "# project_slug:  # Optional — select interactively via TUI (p) or web dashboard"
		} else {
			lines[i] = indent + "project_slug: " + strings.Join(slugs, ", ")
		}
		break
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// runInit scans the current (or specified) directory for repo metadata and
// generates a WORKFLOW.md pre-filled with discovered values.
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	trackerKind := fs.String("tracker", "", "tracker kind: linear or github (required)")
	output := fs.String("output", "WORKFLOW.md", "output file path")
	dir := fs.String("dir", ".", "directory to scan for repo metadata")
	force := fs.Bool("force", false, "overwrite output file if it already exists")
	_ = fs.Parse(args)

	switch *trackerKind {
	case "linear", "github":
		// valid
	case "":
		fmt.Fprintln(os.Stderr, "symphony init: --tracker is required (linear or github)")
		fs.Usage()
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "symphony init: unknown tracker %q (supported: linear, github)\n", *trackerKind)
		os.Exit(1)
	}

	if _, err := os.Stat(*output); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "symphony init: %s already exists (use --force to overwrite)\n", *output)
		os.Exit(1)
	}

	fmt.Printf("symphony init: scanning %s...\n", *dir)
	info := scanRepo(*dir)

	if info.RemoteURL != "" {
		fmt.Printf("  git remote : %s\n", info.RemoteURL)
	}
	fmt.Printf("  branch     : %s\n", info.DefaultBranch)
	if info.HasClaudeMD {
		fmt.Printf("  CLAUDE.md  : found — prompt will reference it\n")
	} else {
		fmt.Printf("  CLAUDE.md  : not found — add one for best results\n")
	}
	for _, s := range info.Stacks {
		fmt.Printf("  stack      : %s (%s)\n", s.Name, strings.Join(s.Commands, ", "))
	}

	content := generateWorkflow(*trackerKind, info)

	if err := os.WriteFile(*output, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "symphony init: write %s: %v\n", *output, err)
		os.Exit(1)
	}
	fmt.Printf("symphony init: wrote %s\n", *output)
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Edit %s — fill in api_key\n", *output)
	if *trackerKind == "linear" {
		fmt.Printf("  2. Run: symphony -workflow %s\n", *output)
		fmt.Printf("  3. Select a project via the TUI (press p) or the web dashboard\n")
	} else {
		fmt.Printf("  2. Run: symphony -workflow %s\n", *output)
	}
}

// serveHTTP starts an HTTP server and returns a channel that receives its exit error.
func serveHTTP(ctx context.Context, addr string, handler http.Handler) <-chan error {
	errCh := make(chan error, 1)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		errCh <- fmt.Errorf("http listen %s: %w", addr, err)
		return errCh
	}

	srv := &http.Server{
		Addr:        addr,
		Handler:     handler,
		ReadTimeout: 5 * time.Second,
		// WriteTimeout is intentionally 0 (no deadline) so the SSE /api/v1/events
		// endpoint can stream indefinitely. Per-route write timeouts should use
		// http.TimeoutHandler for non-SSE handlers if needed in future.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	return errCh
}
