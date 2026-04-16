package agent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/agent"
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

func TestIsContentInputRequired(t *testing.T) {
	// IsContentInputRequired is the literal sentinel detector. Semantic fallback
	// for plain-English blocking questions is exercised at the orchestrator layer.
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"empty string", "", false},
		{"plain output no sentinel", "I've fixed the bug and pushed the changes.", false},
		{"questions without sentinel do not match sentinel detector", "How would you like to proceed?", false},
		{"question mark alone", "Is the test passing?", false},
		{"sentinel alone", "<!-- itervox:needs-input -->", true},
		{"sentinel with question", "All done.\n<!-- itervox:needs-input -->\nWhich path do you prefer?", true},
		{"sentinel at end of long text", string(make([]byte, 5000)) + "\n<!-- itervox:needs-input -->", true},
		{"sentinel at start of text", "<!-- itervox:needs-input -->\nShould I continue?", true},
		{"sentinel is case sensitive", "<!-- ITERVOX:NEEDS-INPUT -->", false},
		{"sentinel with trailing whitespace", "<!-- itervox:needs-input -->   \n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, agent.IsContentInputRequired(tc.text), tc.name)
		})
	}
}

func TestIsSentinelInputRequired(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"empty", "", false},
		{"no sentinel", "just some output with a question?", false},
		{"sentinel alone", agent.InputRequiredSentinel, true},
		{"sentinel mid stream", "before\n" + agent.InputRequiredSentinel + "\nafter", true},
		{"sentinel with trailing whitespace", agent.InputRequiredSentinel + "   \n", true},
		{"heuristic-only phrase does not match sentinel detector", "Questions for you:", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, agent.IsSentinelInputRequired(tc.text), tc.name)
		})
	}
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
