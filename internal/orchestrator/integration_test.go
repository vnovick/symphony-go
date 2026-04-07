package orchestrator_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/orchestrator"
	"github.com/vnovick/itervox/internal/tracker"
)

// succeedOnceRunner returns a successful result on every call (non-zero tokens,
// Failed=false). After the first call it returns zero tokens so the worker
// treats the session as concluded and exits cleanly.
type succeedOnceRunner struct {
	calls atomic.Int32
}

func (r *succeedOnceRunner) RunTurn(_ context.Context, _ agent.Logger, _ func(agent.TurnResult), _ *string, _, _, _, _, _ string, _, _ int) (agent.TurnResult, error) {
	n := r.calls.Add(1)
	if n == 1 {
		return agent.TurnResult{
			SessionID:    "test-session-1",
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			ResultText:   "task completed",
		}, nil
	}
	// Subsequent turns: zero tokens signals "session concluded".
	return agent.TurnResult{SessionID: "test-session-1"}, nil
}

// TestOrchestratorLifecycle exercises the full dispatch cycle:
// create orchestrator → add issue in active state → run a few ticks →
// verify dispatch, runner invocation, worker exit, and state transitions.
func TestOrchestratorLifecycle(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	cfg.Agent.MaxTurns = 3

	mt := tracker.NewMemoryTracker(
		[]domain.Issue{makeIssue("id1", "ENG-1", "In Progress", nil, nil)},
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
	)

	runner := &succeedOnceRunner{}
	dispatched := make(chan string, 1)

	orch := orchestrator.New(cfg, mt, runner, nil)
	orch.OnDispatch = func(issueID string) {
		select {
		case dispatched <- issueID:
		default:
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go orch.Run(ctx) //nolint:errcheck

	// 1. Verify the issue gets dispatched.
	select {
	case id := <-dispatched:
		assert.Equal(t, "id1", id, "dispatched issue ID should match")
	case <-time.After(3 * time.Second):
		t.Fatal("issue was not dispatched within timeout")
	}

	// 2. Wait for the runner to be called at least once.
	deadline := time.After(3 * time.Second)
	for runner.calls.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("runner was not called within timeout")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// 3. Wait for the worker to exit — Running map should become empty.
	deadline = time.After(3 * time.Second)
	for {
		snap := orch.Snapshot()
		if len(snap.Running) == 0 {
			// Worker exited. Verify the issue is no longer claimed
			// (successful completion releases the claim).
			_, claimed := snap.Claimed["id1"]
			assert.False(t, claimed, "issue should not be claimed after successful completion")
			cancel()
			return
		}
		select {
		case <-deadline:
			t.Fatalf("worker did not exit within timeout; Running=%d", len(orch.Snapshot().Running))
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TestOrchestratorLifecycleWithCompletionState verifies that when a
// completion_state is configured, the tracker issue is transitioned after a
// successful worker run.
func TestOrchestratorLifecycleWithCompletionState(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	cfg.Agent.MaxTurns = 3
	cfg.Tracker.CompletionState = "Done"

	mt := tracker.NewMemoryTracker(
		[]domain.Issue{makeIssue("id1", "ENG-1", "In Progress", nil, nil)},
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
	)

	runner := &succeedOnceRunner{}
	orch := orchestrator.New(cfg, mt, runner, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go orch.Run(ctx) //nolint:errcheck

	// Wait for the issue to be transitioned to "Done" by the worker's
	// post-run hook.
	deadline := time.After(4 * time.Second)
	for {
		issues, _ := mt.FetchIssueStatesByIDs(ctx, []string{"id1"})
		if len(issues) > 0 && issues[0].State == "Done" {
			cancel()
			return
		}
		select {
		case <-deadline:
			snap := orch.Snapshot()
			t.Fatalf("issue was not transitioned to Done within timeout; Running=%d, Claimed=%d",
				len(snap.Running), len(snap.Claimed))
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TestOrchestratorLifecycleFailAndRetry verifies the fail → retry → fail →
// pause cycle using alwaysFailRunner with MaxRetries=1.
func TestOrchestratorLifecycleFailAndRetry(t *testing.T) {
	cfg := baseConfig()
	cfg.Polling.IntervalMs = 20
	cfg.Agent.MaxRetries = 1
	cfg.Agent.MaxRetryBackoffMs = 10

	mt := tracker.NewMemoryTracker(
		[]domain.Issue{makeIssue("id1", "ENG-1", "In Progress", nil, nil)},
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
	)

	runner := &alwaysFailRunner{}
	orch := orchestrator.New(cfg, mt, runner, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go orch.Run(ctx) //nolint:errcheck

	// MaxRetries=1: initial attempt fails → retry → second attempt fails → paused.
	deadline := time.After(4 * time.Second)
	for {
		snap := orch.Snapshot()
		if _, paused := snap.PausedIdentifiers["ENG-1"]; paused {
			require.Greater(t, runner.callCount, 1,
				"runner should have been called more than once (initial + retry)")
			cancel()
			return
		}
		select {
		case <-deadline:
			t.Fatal("issue was not paused after max retries within timeout")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
