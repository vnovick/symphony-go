package agenttest

import (
	"context"
	"time"

	"github.com/vnovick/itervox/internal/agent"
)

// DemoRunner simulates a realistic agent session with delays between events.
// Each RunTurn call takes approximately the configured duration before returning
// a successful result with synthetic token counts.
type DemoRunner struct {
	TurnDuration time.Duration // how long each turn takes (default: 3s)
}

// NewDemoRunner creates a runner that simulates agent turns with the given duration.
func NewDemoRunner(turnDuration time.Duration) *DemoRunner {
	return &DemoRunner{TurnDuration: turnDuration}
}

func (d *DemoRunner) RunTurn(
	ctx context.Context,
	log agent.Logger,
	onProgress func(agent.TurnResult),
	sessionID *string,
	prompt, workspacePath, command, workerHost, logDir string,
	readTimeoutMs, turnTimeoutMs int,
) (agent.TurnResult, error) {
	dur := d.TurnDuration
	if dur == 0 {
		dur = 3 * time.Second
	}

	sid := "demo-session"
	if sessionID != nil && *sessionID != "" {
		sid = *sessionID
	}

	// Simulate session start
	log.Info("claude: session started", "session_id", sid)
	if onProgress != nil {
		onProgress(agent.TurnResult{SessionID: sid})
	}

	// Wait for the configured duration (or until cancelled)
	select {
	case <-time.After(dur / 2):
	case <-ctx.Done():
		return agent.TurnResult{Failed: true, SessionID: sid}, ctx.Err()
	}

	// Simulate mid-turn progress
	log.Info("claude: text", "session_id", sid, "text", "Analyzing the issue and implementing changes...")
	mid := agent.TurnResult{
		SessionID:    sid,
		InputTokens:  1500,
		OutputTokens: 800,
		TotalTokens:  2300,
	}
	if onProgress != nil {
		onProgress(mid)
	}

	// Wait for remaining duration
	select {
	case <-time.After(dur / 2):
	case <-ctx.Done():
		return agent.TurnResult{Failed: true, SessionID: sid}, ctx.Err()
	}

	// Return completed result
	log.Info("claude: turn done", "session_id", sid, "input_tokens", 3000, "output_tokens", 1500)
	return agent.TurnResult{
		SessionID:    sid,
		InputTokens:  3000,
		OutputTokens: 1500,
		TotalTokens:  4500,
		ResultText:   "Changes implemented successfully. Created branch and opened PR.",
	}, nil
}
