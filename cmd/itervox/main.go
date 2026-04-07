package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	charmlog "github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/agent/agenttest"
	"github.com/vnovick/itervox/internal/app"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/logbuffer"
	"github.com/vnovick/itervox/internal/logging"
	"github.com/vnovick/itervox/internal/orchestrator"
	"github.com/vnovick/itervox/internal/server"
	"github.com/vnovick/itervox/internal/statusui"
	"github.com/vnovick/itervox/internal/tracker"
	"github.com/vnovick/itervox/internal/tracker/github"
	"github.com/vnovick/itervox/internal/tracker/linear"
	"github.com/vnovick/itervox/internal/workflow"
	"github.com/vnovick/itervox/internal/workspace"
	"gopkg.in/lumberjack.v2"
)

// Set by GoReleaser via ldflags — empty when built with `go build`
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newAppSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: itervox [command] [flags]

Commands:
  init    Scan a repository and generate a WORKFLOW.md starter file
             --tracker  linear|github  (required)
             --runner   claude|codex    (default: claude)
             --output   output file path (default: WORKFLOW.md)
             --dir      directory to scan (default: .)
             --force    overwrite existing output file

  clear   Remove workspace directories created by itervox
             --workflow path to WORKFLOW.md (default: WORKFLOW.md)
             [identifier ...]  specific issues to clear; omit for all

  --version  Print version information

Run mode (default when no command given):
`)
	flag.PrintDefaults()
}

// defaultLogsDir returns a per-project logs directory under ~/.itervox/logs/
// derived from the tracker kind and project slug in the WORKFLOW.md at path.
// Falls back to ~/.itervox/logs if the config can't be read or has no slug.
func defaultLogsDir(workflowPath string) string {
	base := filepath.Join("~", ".itervox", "logs")
	if home, err := os.UserHomeDir(); err == nil {
		base = filepath.Join(home, ".itervox", "logs")
	}
	cfg, err := config.Load(workflowPath)
	if err != nil || cfg.Tracker.Kind == "" || cfg.Tracker.ProjectSlug == "" {
		return base
	}
	// Encode the slug so it is safe as a directory name component.
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_").Replace(cfg.Tracker.ProjectSlug)
	return filepath.Join(base, cfg.Tracker.Kind, safe)
}

func convertAgentModels(models []agent.ModelOption) []config.ModelOption {
	out := make([]config.ModelOption, len(models))
	for i, m := range models {
		out[i] = config.ModelOption{ID: m.ID, Label: m.Label}
	}
	return out
}

func convertModelsForSnapshot(models map[string][]config.ModelOption) map[string][]server.ModelOption {
	if len(models) == 0 {
		return nil
	}
	result := make(map[string][]server.ModelOption, len(models))
	for backend, opts := range models {
		converted := make([]server.ModelOption, len(opts))
		for i, m := range opts {
			converted[i] = server.ModelOption{ID: m.ID, Label: m.Label}
		}
		result[backend] = converted
	}
	return result
}

// buildDemoConfig creates a config for demo mode — no WORKFLOW.md needed.
func buildDemoConfig() *config.Config {
	port := 8090
	return &config.Config{
		Tracker: config.TrackerConfig{
			Kind:            "memory",
			ActiveStates:    []string{"Todo", "In Progress"},
			TerminalStates:  []string{"Done", "Cancelled"},
			BacklogStates:   []string{"Backlog"},
			WorkingState:    "In Progress",
			CompletionState: "Done",
		},
		Polling: config.PollingConfig{
			IntervalMs: 10000,
		},
		Agent: config.AgentConfig{
			Command:             "demo-agent",
			MaxConcurrentAgents: 3,
			MaxTurns:            5,
			MaxRetries:          2,
			TurnTimeoutMs:       60000,
			ReadTimeoutMs:       30000,
			StallTimeoutMs:      30000,
			AvailableModels: map[string][]config.ModelOption{
				"claude": convertAgentModels(agent.DefaultClaudeModels),
				"codex":  convertAgentModels(agent.DefaultCodexModels),
			},
		},
		Workspace: config.WorkspaceConfig{
			Root: filepath.Join(os.TempDir(), "itervox-demo"),
		},
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: &port,
		},
		PromptTemplate: "You are a demo AI agent working on {{ issue.identifier }}: {{ issue.title }}.\n\n{{ issue.description }}\n\nThis is a demo — no real changes will be made.",
	}
}

func configuredBackend(command, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if backend := agent.BackendFromCommand(command); backend != "" {
		return backend
	}
	return "claude"
}

// validateBackend checks that the CLI for the requested agent backend is
// present and accessible. validatedBackends is a dedup set so each backend
// is validated at most once per startup. profileName is "" for the default
// agent and non-empty for named profiles (affects log messages only).
// Returns a non-nil error when the default backend fails validation so the
// caller can abort startup rather than silently waiting until dispatch time.
func validateBackend(backend, profileName string, validatedBackends map[string]struct{}, cfg *config.Config) error {
	switch backend {
	case "", "claude":
		if _, ok := validatedBackends["claude"]; ok {
			return nil
		}
		validatedBackends["claude"] = struct{}{}
		// Use the already-resolved command (absolute path or bare name) so
		// validation runs the same binary that will actually be executed,
		// not the bare name which may not be on PATH in a login shell.
		resolvedCmd := cfg.Agent.Command
		if profileName != "" {
			if p, ok := cfg.Agent.Profiles[profileName]; ok && p.Command != "" {
				resolvedCmd = p.Command
			}
		}
		if err := agent.ValidateClaudeCLICommand(resolvedCmd); err != nil {
			if profileName != "" {
				slog.Warn("claude CLI validation failed for profile", "profile", profileName, "error", err)
				return nil // profile failures are non-fatal
			}
			return fmt.Errorf("claude CLI not found or not executable: %w", err)
		}
		if profileName != "" {
			slog.Info("claude CLI validated successfully for profile", "profile", profileName)
		} else {
			slog.Info("claude CLI validated successfully")
		}
	case "codex":
		if _, ok := validatedBackends["codex"]; ok {
			return nil
		}
		validatedBackends["codex"] = struct{}{}
		resolvedCmd := cfg.Agent.Command
		if profileName != "" {
			if p, ok := cfg.Agent.Profiles[profileName]; ok && p.Command != "" {
				resolvedCmd = p.Command
			}
		}
		if err := agent.ValidateCodexCLICommand(resolvedCmd); err != nil {
			if profileName != "" {
				slog.Warn("codex CLI validation failed for profile", "profile", profileName, "error", err)
				return nil // profile failures are non-fatal
			}
			return fmt.Errorf("codex CLI not found or not executable: %w", err)
		}
		if profileName != "" {
			slog.Info("codex CLI validated successfully for profile", "profile", profileName)
		} else {
			slog.Info("codex CLI validated successfully")
		}
	default:
		if profileName != "" {
			slog.Warn("unsupported backend in profile, will fall back to default runner", "profile", profileName, "backend", backend)
		} else {
			slog.Warn("unsupported default backend, will fall back to default runner", "backend", backend)
		}
	}
	return nil
}

// loadDotEnv silently loads .itervox/.env then .env from the current working
// directory, injecting missing variables into the process environment.
// Existing environment variables are never overwritten.
func loadDotEnv() {
	candidates := []string{
		filepath.Join(".itervox", ".env"),
		".env",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if err := godotenv.Load(p); err != nil {
				slog.Warn("dotenv: failed to load", "path", p, "err", err)
			} else {
				slog.Debug("dotenv: loaded", "path", p)
			}
			return // stop at first file found
		}
	}
}

func main() {
	loadDotEnv() // must run before config.LoadConfig / os.Getenv calls
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			runInit(os.Args[2:])
			return
		case "clear":
			runClear(os.Args[2:])
			return
		case "--version", "-version":
			fmt.Printf("itervox %s (commit: %s, built: %s)\n", version, commit, date)
			return
		case "help", "--help", "-help", "-h":
			printUsage()
			return
		}
	}

	flag.Usage = printUsage
	workflowPath := flag.String("workflow", "WORKFLOW.md", "path to WORKFLOW.md")
	logsDir := flag.String("logs-dir", "", "directory for rotating log files (default: ~/.itervox/logs/<kind>/<project>)")
	verbose := flag.Bool("verbose", false, "enable DEBUG-level logging (includes Claude output)")
	demo := flag.Bool("demo", false, "run in demo mode with synthetic issues and no real agent (no API key or CLI needed)")
	shutdownGrace := flag.Duration("shutdown-grace", 30*time.Second, "grace period for active workers on SIGINT/SIGTERM before force exit")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	// Resolve the logs directory.  When --logs-dir is not set we derive a
	// per-project path under ~/.itervox/logs/<kind>/<slug> so that logs are
	// co-located with workspaces and automatically scoped to the project.
	// We do a lightweight early config read solely to get the tracker kind and
	// project slug; failures are non-fatal and fall back to a shared default.
	resolvedLogsDir := *logsDir
	if resolvedLogsDir == "" {
		resolvedLogsDir = defaultLogsDir(*workflowPath)
	}

	// Tee logs to stderr and a rotating file under <logs-dir>/itervox.log.
	if err := os.MkdirAll(resolvedLogsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logs dir %s: %v\n", resolvedLogsDir, err)
		os.Exit(1)
	}
	rotatingFile := &lumberjack.Logger{
		Filename:   filepath.Join(resolvedLogsDir, "itervox.log"),
		MaxSize:    10, // MB
		MaxBackups: 5,
		Compress:   true,
	}
	// Colored handler for stderr (auto-detects TTY for ANSI colors).
	charmLevel := charmlog.InfoLevel
	if logLevel == slog.LevelDebug {
		charmLevel = charmlog.DebugLevel
	}
	stderrHandler := charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		ReportTimestamp: true,
		TimeFormat:      time.TimeOnly,
		Level:           charmLevel,
	})
	// Plain text handler for the rotating log file (no colors).
	fileHandler := slog.NewTextHandler(rotatingFile, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(logging.NewFanoutHandler(stderrHandler, fileHandler)))
	slog.Info("itervox starting", "version", version, "commit", commit, "date", date)
	slog.Info("logging to file", "path", rotatingFile.Filename)

	// Top-level context: cancelled on first SIGINT/SIGTERM to begin graceful drain.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Outer loop: restart when WORKFLOW.md changes.
	for {
		var cfg *config.Config
		if *demo {
			cfg = buildDemoConfig()
			slog.Info("itervox: demo mode — using synthetic issues and fake agent runner")
		} else {
			var err error
			cfg, err = config.Load(*workflowPath)
			if err != nil {
				slog.Error("failed to load config", "path", *workflowPath, "error", err)
				os.Exit(1)
			}
			if err := config.ValidateDispatch(cfg); err != nil {
				slog.Error("config validation failed", "path", *workflowPath, "error", err)
				os.Exit(1)
			}
		}

		// Auto-discover models at startup when WORKFLOW.md doesn't have available_models.
		// This ensures the dashboard model dropdown is populated even for pre-existing configs.
		if len(cfg.Agent.AvailableModels) == 0 {
			claudeModels := agent.ListClaudeModels()
			codexModels := agent.ListCodexModels()
			cfg.Agent.AvailableModels = map[string][]config.ModelOption{
				"claude": make([]config.ModelOption, len(claudeModels)),
				"codex":  make([]config.ModelOption, len(codexModels)),
			}
			for i, m := range claudeModels {
				cfg.Agent.AvailableModels["claude"][i] = config.ModelOption{ID: m.ID, Label: m.Label}
			}
			for i, m := range codexModels {
				cfg.Agent.AvailableModels["codex"][i] = config.ModelOption{ID: m.ID, Label: m.Label}
			}
			slog.Info("models auto-discovered", "claude", len(claudeModels), "codex", len(codexModels))
		}

		runCtx, runCancel := context.WithCancel(ctx)

		// Watch WORKFLOW.md; cancel runCtx to trigger reload on change.
		// Skip in demo mode (no WORKFLOW.md to watch).
		if !*demo {
			go func() {
				if err := workflow.Watch(runCtx, *workflowPath, runCancel); err != nil && runCtx.Err() == nil {
					slog.Warn("workflow watcher stopped", "error", err)
				}
			}()
		}

		runDone := make(chan error, 1)
		go func() {
			runDone <- run(runCtx, cfg, *workflowPath, rotatingFile.Filename, rotatingFile, logLevel, *demo)
		}()

		// Wait for run to finish or a signal to arrive.
		select {
		case err := <-runDone:
			runCancel()
			if ctx.Err() != nil {
				return // top-level shutdown already in progress
			}
			if err != nil {
				slog.Warn("run returned, restarting", "error", err)
			}
		case sig := <-sigCh:
			slog.Info("shutting down gracefully, waiting for active workers...", "signal", sig, "grace", shutdownGrace.String())
			cancel()    // cancel top-level ctx → stops dispatching new work
			runCancel() // also cancel runCtx

			// Wait for run to finish within grace period, or force-exit on second signal / timeout.
			graceTimer := time.NewTimer(*shutdownGrace)
			defer graceTimer.Stop()
			select {
			case <-runDone:
				slog.Info("all workers finished, exiting")
			case <-graceTimer.C:
				slog.Warn("grace period expired, forcing exit")
			case sig2 := <-sigCh:
				slog.Warn("received second signal, forcing exit", "signal", sig2)
			}
			return
		}

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
func run(ctx context.Context, cfg *config.Config, workflowPath string, logFile string, fileWriter io.Writer, logLevel slog.Level, demoMode bool) error {
	tr, err := buildTracker(cfg)
	if err != nil {
		return fmt.Errorf("build tracker: %w", err)
	}

	cfg.Agent.Command = resolveAgentCommand(cfg.Agent.Command)
	for name, profile := range cfg.Agent.Profiles {
		if profile.Command != "" {
			// Extract the binary name (first token) and resolve it, keeping flags.
			parts := strings.SplitN(profile.Command, " ", 2)
			resolved := resolveAgentCommand(parts[0])
			if resolved != parts[0] {
				if len(parts) > 1 {
					profile.Command = resolved + " " + parts[1]
				} else {
					profile.Command = resolved
				}
				cfg.Agent.Profiles[name] = profile
			}
		}
	}

	var runner agent.Runner
	if demoMode {
		runner = agenttest.NewDemoRunner(5 * time.Second)
	} else {
		runner = agent.NewMultiRunner(
			agent.NewClaudeRunner(),
			map[string]agent.Runner{
				"codex": agent.NewCodexRunner(),
			},
		)
	}

	// Validate CLI availability for the default agent command and all profiles.
	// A missing default binary is a hard error — fail before entering the
	// dispatch loop so the user sees it immediately rather than at dispatch time.
	if !demoMode {
		validatedBackends := make(map[string]struct{})
		if err := validateBackend(configuredBackend(cfg.Agent.Command, cfg.Agent.Backend), "", validatedBackends, cfg); err != nil {
			return fmt.Errorf("agent startup: %w", err)
		}
		for name, profile := range cfg.Agent.Profiles {
			if err := validateBackend(configuredBackend(profile.Command, profile.Backend), name, validatedBackends, cfg); err != nil {
				slog.Warn("agent startup: profile validation failed", "profile", name, "error", err)
			}
		}
	}
	wm := workspace.NewManager(cfg)

	// Remove workspaces for issues that were terminal when we last shut down.
	orchestrator.StartupTerminalCleanup(ctx, tr, cfg.Tracker.TerminalStates, func(id string) error {
		return wm.RemoveWorkspace(ctx, id, "")
	})

	refreshChan := make(chan struct{}, 1)
	logBuf := logbuffer.New()
	// Persist per-issue logs to disk alongside the main log file so they
	// survive restarts and remain viewable after an issue completes.
	if logFile != "" {
		logBuf.SetLogDir(filepath.Join(filepath.Dir(logFile), "issues"))
	}
	orch := orchestrator.New(cfg, tr, runner, wm)
	if os.Getenv("ITERVOX_DRY_RUN") == "1" {
		orch.DryRun = true
		slog.Info("itervox: dry-run mode enabled — agents will not be dispatched")
	}
	orch.SetLogBuffer(logBuf)
	if logFile != "" {
		logDir := filepath.Dir(logFile)
		orch.SetHistoryFile(filepath.Join(logDir, "history.json"))
		orch.SetPausedFile(filepath.Join(logDir, "paused.json"))
		orch.SetInputRequiredFile(filepath.Join(logDir, "input_required.json"))
		orch.SetAgentLogDir(filepath.Join(logDir, "sessions"))
	}
	if cfg.Tracker.Kind != "" && cfg.Tracker.ProjectSlug != "" {
		orch.SetHistoryKey(cfg.Tracker.Kind + ":" + cfg.Tracker.ProjectSlug)
	}

	appSessionID := newAppSessionID()
	orch.SetAppSessionID(appSessionID)

	snap := buildSnapFunc(orch, tr, cfg, appSessionID, logBuf)

	// HTTP server — bind listener early so we know the actual port before
	// starting the TUI (the TUI needs the correct dashboard URL for 'w' key).
	var srvDone <-chan error
	var srvListener net.Listener
	var actualAddr string
	if cfg.Server.Port != nil {
		var err error
		srvListener, actualAddr, err = listenWithFallback(cfg.Server.Host, *cfg.Server.Port, 10)
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
		slog.Info("HTTP server listening", "addr", actualAddr)
		if host := cfg.Server.Host; host != "127.0.0.1" && host != "localhost" && host != "::1" && host != "" {
			slog.Warn("server: binding to non-loopback address — API has no authentication",
				"host", host, "hint", "set ITERVOX_API_TOKEN to enable bearer token auth")
		}
	}

	// Redirect slog to file-only before the TUI takes the alt-screen.
	// Without this, concurrent slog writes to stderr corrupt the bubbletea display.
	// The TUI log pane reads directly from logBuf instead of stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(fileWriter, &slog.HandlerOptions{Level: logLevel})))
	tuiCfg, tuiCancel := buildTUIConfig(orch, tr, cfg, workflowPath)
	if actualAddr != "" {
		tuiCfg.DashboardURL = fmt.Sprintf("http://%s/", actualAddr)
	}
	go statusui.Run(ctx, snap, logBuf, tuiCfg, tuiCancel)

	// Start serving on the already-bound listener.
	if srvListener != nil {
		fetchIssue := func(ctx context.Context, identifier string) (*server.TrackerIssue, error) {
			issue, err := tr.FetchIssueByIdentifier(ctx, identifier)
			if err != nil {
				return nil, err
			}
			if issue == nil {
				return nil, nil
			}
			ti := app.EnrichIssue(*issue, orch.Snapshot(), time.Now(), cfg)
			return &ti, nil
		}

		var pm server.ProjectManager
		if tpm, ok := tr.(tracker.ProjectManager); ok {
			pm = &linearProjectManager{pm: tpm, workflowPath: workflowPath}
		}

		adapter := &orchestratorAdapter{
			orch:         orch,
			logBuf:       logBuf,
			cfg:          cfg,
			tr:           tr,
			workflowPath: workflowPath,
		}
		srv := server.New(server.Config{
			Snapshot:       snap,
			RefreshChan:    refreshChan,
			LogFile:        logFile,
			Client:         adapter,
			FetchIssue:     fetchIssue,
			ProjectManager: pm,
			APIToken:       os.Getenv("ITERVOX_API_TOKEN"),
		})
		adapter.notify = srv.Notify
		if err := srv.Validate(); err != nil {
			return fmt.Errorf("server configuration error: %w", err)
		}
		orch.OnStateChange = srv.Notify
		srvDone = serveOnListener(ctx, srvListener, actualAddr, srv)
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
// buildSnapFunc returns the StateSnapshot function wired to the live orchestrator,
// tracker, and config. Extracted from run() to keep that function scannable.
func buildSnapFunc(orch *orchestrator.Orchestrator, tr tracker.Tracker, cfg *config.Config, appSessionID string, logBuf *logbuffer.Buffer) func() server.StateSnapshot {
	return func() server.StateSnapshot {
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
			// Count subagent markers in the log buffer for this issue.
			var subCount int
			if logBuf != nil {
				for _, line := range logBuf.Get(r.Issue.Identifier) {
					if strings.Contains(line, `"claude: subagent"`) || strings.Contains(line, `"codex: subagent"`) {
						subCount++
					}
				}
			}
			running = append(running, server.RunningRow{
				Identifier:    r.Issue.Identifier,
				State:         r.Issue.State,
				TurnCount:     r.TurnCount,
				Tokens:        r.TotalTokens,
				InputTokens:   r.InputTokens,
				OutputTokens:  r.OutputTokens,
				LastEvent:     msg,
				LastEventAt:   lastEvAt,
				SessionID:     r.SessionID,
				WorkerHost:    r.WorkerHost,
				Backend:       r.Backend,
				Kind:          r.Kind,
				ElapsedMs:     now.Sub(r.StartedAt).Milliseconds(),
				StartedAt:     r.StartedAt,
				SubagentCount: subCount,
			})
		}
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
		if rl, ok := tr.(tracker.RateLimiter); ok {
			if snap := rl.RateLimitSnapshot(); snap != nil {
				rateLimits = &server.RateLimitInfo{
					RequestsLimit:       snap.RequestsLimit,
					RequestsRemaining:   snap.RequestsRemaining,
					RequestsReset:       snap.Reset,
					ComplexityLimit:     snap.ComplexityLimit,
					ComplexityRemaining: snap.ComplexityRemaining,
				}
			}
		}
		if tpm, ok := tr.(tracker.ProjectManager); ok {
			activeProjectFilter = tpm.GetProjectFilter()
		}
		// When no runtime filter is set but WORKFLOW.md has project_slug,
		// surface it so the TUI picker shows it as checked.
		if activeProjectFilter == nil && cfg.Tracker.ProjectSlug != "" {
			activeProjectFilter = []string{cfg.Tracker.ProjectSlug}
		}
		profiles := orch.ProfilesCfg()
		agentMode := orch.AgentModeCfg()
		autoClearWorkspace := orch.AutoClearWorkspaceCfg()
		activeStates, terminalStates, completionState := orch.TrackerStatesCfg()

		var availableProfiles []string
		for name := range profiles {
			availableProfiles = append(availableProfiles, name)
		}
		sort.Strings(availableProfiles)

		var profileDefs map[string]server.ProfileDef
		if len(profiles) > 0 {
			profileDefs = make(map[string]server.ProfileDef, len(profiles))
			for n, p := range profiles {
				profileDefs[n] = server.ProfileDef{Command: p.Command, Prompt: p.Prompt, Backend: p.Backend}
			}
		}

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
				Kind:         r.Kind,
				SessionID:    r.SessionID,
				AppSessionID: r.AppSessionID,
			})
		}

		sshHostAddrs, sshHostDescs := orch.SSHHostsCfg()
		sshHostInfos := make([]server.SSHHostInfo, 0, len(sshHostAddrs))
		for _, h := range sshHostAddrs {
			sshHostInfos = append(sshHostInfos, server.SSHHostInfo{
				Host:        h,
				Description: sshHostDescs[h],
			})
		}

		pausedWithPR := orch.GetPausedOpenPRs()
		snap := server.StateSnapshot{
			GeneratedAt:         now,
			Counts:              server.Counts{Running: len(running), Retrying: len(retrying), Paused: len(paused)},
			Running:             running,
			History:             history,
			Retrying:            retrying,
			Paused:              paused,
			PausedWithPR:        pausedWithPR,
			MaxConcurrentAgents: orch.MaxWorkers(),
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
			AutoClearWorkspace:  autoClearWorkspace,
			CurrentAppSessionID: appSessionID,
			SSHHosts:            sshHostInfos,
			DispatchStrategy:    orch.DispatchStrategyCfg(),
			DefaultBackend:      configuredBackend(cfg.Agent.Command, cfg.Agent.Backend),
			InlineInput:         orch.InlineInputCfg(),
			AvailableModels:     convertModelsForSnapshot(cfg.Agent.AvailableModels),
			ReviewerProfile:     func() string { p, _ := orch.ReviewerCfg(); return p }(),
			AutoReview:          func() bool { _, a := orch.ReviewerCfg(); return a }(),
		}
		// Build input-required rows from the snapshot.
		for _, entry := range s.InputRequiredIssues {
			snap.InputRequired = append(snap.InputRequired, server.InputRequiredRow{
				Identifier: entry.Identifier,
				SessionID:  entry.SessionID,
				Context:    entry.Context,
				Backend:    entry.Backend,
				Profile:    entry.ProfileName,
				QueuedAt:   entry.QueuedAt.Format(time.RFC3339),
			})
		}
		return snap
	}
}

// buildTUIConfig wires the terminal status-UI config and returns the cancel
// function (used as the 'x' key handler in statusui.Run). Extracted from run().
func buildTUIConfig(
	orch *orchestrator.Orchestrator,
	tr tracker.Tracker,
	cfg *config.Config,
	workflowPath string,
) (statusui.Config, func(string) bool) {
	tuiCfg := statusui.Config{
		MaxAgents:     cfg.Agent.MaxConcurrentAgents,
		TodoStates:    cfg.Tracker.ActiveStates,
		BacklogStates: cfg.Tracker.BacklogStates,
	}
	if cfg.Server.Port != nil {
		tuiCfg.DashboardURL = fmt.Sprintf("http://%s:%d/", cfg.Server.Host, *cfg.Server.Port)
	}
	if tpm, ok := tr.(tracker.ProjectManager); ok {
		tuiCfg.FetchProjects = func() ([]statusui.ProjectItem, error) {
			fetchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			projects, err := tpm.FetchProjects(fetchCtx)
			if err != nil {
				return nil, err
			}
			items := make([]statusui.ProjectItem, len(projects))
			for i, p := range projects {
				items[i] = statusui.ProjectItem{ID: p.ID, Name: p.Name, Slug: p.Slug}
			}
			return items, nil
		}
		tuiCfg.SetProjectFilter = func(slugs []string) {
			tpm.SetProjectFilter(slugs)
			updateWorkflowProjectSlug(workflowPath, slugs)
		}
	}
	tuiCfg.AdjustWorkers = func(delta int) {
		next := orch.MaxWorkers() + delta
		orch.SetMaxWorkers(next)
		if err := workflow.PatchIntField(workflowPath, "max_concurrent_agents", orch.MaxWorkers()); err != nil {
			slog.Warn("failed to persist max_concurrent_agents to WORKFLOW.md", "error", err)
		}
	}
	{
		backlogAndActive := append(append([]string{}, cfg.Tracker.BacklogStates...), cfg.Tracker.ActiveStates...)
		tuiCfg.FetchBacklog = func() ([]statusui.BacklogIssueItem, error) {
			fetchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			issues, err := tr.FetchIssuesByStates(fetchCtx, backlogAndActive)
			if err != nil {
				return nil, err
			}
			items := make([]statusui.BacklogIssueItem, len(issues))
			for i, iss := range issues {
				pri := 0
				if iss.Priority != nil {
					pri = *iss.Priority
				}
				var desc string
				if iss.Description != nil {
					desc = *iss.Description
				}
				var comments []statusui.CommentItem
				for _, c := range iss.Comments {
					comments = append(comments, statusui.CommentItem{Author: c.AuthorName, Body: c.Body})
				}
				items[i] = statusui.BacklogIssueItem{
					Identifier:  iss.Identifier,
					Title:       iss.Title,
					State:       iss.State,
					Priority:    pri,
					Description: desc,
					Comments:    comments,
				}
			}
			return items, nil
		}
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
	tuiCfg.ResumeIssue = func(identifier string) bool {
		ok := orch.ResumeIssue(identifier)
		if ok {
			orch.Refresh()
		}
		return ok
	}
	tuiCfg.TerminateIssue = func(identifier string) bool {
		ok := orch.TerminateIssue(identifier)
		if ok {
			orch.Refresh()
		}
		return ok
	}
	tuiCfg.SetIssueProfile = func(identifier, profile string) {
		orch.SetIssueProfile(identifier, profile)
	}
	tuiCfg.IssueProfiles = func() map[string]string {
		s := orch.Snapshot()
		return s.IssueProfiles
	}
	tuiCfg.TriggerPoll = orch.Refresh
	tuiCfg.FetchIssueDetail = func(identifier string) (*statusui.BacklogIssueItem, error) {
		fetchCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		issue, err := tr.FetchIssueByIdentifier(fetchCtx, identifier)
		if err != nil {
			return nil, err
		}
		pri := 0
		if issue.Priority != nil {
			pri = *issue.Priority
		}
		var desc string
		if issue.Description != nil {
			desc = *issue.Description
		}
		var comments []statusui.CommentItem
		for _, c := range issue.Comments {
			comments = append(comments, statusui.CommentItem{Author: c.AuthorName, Body: c.Body})
		}
		return &statusui.BacklogIssueItem{
			Identifier:  issue.Identifier,
			Title:       issue.Title,
			State:       issue.State,
			Priority:    pri,
			Description: desc,
			Comments:    comments,
		}, nil
	}

	tuiCancel := func(identifier string) bool {
		issue := orch.GetRunningIssue(identifier)
		if issue == nil {
			return false
		}
		if !orch.CancelIssue(identifier) {
			return false
		}
		return true
	}
	return tuiCfg, tuiCancel
}

// profilesToEntries converts config.AgentProfile map to workflow.ProfileEntry map
// for persistence to WORKFLOW.md.
func profilesToEntries(profiles map[string]config.AgentProfile) map[string]workflow.ProfileEntry {
	entries := make(map[string]workflow.ProfileEntry, len(profiles))
	for name, p := range profiles {
		entries[name] = workflow.ProfileEntry{
			Command: p.Command,
			Prompt:  p.Prompt,
			Backend: p.Backend,
		}
	}
	return entries
}

// orchestratorAdapter implements server.OrchestratorClient using the live
// orchestrator, log buffer, tracker, and WORKFLOW.md persistence helpers.
// notify must be set after server construction (adapter.notify = srv.Notify).
type orchestratorAdapter struct {
	orch         *orchestrator.Orchestrator
	logBuf       *logbuffer.Buffer
	cfg          *config.Config
	tr           tracker.Tracker
	workflowPath string
	notify       func()
}

func (a *orchestratorAdapter) FetchIssues(ctx context.Context) ([]server.TrackerIssue, error) {
	allStates := deduplicateStates(a.cfg.Tracker.BacklogStates, a.cfg.Tracker.ActiveStates, a.cfg.Tracker.TerminalStates, a.cfg.Tracker.CompletionState)
	issues, err := a.tr.FetchIssuesByStates(ctx, allStates)
	if err != nil {
		return nil, err
	}
	snap := a.orch.Snapshot()
	now := time.Now()
	result := make([]server.TrackerIssue, len(issues))
	for i, issue := range issues {
		result[i] = app.EnrichIssue(issue, snap, now, a.cfg)
	}
	return result, nil
}

func (a *orchestratorAdapter) CancelIssue(identifier string) bool {
	return a.orch.CancelIssue(identifier)
}

func (a *orchestratorAdapter) ResumeIssue(identifier string) bool {
	ok := a.orch.ResumeIssue(identifier)
	if ok {
		a.orch.Refresh()
	}
	return ok
}

func (a *orchestratorAdapter) TerminateIssue(identifier string) bool {
	ok := a.orch.TerminateIssue(identifier)
	if ok {
		a.orch.Refresh()
	}
	return ok
}

func (a *orchestratorAdapter) ReanalyzeIssue(identifier string) bool {
	return a.orch.ReanalyzeIssue(identifier)
}

func (a *orchestratorAdapter) FetchLogs(identifier string) []string {
	return a.logBuf.Get(identifier)
}

func (a *orchestratorAdapter) FetchLogIdentifiers() []string {
	return a.logBuf.Identifiers()
}

func (a *orchestratorAdapter) ClearLogs(identifier string) error {
	return a.logBuf.Clear(identifier)
}

func (a *orchestratorAdapter) ClearAllLogs() error {
	return a.logBuf.ClearAll()
}

func (a *orchestratorAdapter) ClearIssueSubLogs(identifier string) error {
	logDir := a.orch.AgentLogDir()
	if logDir == "" {
		return nil
	}
	issueDir := filepath.Join(logDir, workspace.SanitizeKey(identifier))
	if err := workspace.AssertContained(logDir, issueDir); err != nil {
		return fmt.Errorf("clear sublogs: %w", err)
	}
	entries, err := os.ReadDir(issueDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		_ = os.Remove(filepath.Join(issueDir, e.Name()))
	}
	return nil
}

func (a *orchestratorAdapter) ClearSessionSublog(identifier, sessionID string) error {
	logDir := a.orch.AgentLogDir()
	if logDir == "" {
		return nil
	}
	// Sanitize both path components to prevent directory traversal.
	safeID := workspace.SanitizeKey(identifier)
	safeSess := workspace.SanitizeKey(sessionID)
	p := filepath.Join(logDir, safeID, safeSess+".jsonl")
	if err := workspace.AssertContained(logDir, p); err != nil {
		return fmt.Errorf("clear session sublog: %w", err)
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// FetchSubLogs returns parsed Claude Code session logs from CLAUDE_CODE_LOG_DIR.
// The fetcher is selected based on where the issue was last run:
//   - SSH host → SSHSublogFetcher (tar-over-SSH, session IDs from filenames)
//   - local    → LocalSublogFetcher (direct disk read)
//   - Docker   → DockerSublogFetcher (planned)
func (a *orchestratorAdapter) FetchSubLogs(identifier string) ([]domain.IssueLogEntry, error) {
	logDir := a.orch.AgentLogDir()
	if logDir == "" {
		return nil, nil
	}
	issueLogDir := filepath.Join(logDir, workspace.SanitizeKey(identifier))
	return a.sublogFetcher(identifier).FetchSubLogs(context.Background(), issueLogDir)
}

// sublogFetcher resolves the correct SublogFetcher for identifier by inspecting
// run history and live running sessions. Returns LocalSublogFetcher when no
// remote host is found.
func (a *orchestratorAdapter) sublogFetcher(identifier string) agent.SublogFetcher {
	// Check currently-running sessions first (most recent wins).
	// Running is keyed by issue ID, not identifier — iterate values.
	snap := a.orch.Snapshot()
	for _, entry := range snap.Running {
		if entry.Issue.Identifier == identifier && entry.WorkerHost != "" {
			return agent.SSHSublogFetcher{Host: entry.WorkerHost}
		}
	}
	// Fall back to run history.
	for _, run := range a.orch.RunHistory() {
		if run.Identifier == identifier && run.WorkerHost != "" {
			return agent.SSHSublogFetcher{Host: run.WorkerHost}
		}
	}
	return agent.LocalSublogFetcher{}
}

func (a *orchestratorAdapter) DispatchReviewer(identifier string) error {
	return a.orch.DispatchReviewer(identifier)
}

func (a *orchestratorAdapter) UpdateIssueState(ctx context.Context, identifier, stateName string) error {
	active, terminal, completion := a.orch.TrackerStatesCfg()
	allStates := deduplicateStates(a.cfg.Tracker.BacklogStates, active, terminal, completion)
	issues, err := a.tr.FetchIssuesByStates(ctx, allStates)
	if err != nil {
		return fmt.Errorf("fetch issues: %w", err)
	}
	for _, iss := range issues {
		if iss.Identifier == identifier {
			return a.tr.UpdateIssueState(ctx, iss.ID, stateName)
		}
	}
	return fmt.Errorf("issue %s not found", identifier)
}

// deduplicateStates concatenates backlog, active, terminal states and the
// completion state (if non-empty), removing duplicates while preserving order.
func deduplicateStates(backlog, active, terminal []string, completion string) []string {
	base := append(append(append([]string{}, backlog...), active...), terminal...)
	if completion != "" {
		base = append(base, completion)
	}
	seen := make(map[string]bool, len(base))
	out := make([]string, 0, len(base))
	for _, s := range base {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func (a *orchestratorAdapter) SetWorkers(n int) {
	a.orch.SetMaxWorkers(n)
	if err := workflow.PatchIntField(a.workflowPath, "max_concurrent_agents", n); err != nil {
		slog.Warn("failed to persist max_concurrent_agents to WORKFLOW.md", "error", err)
	}
}

func (a *orchestratorAdapter) BumpWorkers(delta int) int {
	next := a.orch.BumpMaxWorkers(delta)
	if err := workflow.PatchIntField(a.workflowPath, "max_concurrent_agents", next); err != nil {
		slog.Warn("failed to persist max_concurrent_agents to WORKFLOW.md", "error", err)
	}
	return next
}

func (a *orchestratorAdapter) SetIssueProfile(identifier, profile string) {
	a.orch.SetIssueProfile(identifier, profile)
}

func (a *orchestratorAdapter) SetIssueBackend(identifier, backend string) {
	a.orch.SetIssueBackend(identifier, backend)
}

func (a *orchestratorAdapter) ProfileDefs() map[string]server.ProfileDef {
	profiles := a.orch.ProfilesCfg()
	defs := make(map[string]server.ProfileDef, len(profiles))
	for name, p := range profiles {
		defs[name] = server.ProfileDef{Command: p.Command, Prompt: p.Prompt, Backend: p.Backend}
	}
	return defs
}

func (a *orchestratorAdapter) ReviewerConfig() (string, bool) {
	return a.orch.ReviewerCfg()
}

func (a *orchestratorAdapter) SetReviewerConfig(profile string, autoReview bool) error {
	a.orch.SetReviewerCfg(profile, autoReview)
	return nil
}

func (a *orchestratorAdapter) AvailableModels() map[string][]server.ModelOption {
	models := a.orch.AvailableModelsCfg()
	result := make(map[string][]server.ModelOption, len(models))
	for backend, opts := range models {
		converted := make([]server.ModelOption, len(opts))
		for i, m := range opts {
			converted[i] = server.ModelOption{ID: m.ID, Label: m.Label}
		}
		result[backend] = converted
	}
	return result
}

func (a *orchestratorAdapter) UpsertProfile(name string, def server.ProfileDef) error {
	profiles := a.orch.ProfilesCfg()
	if profiles == nil {
		profiles = make(map[string]config.AgentProfile)
	}
	// Resolve the command binary (e.g. alias → absolute path) so dispatch works
	// in non-interactive shell contexts.
	cmd := def.Command
	if cmd != "" {
		parts := strings.SplitN(cmd, " ", 2)
		resolved := resolveAgentCommand(parts[0])
		if resolved != parts[0] {
			if len(parts) > 1 {
				cmd = resolved + " " + parts[1]
			} else {
				cmd = resolved
			}
		}
	}
	profiles[name] = config.AgentProfile{Command: cmd, Prompt: def.Prompt, Backend: def.Backend}
	a.orch.SetProfilesCfg(profiles)
	if err := workflow.PatchProfilesBlock(a.workflowPath, profilesToEntries(profiles)); err != nil {
		return err
	}
	a.notify()
	return nil
}

func (a *orchestratorAdapter) DeleteProfile(name string) error {
	profiles := a.orch.ProfilesCfg()
	delete(profiles, name)
	a.orch.SetProfilesCfg(profiles)
	if err := workflow.PatchProfilesBlock(a.workflowPath, profilesToEntries(profiles)); err != nil {
		return err
	}
	a.notify()
	return nil
}

func (a *orchestratorAdapter) SetAgentMode(mode string) error {
	a.orch.SetAgentModeCfg(mode)
	if err := workflow.PatchAgentStringField(a.workflowPath, "agent_mode", mode); err != nil {
		slog.Warn("failed to persist agent_mode to WORKFLOW.md", "error", err)
	}
	a.notify()
	return nil
}

func (a *orchestratorAdapter) ClearAllWorkspaces() error {
	// Clear run history (in-memory + disk) so Timeline resets.
	a.orch.ClearHistory()

	root := a.cfg.Workspace.Root
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("clear workspaces: read dir %s: %w", root, err)
	}
	var firstErr error
	for _, e := range entries {
		path := filepath.Join(root, e.Name())
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (a *orchestratorAdapter) SetAutoClearWorkspace(enabled bool) error {
	a.orch.SetAutoClearWorkspaceCfg(enabled)
	if err := workflow.PatchWorkspaceBoolField(a.workflowPath, "auto_clear", enabled); err != nil {
		slog.Warn("failed to persist auto_clear to WORKFLOW.md", "error", err)
	}
	a.notify()
	return nil
}

func (a *orchestratorAdapter) UpdateTrackerStates(active, terminal []string, completion string) error {
	a.orch.SetTrackerStatesCfg(active, terminal, completion)
	if err := workflow.PatchStringSliceField(a.workflowPath, "active_states", active); err != nil {
		slog.Warn("could not patch active_states in WORKFLOW.md", "error", err)
	}
	if err := workflow.PatchStringSliceField(a.workflowPath, "terminal_states", terminal); err != nil {
		slog.Warn("could not patch terminal_states in WORKFLOW.md", "error", err)
	}
	if err := workflow.PatchStringField(a.workflowPath, "completion_state", completion); err != nil {
		slog.Warn("could not patch completion_state in WORKFLOW.md", "error", err)
	}
	a.notify()
	return nil
}

func (a *orchestratorAdapter) AddSSHHost(host, description string) error {
	a.orch.AddSSHHostCfg(host, description)
	return nil
}

func (a *orchestratorAdapter) RemoveSSHHost(host string) error {
	a.orch.RemoveSSHHostCfg(host)
	return nil
}

func (a *orchestratorAdapter) SetDispatchStrategy(strategy string) error {
	a.orch.SetDispatchStrategyCfg(strategy)
	return nil
}

func (a *orchestratorAdapter) ProvideInput(identifier, message string) bool {
	return a.orch.ProvideInput(identifier, message)
}

func (a *orchestratorAdapter) DismissInput(identifier string) bool {
	return a.orch.DismissInput(identifier)
}

func (a *orchestratorAdapter) SetInlineInput(enabled bool) error {
	a.orch.SetInlineInputCfg(enabled)
	a.notify()
	return nil
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

// linearProjectManager adapts tracker.ProjectManager to server.ProjectManager,
// converting domain.Project → server.Project and persisting filter changes to WORKFLOW.md.
type linearProjectManager struct {
	pm           tracker.ProjectManager
	workflowPath string
}

// FetchProjects implements server.ProjectManager.
func (m *linearProjectManager) FetchProjects(ctx context.Context) ([]server.Project, error) {
	projects, err := m.pm.FetchProjects(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]server.Project, len(projects))
	for i, p := range projects {
		result[i] = server.Project{ID: p.ID, Name: p.Name, Slug: p.Slug}
	}
	return result, nil
}

// SetProjectFilter implements server.ProjectManager and persists the filter to WORKFLOW.md.
func (m *linearProjectManager) SetProjectFilter(slugs []string) {
	m.pm.SetProjectFilter(slugs)
	if m.workflowPath != "" {
		updateWorkflowProjectSlug(m.workflowPath, slugs)
	}
}

// GetProjectFilter implements server.ProjectManager.
func (m *linearProjectManager) GetProjectFilter() []string { return m.pm.GetProjectFilter() }

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
	case "memory":
		issues := tracker.GenerateDemoIssues(10)
		return tracker.NewMemoryTracker(issues, cfg.Tracker.ActiveStates, cfg.Tracker.TerminalStates), nil
	default:
		return nil, fmt.Errorf("unknown tracker kind %q (supported: linear, github, memory)", cfg.Tracker.Kind)
	}
}

// runClear removes workspace directories for one or more issues, or all workspaces
// under workspace.root when no identifiers are given.
//
// Usage:
//
//	itervox clear [--workflow WORKFLOW.md] [identifier ...]
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
		fmt.Fprintf(os.Stderr, "itervox clear: load config %s: %v\n", *workflowPath, err)
		os.Exit(1)
	}

	root := cfg.Workspace.Root

	if len(identifiers) == 0 {
		// Remove all entries under workspace.root.
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("itervox clear: workspace root %s does not exist — nothing to clear\n", root)
				return
			}
			fmt.Fprintf(os.Stderr, "itervox clear: read dir %s: %v\n", root, err)
			os.Exit(1)
		}
		removed := 0
		for _, e := range entries {
			path := filepath.Join(root, e.Name())
			if err := os.RemoveAll(path); err != nil {
				fmt.Fprintf(os.Stderr, "itervox clear: remove %s: %v\n", path, err)
			} else {
				fmt.Printf("  removed %s\n", path)
				removed++
			}
		}
		fmt.Printf("itervox clear: removed %d workspace(s) from %s\n", removed, root)
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
		if err := wm.RemoveWorkspace(context.Background(), id, ""); err != nil {
			fmt.Fprintf(os.Stderr, "itervox clear: remove %s: %v\n", path, err)
		} else {
			fmt.Printf("  removed %s\n", path)
		}
	}
}

// repoInfo holds values discovered by scanning the current directory.
type repoInfo struct {
	RemoteURL     string // raw git remote URL
	Owner         string // e.g. "vnovick"
	Repo          string // e.g. "itervox"
	CloneURL      string // SSH clone URL reconstructed for after_create hook
	DefaultBranch string // "main" or "master"
	ProjectName   string // repo name, used for workspace.root
	HasClaudeMD   bool   // CLAUDE.md present in dir
	HasAgentsMD   bool   // AGENTS.md present in dir
	Stacks        []detectedStack
	ClaudeModels  []agent.ModelOption // discovered Claude models (may be empty)
	CodexModels   []agent.ModelOption // discovered Codex models (may be empty)
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

	// ── AGENTS.md ─────────────────────────────────────────────────────────────
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
		info.HasAgentsMD = true
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
func generateWorkflow(trackerKind, runner string, info repoInfo) string {
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
		b.WriteString("  # failed_state: \"Backlog\"       # State for issues that exhaust all retries.\n")
	} else {
		b.WriteString("  project_slug: " + slug + "\n")
		b.WriteString("  # GitHub uses labels to map states. Labels must exist in your repo.\n")
		b.WriteString("  # NOTE: GitHub Projects v2 'Status' field is separate from labels — Itervox\n")
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
		b.WriteString("  # failed_state: \"backlog\"        # Label for issues that exhaust all retries.\n")
	}

	b.WriteString("\npolling:\n  interval_ms: 60000\n")

	b.WriteString("\nagent:\n")
	if runner == "codex" {
		b.WriteString("  command: codex\n")
		b.WriteString("  backend: codex\n")
	} else {
		b.WriteString("  command: claude\n")
	}
	b.WriteString("  max_turns: 60\n")
	b.WriteString("  max_concurrent_agents: 3\n")
	b.WriteString("  max_retries: 5\n")
	b.WriteString("  turn_timeout_ms: 3600000\n")
	b.WriteString("  read_timeout_ms: 120000\n")
	b.WriteString("  stall_timeout_ms: 300000\n")

	// Reviewer prompt — used when a reviewer worker is dispatched (via auto_review or AI Review button).
	// Uses the reviewer_prompt template instead of the main WORKFLOW.md body.
	b.WriteString("  # reviewer_profile: reviewer       # Uncomment and create a 'reviewer' profile to enable AI code review.\n")
	b.WriteString("  # auto_review: false               # Set to true to auto-review after each successful agent run.\n")
	b.WriteString("  reviewer_prompt: |\n")
	b.WriteString("    You are an AI code reviewer for issue {{ issue.identifier }}: {{ issue.title }}.\n")
	b.WriteString("\n")
	b.WriteString("    ## Your task\n")
	b.WriteString("\n")
	b.WriteString("    Review the pull request created for this issue.\n")
	b.WriteString("\n")
	b.WriteString("    1. Run `gh pr diff` to read the PR changes\n")
	b.WriteString("    2. Review for: correctness, test coverage, edge cases, security issues, code style\n")
	b.WriteString("    3. If you find problems:\n")
	b.WriteString("       - Fix them directly in the workspace\n")
	b.WriteString("       - Commit and push: `git add -A && git commit -m \"fix: reviewer corrections\" && git push`\n")
	b.WriteString("       - Post a comment on the tracker issue summarising what you fixed\n")
	b.WriteString("    4. If the PR is clean:\n")
	b.WriteString("       - Post an approval comment: \"AI review passed — no issues found\"\n")
	b.WriteString("\n")
	b.WriteString("    Be concise. Focus on real bugs, not style preferences.\n")

	// Write discovered models so the dashboard profile editor has suggestions.
	if len(info.ClaudeModels) > 0 || len(info.CodexModels) > 0 {
		b.WriteString("  available_models:\n")
		if len(info.ClaudeModels) > 0 {
			b.WriteString("    claude:\n")
			for _, m := range info.ClaudeModels {
				b.WriteString(fmt.Sprintf("      - { id: %q, label: %q }\n", m.ID, m.Label))
			}
		}
		if len(info.CodexModels) > 0 {
			b.WriteString("    codex:\n")
			for _, m := range info.CodexModels {
				b.WriteString(fmt.Sprintf("      - { id: %q, label: %q }\n", m.ID, m.Label))
			}
		}
	}

	cloneURL := info.CloneURL
	if cloneURL == "" {
		cloneURL = "git@github.com:owner/" + info.ProjectName + ".git"
	}
	b.WriteString("\nworkspace:\n")
	b.WriteString("  root: ~/.itervox/workspaces/" + info.ProjectName + "\n")
	b.WriteString("  worktree: true\n")
	b.WriteString("  clone_url: " + cloneURL + "\n")
	b.WriteString("  base_branch: " + info.DefaultBranch + "\n")

	b.WriteString("\nhooks:\n")
	b.WriteString("  # after_create and before_run are no longer needed for clone/reset —\n")
	b.WriteString("  # Itervox maintains a bare clone and creates worktrees automatically.\n")
	b.WriteString("  # Add custom hooks here if your project needs extra setup:\n")
	b.WriteString("  # after_create: |\n")
	b.WriteString("  #   npm install\n")

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

	// AGENTS.md — multi-agent configuration
	if info.HasAgentsMD {
		b.WriteString("## Multi-Agent Configuration\n\n")
		b.WriteString("This project has an `AGENTS.md`. Read it for multi-agent conventions and coordination rules:\n\n")
		b.WriteString("```bash\ncat AGENTS.md\n```\n\n")
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
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// runInit scans the current (or specified) directory for repo metadata and
// generates a WORKFLOW.md pre-filled with discovered values.
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	trackerKind := fs.String("tracker", "", "tracker kind: linear or github (required)")
	runner := fs.String("runner", "claude", "default runner backend: claude or codex")
	output := fs.String("output", "WORKFLOW.md", "output file path")
	dir := fs.String("dir", ".", "directory to scan for repo metadata")
	force := fs.Bool("force", false, "overwrite output file if it already exists")
	_ = fs.Parse(args)

	switch *trackerKind {
	case "linear", "github":
		// valid
	case "":
		fmt.Fprintln(os.Stderr, "itervox init: --tracker is required (linear or github)")
		fs.Usage()
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "itervox init: unknown tracker %q (supported: linear, github)\n", *trackerKind)
		os.Exit(1)
	}

	switch *runner {
	case "claude", "codex":
		// valid
	default:
		fmt.Fprintf(os.Stderr, "itervox init: unknown runner %q (supported: claude, codex)\n", *runner)
		os.Exit(1)
	}

	// Validate that the selected runner CLI is available on PATH.
	switch *runner {
	case "claude":
		if err := agent.ValidateClaudeCLI(); err != nil {
			fmt.Fprintf(os.Stderr, "itervox init: %v\n", err)
			os.Exit(1)
		}
	case "codex":
		if err := agent.ValidateCodexCLI(); err != nil {
			fmt.Fprintf(os.Stderr, "itervox init: %v\n", err)
			os.Exit(1)
		}
	}

	if _, err := os.Stat(*output); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "itervox init: %s already exists (use --force to overwrite)\n", *output)
		os.Exit(1)
	}

	fmt.Printf("itervox init: scanning %s...\n", *dir)
	info := scanRepo(*dir)

	if info.RemoteURL != "" {
		fmt.Printf("  git remote : %s\n", info.RemoteURL)
	}
	fmt.Printf("  branch     : %s\n", info.DefaultBranch)
	fmt.Printf("  runner     : %s\n", *runner)
	if info.HasClaudeMD {
		fmt.Printf("  CLAUDE.md  : found — prompt will reference it\n")
	} else {
		fmt.Printf("  CLAUDE.md  : not found — add one for best results\n")
	}
	if info.HasAgentsMD {
		fmt.Printf("  AGENTS.md  : found — prompt will reference it\n")
	}
	for _, s := range info.Stacks {
		fmt.Printf("  stack      : %s (%s)\n", s.Name, strings.Join(s.Commands, ", "))
	}

	// Discover available models from provider APIs (best-effort).
	fmt.Printf("itervox init: discovering available models...\n")
	info.ClaudeModels = agent.ListClaudeModels()
	info.CodexModels = agent.ListCodexModels()
	fmt.Printf("  models     : %d claude, %d codex\n", len(info.ClaudeModels), len(info.CodexModels))

	content := generateWorkflow(*trackerKind, *runner, info)

	if err := os.WriteFile(*output, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "itervox init: write %s: %v\n", *output, err)
		os.Exit(1)
	}
	fmt.Printf("itervox init: wrote %s\n", *output)

	// Create .itervox/.env if it doesn't exist.
	envDir := ".itervox"
	envPath := filepath.Join(envDir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		_ = os.MkdirAll(envDir, 0o755)
		var envContent string
		switch *trackerKind {
		case "linear":
			envContent = "# Itervox environment — this file is gitignored.\n# See WORKFLOW.md for which variables are referenced.\nLINEAR_API_KEY=lin_api_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n"
		case "github":
			envContent = "# Itervox environment — this file is gitignored.\n# See WORKFLOW.md for which variables are referenced.\nGITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n"
		}
		if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "itervox init: write %s: %v\n", envPath, err)
		} else {
			fmt.Printf("itervox init: wrote %s\n", envPath)
		}
	}

	// Ensure .itervox/.env is gitignored.
	gitignorePath := filepath.Join(envDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		_ = os.WriteFile(gitignorePath, []byte(".env\n"), 0o644)
	}

	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Edit %s — fill in your API key\n", envPath)
	runCmd := "itervox"
	if *output != "WORKFLOW.md" {
		runCmd = "itervox -workflow " + *output
	}
	if *trackerKind == "linear" {
		fmt.Printf("  2. Run: %s\n", runCmd)
		fmt.Printf("  3. Select a project via the TUI (press p) or the web dashboard\n")
	} else {
		fmt.Printf("  2. Run: %s\n", runCmd)
	}
}

// listenWithFallback tries to listen on the given host:port. If the port is
// already in use, it tries up to maxPortRetries successive ports. Returns the
// listener and the actual address it bound to.
func listenWithFallback(host string, port, maxPortRetries int) (net.Listener, string, error) {
	for i := 0; i <= maxPortRetries; i++ {
		tryPort := port + i
		addr := fmt.Sprintf("%s:%d", host, tryPort)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			if i > 0 {
				slog.Warn("server: configured port in use, using next available",
					"configured_port", port, "actual_port", tryPort)
			}
			return ln, addr, nil
		}
		if !isAddrInUse(err) {
			return nil, "", fmt.Errorf("http listen %s: %w", addr, err)
		}
	}
	return nil, "", fmt.Errorf("ports %d–%d all in use — is another itervox instance running?",
		port, port+maxPortRetries)
}

// serveOnListener starts an HTTP server on an already-bound listener and
// returns a channel that receives its exit error.
func serveOnListener(ctx context.Context, ln net.Listener, addr string, handler http.Handler) <-chan error {
	errCh := make(chan error, 1)

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

// isAddrInUse reports whether err indicates the address is already in use.
func isAddrInUse(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			return sysErr.Err == syscall.EADDRINUSE
		}
	}
	return false
}
