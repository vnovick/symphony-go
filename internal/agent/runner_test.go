package agent_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/agent"
	"github.com/vnovick/symphony-go/internal/agent/agenttest"
)

func TestRunTurnFirstTurnBuildsPromptFlag(t *testing.T) {
	dir := t.TempDir()
	fake := agenttest.NewFakeRunner([]agent.StreamEvent{
		{Type: "system", SessionID: "sess-1"},
		{Type: "result", SessionID: "sess-1"},
	})
	result, err := fake.RunTurn(context.Background(), slog.Default(), nil, nil, "do the thing", dir, "claude", "", 5000, 10000)
	require.NoError(t, err)
	assert.Equal(t, "sess-1", result.SessionID)
	assert.False(t, result.Failed)
}

func TestRunTurnContinuationUsesResume(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess-1"
	fake := agenttest.NewFakeRunner([]agent.StreamEvent{
		{Type: "system", SessionID: "sess-1"},
		{Type: "result", SessionID: "sess-1"},
	})
	result, err := fake.RunTurn(context.Background(), slog.Default(), nil, &sessionID, "continue", dir, "claude", "", 5000, 10000)
	require.NoError(t, err)
	assert.Equal(t, "sess-1", result.SessionID)
}

func TestRunTurnFailedOnErrorResult(t *testing.T) {
	dir := t.TempDir()
	fake := agenttest.NewFakeRunner([]agent.StreamEvent{
		{Type: "system", SessionID: "sess-1"},
		{Type: "result", SessionID: "sess-1", IsError: true},
	})
	result, err := fake.RunTurn(context.Background(), slog.Default(), nil, nil, "prompt", dir, "claude", "", 5000, 10000)
	require.NoError(t, err)
	assert.True(t, result.Failed)
}

func TestRunTurnInputRequiredSetsFlag(t *testing.T) {
	dir := t.TempDir()
	fake := agenttest.NewFakeRunner([]agent.StreamEvent{
		{Type: "system", SessionID: "sess-1"},
		{Type: "result", SessionID: "sess-1", IsError: true, IsInputRequired: true},
	})
	result, err := fake.RunTurn(context.Background(), slog.Default(), nil, nil, "prompt", dir, "claude", "", 5000, 10000)
	require.NoError(t, err)
	assert.True(t, result.Failed)
	assert.True(t, result.InputRequired)
}

func TestRunTurnTokensAccumulated(t *testing.T) {
	dir := t.TempDir()
	fake := agenttest.NewFakeRunner([]agent.StreamEvent{
		{Type: "system", SessionID: "sess-1"},
		{Type: "assistant", Usage: agent.UsageSnapshot{InputTokens: 100, OutputTokens: 50}},
		{Type: "result", SessionID: "sess-1"},
	})
	result, err := fake.RunTurn(context.Background(), slog.Default(), nil, nil, "prompt", dir, "claude", "", 5000, 10000)
	require.NoError(t, err)
	assert.Equal(t, 100, result.InputTokens)
	assert.Equal(t, 50, result.OutputTokens)
}

func TestPartialLineBuffering(t *testing.T) {
	r, w := io.Pipe()
	resultCh := make(chan agent.StreamEvent, 1)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			ev, err := agent.ParseLine(scanner.Bytes())
			if err == nil {
				resultCh <- ev
				return
			}
		}
	}()
	partial := `{"type":"result","subtype":"success","session_id":"s1"}`
	_, _ = fmt.Fprint(w, partial[:10])
	time.Sleep(10 * time.Millisecond)
	_, _ = fmt.Fprintln(w, partial[10:])
	ev := <-resultCh
	assert.Equal(t, agent.EventResult, ev.Type)
	_ = r.Close()
}
