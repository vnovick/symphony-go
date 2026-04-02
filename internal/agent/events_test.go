package agent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/agent"
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
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"empty string", "", false},
		{"plain output no questions", "I've fixed the bug and pushed the changes.", false},
		{"questions for you", "Here are the results.\n\nQuestions for you:\n1. Which approach?", true},
		{"how would you like to proceed", "Analysis complete. How would you like to proceed?", true},
		{"please answer", "Please answer whichever questions are relevant.", true},
		{"should i proceed", "Should I proceed with the implementation?", true},
		{"case insensitive", "QUESTIONS FOR YOU: pick one", true},
		{"what would you like", "What would you like me to do next?", true},
		{"which is higher priority", "Which is higher priority: fixing A or B?", true},
		{"awaiting your input", "I'm awaiting your input on the design.", true},
		{"no false positive on question mark alone", "Is the test passing? Yes it is.", false},
		{"tail scan only", string(make([]byte, 3000)) + "Questions for you:", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, agent.IsContentInputRequired(tc.text), tc.name)
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
