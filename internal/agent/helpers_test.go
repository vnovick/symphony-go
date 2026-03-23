package agent

// White-box tests for unexported helper functions in the agent package.
// These functions contain non-trivial logic that deserves direct coverage.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- shellQuote ---

func TestShellQuoteSimple(t *testing.T) {
	assert.Equal(t, "'hello world'", shellQuote("hello world"))
}

func TestShellQuoteWithSingleQuote(t *testing.T) {
	// Single quotes inside the string must be escaped.
	assert.Equal(t, "'it'\\''s fine'", shellQuote("it's fine"))
}

func TestShellQuoteEmpty(t *testing.T) {
	assert.Equal(t, "''", shellQuote(""))
}

func TestShellQuoteSpecialChars(t *testing.T) {
	// Backticks, $, ! etc. are safe inside single quotes.
	got := shellQuote("`echo $HOME`")
	assert.Equal(t, "'`echo $HOME`'", got)
}

// --- buildShellCmd ---

func TestBuildShellCmdNewSession(t *testing.T) {
	cmd := buildShellCmd("claude", nil, "do the thing")
	assert.Contains(t, cmd, "claude")
	assert.Contains(t, cmd, "--output-format stream-json")
	assert.Contains(t, cmd, "-p")
	assert.Contains(t, cmd, "do the thing")
	assert.NotContains(t, cmd, "--resume")
}

func TestBuildShellCmdResume(t *testing.T) {
	id := "sess-abc"
	cmd := buildShellCmd("claude", &id, "ignored prompt")
	assert.Contains(t, cmd, "--resume")
	assert.Contains(t, cmd, "sess-abc")
	// When resuming, the new-session flag ` -p ` should not appear (note spaces).
	assert.NotContains(t, cmd, " -p ")
}

func TestBuildShellCmdEmptySessionID(t *testing.T) {
	id := ""
	cmd := buildShellCmd("claude", &id, "use prompt")
	// Empty session ID should be treated as new session.
	assert.NotContains(t, cmd, "--resume")
	assert.Contains(t, cmd, "-p")
}

// --- todoItems ---

func TestTodoItemsBasic(t *testing.T) {
	raw := json.RawMessage(`{"todos":[{"content":"fix bug","status":"pending"},{"content":"add tests","status":"pending"}]}`)
	items := todoItems(raw)
	assert.Equal(t, []string{"fix bug", "add tests"}, items)
}

func TestTodoItemsSkipsEmptyContent(t *testing.T) {
	raw := json.RawMessage(`{"todos":[{"content":""},{"content":"real item"}]}`)
	items := todoItems(raw)
	assert.Equal(t, []string{"real item"}, items)
}

func TestTodoItemsNilInput(t *testing.T) {
	assert.Nil(t, todoItems(nil))
}

func TestTodoItemsEmptyJSON(t *testing.T) {
	assert.Nil(t, todoItems(json.RawMessage(`{}`)))
}

func TestTodoItemsInvalidJSON(t *testing.T) {
	assert.Nil(t, todoItems(json.RawMessage(`not json`)))
}

func TestTodoItemsNoTodosKey(t *testing.T) {
	assert.Nil(t, todoItems(json.RawMessage(`{"other":"field"}`)))
}

// --- buildCodexShellCmd ---

func TestBuildCodexShellCmdNewSession(t *testing.T) {
	cmd := buildCodexShellCmd("codex", nil, "do the work", "/workspace")
	assert.Contains(t, cmd, "codex")
	assert.Contains(t, cmd, "-C")
	assert.Contains(t, cmd, "/workspace")
	assert.Contains(t, cmd, " exec")
	assert.Contains(t, cmd, "--json")
	assert.Contains(t, cmd, "do the work")
	assert.NotContains(t, cmd, "resume")
}

func TestBuildCodexShellCmdResume(t *testing.T) {
	id := "sess-xyz"
	cmd := buildCodexShellCmd("codex", &id, "continue", "")
	assert.Contains(t, cmd, "resume")
	assert.Contains(t, cmd, "sess-xyz")
	assert.Contains(t, cmd, "continue")
}

func TestBuildCodexShellCmdNoWorkspace(t *testing.T) {
	cmd := buildCodexShellCmd("codex", nil, "prompt", "")
	assert.NotContains(t, cmd, "-C")
}

func TestBuildCodexShellCmdEmptySessionID(t *testing.T) {
	id := ""
	cmd := buildCodexShellCmd("codex", &id, "prompt text", "")
	assert.NotContains(t, cmd, "resume")
}
