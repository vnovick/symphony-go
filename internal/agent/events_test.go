package agent_test

import (
	"testing"

	"github.com/vnovick/symphony-go/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLineSystemEvent(t *testing.T) {
	line := []byte(`{"type":"system","session_id":"sess-abc"}`)
	ev, err := agent.ParseLine(line)
	require.NoError(t, err)
	assert.Equal(t, "system", ev.Type)
	assert.Equal(t, "sess-abc", ev.SessionID)
}

func TestParseLineAssistantEvent(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]},"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`)
	ev, err := agent.ParseLine(line)
	require.NoError(t, err)
	assert.Equal(t, "assistant", ev.Type)
	assert.Equal(t, 10, ev.Usage.InputTokens)
	assert.Equal(t, 5, ev.Usage.OutputTokens)
}

func TestParseLineResultSuccess(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","session_id":"sess-abc"}`)
	ev, err := agent.ParseLine(line)
	require.NoError(t, err)
	assert.Equal(t, "result", ev.Type)
	assert.Equal(t, "sess-abc", ev.SessionID)
	assert.False(t, ev.IsError)
	assert.False(t, ev.IsInputRequired)
}

func TestParseLineResultError(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"error","session_id":"sess-abc"}`)
	ev, err := agent.ParseLine(line)
	require.NoError(t, err)
	assert.True(t, ev.IsError)
	assert.False(t, ev.IsInputRequired)
}

func TestParseLineResultInputRequired(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"error","is_error":true,"session_id":"sess-abc","result":"Human turn required"}`)
	ev, err := agent.ParseLine(line)
	require.NoError(t, err)
	assert.True(t, ev.IsError)
}

func TestParseLineNonJSONReturnsError(t *testing.T) {
	line := []byte(`not json at all`)
	_, err := agent.ParseLine(line)
	assert.Error(t, err)
}

func TestParseLineEmptyLineReturnsError(t *testing.T) {
	_, err := agent.ParseLine([]byte(``))
	assert.Error(t, err)
}
