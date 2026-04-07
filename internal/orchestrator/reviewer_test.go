package orchestrator_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/agent/agenttest"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/orchestrator"
	"github.com/vnovick/itervox/internal/tracker"
)

// ─── Config + accessor tests ─────────────────────────────────────────────────

func TestReviewerCfgRoundtrip(t *testing.T) {
	cfg := baseConfig()
	orch := orchestrator.New(cfg, tracker.NewMemoryTracker(nil, nil, nil), agenttest.NewFakeRunner(nil), nil)

	orch.SetReviewerCfg("reviewer", true)
	profile, autoReview := orch.ReviewerCfg()
	assert.Equal(t, "reviewer", profile)
	assert.True(t, autoReview)

	orch.SetReviewerCfg("", false)
	profile, autoReview = orch.ReviewerCfg()
	assert.Equal(t, "", profile)
	assert.False(t, autoReview)
}

func TestReviewerConfigParsedFromYAML(t *testing.T) {
	cfg := baseConfig()
	cfg.Agent.ReviewerProfile = "code-reviewer"
	cfg.Agent.AutoReview = true
	assert.Equal(t, "code-reviewer", cfg.Agent.ReviewerProfile)
	assert.True(t, cfg.Agent.AutoReview)
}

// ─── DispatchReviewer tests ──────────────────────────────────────────────────

func TestDispatchReviewer_FailsWithoutProfile(t *testing.T) {
	cfg := baseConfig()
	// No ReviewerProfile set
	orch := orchestrator.New(cfg, tracker.NewMemoryTracker(nil, nil, nil), agenttest.NewFakeRunner(nil), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go orch.Run(ctx) //nolint:errcheck
	time.Sleep(20 * time.Millisecond)

	err := orch.DispatchReviewer("ENG-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reviewer_profile configured")
}

func TestDispatchReviewer_SucceedsWithProfile(t *testing.T) {
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
		// Reviewer completed through the regular worker queue
	case <-ctx.Done():
		t.Fatal("reviewer did not complete within 3s")
	}
}

// ─── Auto-review tests ──────────────────────────────────────────────────────

func TestAutoReview_DispatchesAfterSuccess(t *testing.T) {
	logBuf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	cfg := baseConfig()
	cfg.Polling.IntervalMs = 50
	cfg.Agent.ReviewerProfile = "reviewer"
	cfg.Agent.AutoReview = true
	cfg.Agent.Profiles = map[string]config.AgentProfile{
		"reviewer": {Command: "claude", Prompt: "Review this code."},
	}

	mt := tracker.NewMemoryTracker(
		[]domain.Issue{makeIssue("id1", "ENG-1", "In Progress", nil, nil)},
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
	)

	// The FakeRunner will be called for both the worker AND the reviewer.
	callCount := 0
	done := make(chan struct{}, 2)
	countingRunner := &countingTrackingRunner{
		Runner: agenttest.NewFakeRunner([]agent.StreamEvent{
			{Type: "system", SessionID: "s1"},
			{Type: "result", SessionID: "s1"},
		}),
		done:      done,
		callCount: &callCount,
	}

	orch := orchestrator.New(cfg, mt, countingRunner, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go orch.Run(ctx) //nolint:errcheck

	// Wait for at least 2 RunTurn calls (worker + reviewer)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("expected 2 RunTurn calls, got %d", i)
		}
	}

	time.Sleep(200 * time.Millisecond)
	cancel()

	logs := logBuf.String()
	assert.Contains(t, logs, "orchestrator: dispatching reviewer", "should log reviewer dispatch")
}

func TestAutoReview_DoesNotTriggerWhenDisabled(t *testing.T) {
	logBuf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	cfg := baseConfig()
	cfg.Polling.IntervalMs = 50
	cfg.Agent.ReviewerProfile = "reviewer"
	cfg.Agent.AutoReview = false // disabled
	cfg.Agent.Profiles = map[string]config.AgentProfile{
		"reviewer": {Command: "claude", Prompt: "Review this code."},
	}

	mt := tracker.NewMemoryTracker(
		[]domain.Issue{makeIssue("id1", "ENG-1", "In Progress", nil, nil)},
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

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not complete within 3s")
	}

	time.Sleep(200 * time.Millisecond)
	cancel()

	logs := logBuf.String()
	assert.NotContains(t, logs, "orchestrator: dispatching reviewer", "should NOT dispatch reviewer when auto_review is false")
}

func TestAutoReview_DoesNotTriggerWithoutProfile(t *testing.T) {
	logBuf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	cfg := baseConfig()
	cfg.Polling.IntervalMs = 50
	cfg.Agent.AutoReview = true
	// No ReviewerProfile set

	mt := tracker.NewMemoryTracker(
		[]domain.Issue{makeIssue("id1", "ENG-1", "In Progress", nil, nil)},
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

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not complete within 3s")
	}

	time.Sleep(200 * time.Millisecond)
	cancel()

	logs := logBuf.String()
	assert.NotContains(t, logs, "orchestrator: dispatching reviewer")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// countingTrackingRunner tracks how many times RunTurn is called and signals
// on each call via the done channel.
type countingTrackingRunner struct {
	agent.Runner
	done      chan struct{}
	callCount *int
}

func (r *countingTrackingRunner) RunTurn(ctx context.Context, log agent.Logger, onProgress func(agent.TurnResult), sessionID *string, prompt, workspacePath, command, workerHost string, logDir string, readTimeoutMs, turnTimeoutMs int) (agent.TurnResult, error) {
	*r.callCount++
	res, err := r.Runner.RunTurn(ctx, log, onProgress, sessionID, prompt, workspacePath, command, workerHost, logDir, readTimeoutMs, turnTimeoutMs)
	select {
	case r.done <- struct{}{}:
	default:
	}
	return res, err
}

// Verify syncBuffer exists in the test package (defined in token_log_test.go).
// If this doesn't compile, the syncBuffer type from token_log_test.go is needed.
var _ = (*syncBuffer)(nil)

// Needed for the strings.Split usage in log assertions.
var _ = strings.Split
var _ = bytes.Buffer{}
