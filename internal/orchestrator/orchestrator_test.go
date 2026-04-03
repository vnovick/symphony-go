package orchestrator_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/agent"
	"github.com/vnovick/symphony-go/internal/agent/agenttest"
	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/domain"
	"github.com/vnovick/symphony-go/internal/orchestrator"
	"github.com/vnovick/symphony-go/internal/tracker"
)

func singleIssueTracker(t *testing.T, state string) *tracker.MemoryTracker {
	t.Helper()
	cfg := baseConfig()
	return tracker.NewMemoryTracker(
		[]domain.Issue{makeIssue("id1", "ENG-1", state, nil, nil)},
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
	)
}

func TestOrchestratorDispatchesOnTick(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 50
	mt := singleIssueTracker(t, "In Progress")
	fake := agenttest.NewFakeRunner([]agent.StreamEvent{
		{Type: "system", SessionID: "s1"},
		{Type: "result", SessionID: "s1"},
	})

	orch := orchestrator.New(cfg, mt, fake, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dispatched := make(chan string, 1)
	orch.OnDispatch = func(issueID string) {
		select {
		case dispatched <- issueID:
		default: // ignore subsequent dispatches
		}
	}

	go orch.Run(ctx) //nolint:errcheck

	select {
	case id := <-dispatched:
		assert.Equal(t, "id1", id)
	case <-time.After(2 * time.Second):
		t.Fatal("expected dispatch within 2s")
	}
}

func TestOrchestratorNoDuplicateDispatch(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	mt := singleIssueTracker(t, "In Progress")
	// Stall = true keeps the worker goroutine blocked, so the issue stays in Running state.
	fake := &agenttest.FakeRunner{Stall: true}

	orch := orchestrator.New(cfg, mt, fake, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var count int
	orch.OnDispatch = func(_ string) { count++ }
	_ = orch.Run(ctx)

	require.Equal(t, 1, count, "issue must not be dispatched twice while it is running")
}

// TestCancelResumeRace detects the savePausedToDisk map-reference data race.
// Run with: go test -race ./internal/orchestrator/...
func TestCancelResumeRace(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	mt := singleIssueTracker(t, "In Progress")
	fake := &agenttest.FakeRunner{Stall: true}

	dir := t.TempDir()
	orch := orchestrator.New(cfg, mt, fake, nil)
	orch.SetPausedFile(filepath.Join(dir, "paused.json"))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go orch.Run(ctx) //nolint:errcheck

	// Let the orchestrator start.
	time.Sleep(30 * time.Millisecond)

	// Concurrently call Resume + Cancel many times to trigger any map race.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			orch.CancelIssue("ENG-1")
		}()
		go func() {
			defer wg.Done()
			orch.ResumeIssue("ENG-1")
		}()
	}
	wg.Wait()
}

// TestReviewerRespectsCancellation verifies that a reviewer goroutine exits
// when the orchestrator context is cancelled.
func TestReviewerRespectsCancellation(t *testing.T) {
	cfg := baseConfig()
	mt := singleIssueTracker(t, "In Review")
	fake := &agenttest.FakeRunner{Stall: true}

	orch := orchestrator.New(cfg, mt, fake, nil)
	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		orch.Run(ctx) //nolint:errcheck
	}()

	time.Sleep(30 * time.Millisecond)
	_ = orch.DispatchReviewer("ENG-1") // may return "not found" — that's fine

	cancel()

	select {
	case <-runDone:
		// Good — orchestrator exited promptly.
	case <-time.After(2 * time.Second):
		t.Fatal("orchestrator did not exit within 2s after context cancellation")
	}
}

// TestReanalyzeIssueRace detects the ForceReanalyze map data race.
func TestReanalyzeIssueRace(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	mt := singleIssueTracker(t, "In Progress")
	fake := &agenttest.FakeRunner{Stall: true}

	dir := t.TempDir()
	orch := orchestrator.New(cfg, mt, fake, nil)
	orch.SetPausedFile(filepath.Join(dir, "paused.json"))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go orch.Run(ctx) //nolint:errcheck

	// First pause the issue so ReanalyzeIssue has something to operate on.
	time.Sleep(30 * time.Millisecond)
	orch.CancelIssue("ENG-1")
	time.Sleep(20 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orch.ReanalyzeIssue("ENG-1")
		}()
	}
	wg.Wait()
}

func TestCancelIssue_NotRunning(t *testing.T) {
	cfg := baseConfig()
	mt := singleIssueTracker(t, "In Progress")
	fake := &agenttest.FakeRunner{Stall: true}
	orch := orchestrator.New(cfg, mt, fake, nil)
	// Do not call Run — no workers are dispatched.

	ok := orch.CancelIssue("ENG-99")
	require.False(t, ok, "cancel should return false when no worker is running")

	// Marker should be cleaned up — second call also returns false.
	ok = orch.CancelIssue("ENG-99")
	require.False(t, ok)
}

func TestCancelIssue_Running(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	mt := singleIssueTracker(t, "In Progress")
	fake := &agenttest.FakeRunner{Stall: true}

	orch := orchestrator.New(cfg, mt, fake, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Wait for the worker to appear in lastSnap.Running (set by storeSnap,
	// which fires OnStateChange after OnDispatch). Using a channel avoids
	// the race between OnDispatch and the subsequent storeSnap call.
	workerVisible := make(chan struct{})
	orch.OnStateChange = func() {
		snap := orch.Snapshot()
		if len(snap.Running) > 0 {
			select {
			case workerVisible <- struct{}{}:
			default:
			}
		}
	}
	go orch.Run(ctx) //nolint:errcheck

	select {
	case <-workerVisible:
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not appear in snapshot within 1s")
	}

	ok := orch.CancelIssue("ENG-1")
	require.True(t, ok, "cancel should return true for a running worker")
}

func TestDispatchReviewer_NoProfileConfigured(t *testing.T) {
	cfg := baseConfig()
	mt := tracker.NewMemoryTracker(nil, cfg.Tracker.ActiveStates, cfg.Tracker.TerminalStates)
	fake := agenttest.NewFakeRunner(nil)
	orch := orchestrator.New(cfg, mt, fake, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go orch.Run(ctx) //nolint:errcheck
	time.Sleep(20 * time.Millisecond)

	err := orch.DispatchReviewer("NONEXISTENT-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reviewer_profile configured")
}

// trackingRunner wraps a Runner and signals done on the first RunTurn call.
type trackingRunner struct {
	agent.Runner
	once sync.Once
	done chan struct{}
}

func (r *trackingRunner) RunTurn(ctx context.Context, log agent.Logger, onProgress func(agent.TurnResult), sessionID *string, prompt, workspacePath, command, workerHost string, logDir string, readTimeoutMs, turnTimeoutMs int) (agent.TurnResult, error) {
	r.once.Do(func() { close(r.done) })
	return r.Runner.RunTurn(ctx, log, onProgress, sessionID, prompt, workspacePath, command, workerHost, logDir, readTimeoutMs, turnTimeoutMs)
}

type capturingRunner struct {
	agent.Runner
	mu      sync.Mutex
	once    sync.Once
	done    chan struct{}
	command string
	prompt  string
}

func (r *capturingRunner) RunTurn(ctx context.Context, log agent.Logger, onProgress func(agent.TurnResult), sessionID *string, prompt, workspacePath, command, workerHost string, logDir string, readTimeoutMs, turnTimeoutMs int) (agent.TurnResult, error) {
	r.mu.Lock()
	r.command = command
	r.prompt = prompt
	r.mu.Unlock()
	r.once.Do(func() { close(r.done) })
	return r.Runner.RunTurn(ctx, log, onProgress, sessionID, prompt, workspacePath, command, workerHost, logDir, readTimeoutMs, turnTimeoutMs)
}

func (r *capturingRunner) LastCommand() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.command
}

func (r *capturingRunner) LastPrompt() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.prompt
}

func TestDispatchReviewer_Success(t *testing.T) {
	cfg := baseConfig()
	cfg.Tracker.CompletionState = "In Review"
	cfg.Agent.ReviewerProfile = "reviewer"
	cfg.Agent.Profiles = map[string]config.AgentProfile{
		"reviewer": {Command: "claude", Prompt: "You are a code reviewer."},
	}
	issue := makeIssue("id1", "ENG-1", "In Review", nil, nil)
	mt := tracker.NewMemoryTracker(
		[]domain.Issue{issue},
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
	)
	done := make(chan struct{})
	wrapped := &trackingRunner{
		Runner: agenttest.NewFakeRunner([]agent.StreamEvent{
			{Type: "system", SessionID: "s1"},
			{Type: "result", SessionID: "s1"},
		}),
		done: done,
	}
	orch := orchestrator.New(cfg, mt, wrapped, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go orch.Run(ctx) //nolint:errcheck
	time.Sleep(20 * time.Millisecond)

	err := orch.DispatchReviewer("ENG-1")
	require.NoError(t, err)

	select {
	case <-done:
		// reviewer RunTurn completed through the regular worker queue
	case <-ctx.Done():
		t.Fatal("reviewer did not complete within 3s")
	}
}

func TestDispatchUsesProfileBackendOverride(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	cfg.Agent.Command = "claude"
	cfg.Agent.Profiles = map[string]config.AgentProfile{
		"codex-fast": {
			Command: "run-codex-wrapper",
			Backend: "codex",
		},
	}
	mt := singleIssueTracker(t, "In Progress")
	wrapped := &capturingRunner{
		Runner: &agenttest.FakeRunner{Stall: true},
		done:   make(chan struct{}),
	}
	orch := orchestrator.New(cfg, mt, wrapped, nil)
	orch.SetIssueProfile("ENG-1", "codex-fast")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	workerVisible := make(chan struct{}, 1)
	orch.OnStateChange = func() {
		if len(orch.Snapshot().Running) > 0 {
			select {
			case workerVisible <- struct{}{}:
			default:
			}
		}
	}

	go orch.Run(ctx) //nolint:errcheck

	select {
	case <-workerVisible:
	case <-ctx.Done():
		t.Fatal("worker did not appear in snapshot within 2s")
	}
	select {
	case <-wrapped.done:
	case <-ctx.Done():
		t.Fatal("runner was not invoked within 2s")
	}

	snap := orch.Snapshot()
	require.Len(t, snap.Running, 1)
	for _, entry := range snap.Running {
		assert.Equal(t, "codex", entry.Backend)
	}
	assert.Equal(t, agent.CommandWithBackendHint("run-codex-wrapper", "codex"), wrapped.LastCommand())
}

func TestDispatchUsesDefaultBackendOverride(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	cfg.Agent.Command = "run-codex-wrapper"
	cfg.Agent.Backend = "codex"
	mt := singleIssueTracker(t, "In Progress")
	wrapped := &capturingRunner{
		Runner: &agenttest.FakeRunner{Stall: true},
		done:   make(chan struct{}),
	}
	orch := orchestrator.New(cfg, mt, wrapped, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	workerVisible := make(chan struct{}, 1)
	orch.OnStateChange = func() {
		if len(orch.Snapshot().Running) > 0 {
			select {
			case workerVisible <- struct{}{}:
			default:
			}
		}
	}

	go orch.Run(ctx) //nolint:errcheck

	select {
	case <-workerVisible:
	case <-ctx.Done():
		t.Fatal("worker did not appear in snapshot within 2s")
	}
	select {
	case <-wrapped.done:
	case <-ctx.Done():
		t.Fatal("runner was not invoked within 2s")
	}

	snap := orch.Snapshot()
	require.Len(t, snap.Running, 1)
	for _, entry := range snap.Running {
		assert.Equal(t, "codex", entry.Backend)
	}
	assert.Equal(t, agent.CommandWithBackendHint("run-codex-wrapper", "codex"), wrapped.LastCommand())
}

func TestTeamsModeUsesResolvedProfileBackendForSubagentContext(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	cfg.Agent.Command = "claude"
	cfg.Agent.AgentMode = "teams"
	cfg.Agent.Profiles = map[string]config.AgentProfile{
		"codex-fast": {
			Command: "run-codex-wrapper",
			Backend: "codex",
		},
		"research": {
			Command: "claude --model claude-sonnet-4-6",
			Prompt:  "Deep research support.",
		},
	}
	mt := singleIssueTracker(t, "In Progress")
	wrapped := &capturingRunner{
		Runner: agenttest.NewFakeRunner([]agent.StreamEvent{
			{Type: "system", SessionID: "s1"},
			{Type: "result", SessionID: "s1"},
		}),
		done: make(chan struct{}),
	}
	orch := orchestrator.New(cfg, mt, wrapped, nil)
	orch.SetIssueProfile("ENG-1", "codex-fast")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go orch.Run(ctx) //nolint:errcheck

	select {
	case <-wrapped.done:
	case <-ctx.Done():
		t.Fatal("runner was not invoked within 2s")
	}

	assert.Contains(t, wrapped.LastPrompt(), "spawn_agent tool")
	assert.NotContains(t, wrapped.LastPrompt(), "Task tool")
}

// --- Getter/setter unit tests ---

func newOrch() *orchestrator.Orchestrator {
	cfg := baseConfig()
	mt := tracker.NewMemoryTracker(nil, cfg.Tracker.ActiveStates, cfg.Tracker.TerminalStates)
	return orchestrator.New(cfg, mt, nil, nil)
}

func TestMaxWorkersClamped(t *testing.T) {
	o := newOrch()
	o.SetMaxWorkers(0)
	assert.Equal(t, 1, o.MaxWorkers())

	o.SetMaxWorkers(100)
	assert.Equal(t, 50, o.MaxWorkers())

	o.SetMaxWorkers(5)
	assert.Equal(t, 5, o.MaxWorkers())
}

func TestAgentModeCfgRoundtrip(t *testing.T) {
	o := newOrch()
	o.SetAgentModeCfg("codex")
	assert.Equal(t, "codex", o.AgentModeCfg())
}

func TestProfilesCfgRoundtrip(t *testing.T) {
	o := newOrch()
	profiles := map[string]config.AgentProfile{
		"fast": {Command: "codex", Backend: "codex"},
	}
	o.SetProfilesCfg(profiles)
	got := o.ProfilesCfg()
	require.Contains(t, got, "fast")
	assert.Equal(t, "codex", got["fast"].Command)
}

func TestTrackerStatesCfgRoundtrip(t *testing.T) {
	o := newOrch()
	o.SetTrackerStatesCfg([]string{"Todo"}, []string{"Done"}, "Done")
	active, terminal, completion := o.TrackerStatesCfg()
	assert.Equal(t, []string{"Todo"}, active)
	assert.Equal(t, []string{"Done"}, terminal)
	assert.Equal(t, "Done", completion)
}

func TestGetRunningIssueNotFound(t *testing.T) {
	o := newOrch()
	result := o.GetRunningIssue("ENG-99")
	assert.Nil(t, result)
}

func TestRunHistoryEmpty(t *testing.T) {
	o := newOrch()
	assert.Empty(t, o.RunHistory())
}

func TestSetHistoryFile(t *testing.T) {
	o := newOrch()
	// Just verify no panic; the orchestrator isn't running.
	o.SetHistoryFile(filepath.Join(t.TempDir(), "history.json"))
}

func TestRefreshNonBlocking(t *testing.T) {
	o := newOrch()
	// Refresh is non-blocking; calling it multiple times without a reader must not deadlock.
	o.Refresh()
	o.Refresh()
}

func TestSetLogBuffer(t *testing.T) {
	o := newOrch()
	// Just verify it doesn't panic when a nil buffer is passed.
	o.SetLogBuffer(nil)
}

func TestTerminateIssue_NotRunning(t *testing.T) {
	o := newOrch()
	// Neither running nor paused — should return false.
	assert.False(t, o.TerminateIssue("ENG-99"))
}

func TestGetPausedOpenPRs_Empty(t *testing.T) {
	o := newOrch()
	result := o.GetPausedOpenPRs()
	assert.NotNil(t, result)
	assert.Empty(t, result)
}
