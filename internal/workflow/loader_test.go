package workflow_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/workflow"
)

func TestLoadBasicWorkflow(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "workflows", "basic.md")
	wf, err := workflow.Load(path)
	require.NoError(t, err)
	assert.NotNil(t, wf.Config)
	assert.Contains(t, wf.PromptTemplate, "issue.identifier")
	trackerKind, ok := wf.Config["tracker"].(map[string]interface{})
	require.True(t, ok, "tracker should be a map")
	assert.Equal(t, "linear", trackerKind["kind"])
}

func TestLoadNoFrontMatter(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "workflows", "no-front-matter.md")
	wf, err := workflow.Load(path)
	require.NoError(t, err)
	assert.Empty(t, wf.Config)
	assert.Contains(t, wf.PromptTemplate, "no front matter")
}

func TestLoadMissingFile(t *testing.T) {
	_, err := workflow.Load("/nonexistent/path/WORKFLOW.md")
	require.Error(t, err)
	var wfErr *workflow.Error
	require.ErrorAs(t, err, &wfErr)
	assert.Equal(t, workflow.ErrMissingFile, wfErr.Code)
}

func TestLoadInvalidYAML(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "workflows", "invalid-yaml.md")
	_, err := workflow.Load(path)
	require.Error(t, err)
	var wfErr *workflow.Error
	require.ErrorAs(t, err, &wfErr)
	assert.Equal(t, workflow.ErrParseError, wfErr.Code)
}

func TestLoadFrontMatterNotAMap(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	content := "---\n- item1\n- item2\n---\n\nPrompt body.\n"
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	_, err := workflow.Load(f)
	require.Error(t, err)
	var wfErr *workflow.Error
	require.ErrorAs(t, err, &wfErr)
	assert.Equal(t, workflow.ErrFrontMatterNotAMap, wfErr.Code)
}

func TestLoadEmptyFrontMatter(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	content := "---\n---\n\nSome prompt.\n"
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	wf, err := workflow.Load(f)
	require.NoError(t, err)
	assert.Empty(t, wf.Config)
	assert.Equal(t, "Some prompt.", wf.PromptTemplate)
}

func TestLoadPromptIsTrimmed(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	content := "---\ntracker:\n  kind: linear\n---\n\n\n  hello  \n\n"
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	wf, err := workflow.Load(f)
	require.NoError(t, err)
	assert.Equal(t, "hello", wf.PromptTemplate)
}

func TestPatchIntField(t *testing.T) {
	content := "---\nagent:\n  max_concurrent_agents: 3\n  max_turns: 60\n---\n\nPrompt body.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	require.NoError(t, workflow.PatchIntField(f, "max_concurrent_agents", 7))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	assert.Contains(t, got, "max_concurrent_agents: 7")
	assert.Contains(t, got, "max_turns: 60") // unchanged
	assert.Contains(t, got, "Prompt body.")  // body preserved
}

func TestPatchIntFieldKeyNotFound(t *testing.T) {
	content := "---\nagent:\n  max_turns: 60\n---\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	err := workflow.PatchIntField(f, "max_concurrent_agents", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPatchIntFieldPreservesComments(t *testing.T) {
	content := "---\nagent:\n  max_concurrent_agents: 3 # set at runtime\n---\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	require.NoError(t, workflow.PatchIntField(f, "max_concurrent_agents", 10))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	assert.Contains(t, string(data), "max_concurrent_agents: 10 # set at runtime")
}

func TestPatchProfilesBlock_Create(t *testing.T) {
	// File with no profiles block — adds one under agent:
	content := "---\nagent:\n  max_concurrent_agents: 3\n  command: claude\n---\n\nPrompt body.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	profiles := map[string]workflow.ProfileEntry{
		"fast":     {Command: "claude --model claude-haiku-4-5-20251001"},
		"thorough": {Command: "claude --model claude-opus-4-6"},
		"codex":    {Command: "run-codex-wrapper", Backend: "codex"},
	}
	require.NoError(t, workflow.PatchProfilesBlock(f, profiles))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	assert.Contains(t, got, "  profiles:")
	assert.Contains(t, got, "    fast:")
	assert.Contains(t, got, "      command: claude --model claude-haiku-4-5-20251001")
	assert.Contains(t, got, "    codex:")
	assert.Contains(t, got, "      command: run-codex-wrapper")
	assert.Contains(t, got, "      backend: codex")
	assert.Contains(t, got, "    thorough:")
	assert.Contains(t, got, "      command: claude --model claude-opus-4-6")
	// Other fields preserved
	assert.Contains(t, got, "max_concurrent_agents: 3")
	assert.Contains(t, got, "command: claude")
	// Body preserved
	assert.Contains(t, got, "Prompt body.")
}

func TestPatchProfilesBlock_Replace(t *testing.T) {
	// File with existing profiles — replaces them.
	content := "---\nagent:\n  max_concurrent_agents: 5\n  profiles:\n    old:\n      command: claude --model old\n---\n\nBody.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	profiles := map[string]workflow.ProfileEntry{
		"fast": {Command: "run-codex-wrapper", Backend: "codex"},
	}
	require.NoError(t, workflow.PatchProfilesBlock(f, profiles))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	assert.Contains(t, got, "    fast:")
	assert.Contains(t, got, "      command: run-codex-wrapper")
	assert.Contains(t, got, "      backend: codex")
	// Old profile gone
	assert.NotContains(t, got, "old:")
	// Other fields preserved
	assert.Contains(t, got, "max_concurrent_agents: 5")
	assert.Contains(t, got, "Body.")
}

func TestPatchProfilesBlock_Delete(t *testing.T) {
	// Passing nil profiles removes the block.
	content := "---\nagent:\n  max_concurrent_agents: 2\n  profiles:\n    fast:\n      command: claude --model fast\n---\n\nBody.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	require.NoError(t, workflow.PatchProfilesBlock(f, nil))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	assert.NotContains(t, got, "profiles:")
	assert.NotContains(t, got, "fast:")
	// Other fields preserved
	assert.Contains(t, got, "max_concurrent_agents: 2")
	assert.Contains(t, got, "Body.")
}

func TestPatchStringSliceField_Replace(t *testing.T) {
	content := "---\ntracker:\n  active_states: [\"a\", \"b\"]\n  terminal_states: [\"Done\"]\n---\n\nBody.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	require.NoError(t, workflow.PatchStringSliceField(f, "active_states", []string{"x", "y", "z"}))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	assert.Contains(t, got, `active_states: ["x","y","z"]`)
	assert.Contains(t, got, `terminal_states: ["Done"]`) // unchanged
	assert.Contains(t, got, "Body.")                     // body preserved
}

func TestPatchStringSliceField_KeyNotFound(t *testing.T) {
	content := "---\ntracker:\n  active_states: [\"Todo\"]\n---\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	err := workflow.PatchStringSliceField(f, "nonexistent_key", []string{"a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPatchStringField_Replace(t *testing.T) {
	content := "---\ntracker:\n  completion_state: \"In Review\"\n---\n\nBody.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	require.NoError(t, workflow.PatchStringField(f, "completion_state", "Done"))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	assert.Contains(t, string(data), `completion_state: "Done"`)
	assert.Contains(t, string(data), "Body.")
}

func TestPatchStringField_KeyNotFound(t *testing.T) {
	content := "---\ntracker:\n  kind: linear\n---\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	err := workflow.PatchStringField(f, "nonexistent_key", "value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPatchProfilesBlock_PreservesOtherKeys(t *testing.T) {
	// Other agent keys and comments are unchanged.
	content := "---\n# Top comment\ntracker:\n  kind: linear\nagent:\n  # agent comment\n  max_concurrent_agents: 3\n  max_turns: 60\n  profiles:\n    old:\n      command: claude\nserver:\n  port: 8090\n---\n\nPrompt.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	profiles := map[string]workflow.ProfileEntry{
		"fast": {Command: "run-codex-wrapper", Backend: "codex"},
	}
	require.NoError(t, workflow.PatchProfilesBlock(f, profiles))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	// New profile present
	assert.Contains(t, got, "    fast:")
	assert.Contains(t, got, "      command: run-codex-wrapper")
	assert.Contains(t, got, "      backend: codex")
	// Old profile gone
	assert.NotContains(t, got, "    old:")
	// Other top-level keys preserved
	assert.Contains(t, got, "tracker:")
	assert.Contains(t, got, "  kind: linear")
	assert.Contains(t, got, "server:")
	assert.Contains(t, got, "  port: 8090")
	// Comments preserved
	assert.Contains(t, got, "# Top comment")
	assert.Contains(t, got, "# agent comment")
	// Body preserved
	assert.Contains(t, got, "Prompt.")
}

// --- workflow.Error ---

func TestWorkflowErrorMessage(t *testing.T) {
	err := &workflow.Error{Code: workflow.ErrMissingFile, Path: "/some/path.md"}
	assert.Equal(t, "missing_workflow_file: /some/path.md", err.Error())
}

func TestWorkflowErrorMessageWithCause(t *testing.T) {
	inner := fmt.Errorf("inner cause")
	err := &workflow.Error{Code: workflow.ErrParseError, Path: "/w.md", Cause: inner}
	msg := err.Error()
	assert.Contains(t, msg, "workflow_parse_error")
	assert.Contains(t, msg, "/w.md")
	assert.Contains(t, msg, "inner cause")
}

func TestWorkflowErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("root")
	err := &workflow.Error{Code: workflow.ErrParseError, Path: "p", Cause: inner}
	assert.Equal(t, inner, err.Unwrap())
}

func TestWorkflowErrorUnwrapNoCause(t *testing.T) {
	err := &workflow.Error{Code: workflow.ErrMissingFile, Path: "p"}
	assert.Nil(t, err.Unwrap())
}

// --- PatchAgentBoolField ---

func writeTmp(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))
	return f
}

func TestPatchAgentBoolFieldSetTrue(t *testing.T) {
	f := writeTmp(t, "---\nagent:\n  verbose: false\n---\n\nBody.\n")
	require.NoError(t, workflow.PatchAgentBoolField(f, "verbose", true))

	data, _ := os.ReadFile(f)
	assert.Contains(t, string(data), "  verbose: true")
	assert.Contains(t, string(data), "Body.")
}

func TestPatchAgentBoolFieldSetFalseRemovesKey(t *testing.T) {
	f := writeTmp(t, "---\nagent:\n  verbose: true\n---\n\nBody.\n")
	require.NoError(t, workflow.PatchAgentBoolField(f, "verbose", false))

	data, _ := os.ReadFile(f)
	assert.NotContains(t, string(data), "verbose")
}

func TestPatchAgentBoolFieldInsertWhenMissing(t *testing.T) {
	f := writeTmp(t, "---\nagent:\n  max_turns: 50\n---\n\nBody.\n")
	require.NoError(t, workflow.PatchAgentBoolField(f, "auto_resume", true))

	data, _ := os.ReadFile(f)
	assert.Contains(t, string(data), "  auto_resume: true")
}

func TestPatchAgentBoolFieldNoFrontMatterErrors(t *testing.T) {
	f := writeTmp(t, "No front matter here.\n")
	err := workflow.PatchAgentBoolField(f, "verbose", true)
	require.Error(t, err)
}

func TestPatchAgentBoolFieldMissingFileErrors(t *testing.T) {
	err := workflow.PatchAgentBoolField("/no/such/file.md", "verbose", true)
	require.Error(t, err)
}

// --- PatchAgentStringField ---

func TestPatchAgentStringFieldSet(t *testing.T) {
	f := writeTmp(t, "---\nagent:\n  backend: claude\n---\n\nBody.\n")
	require.NoError(t, workflow.PatchAgentStringField(f, "backend", "codex"))

	data, _ := os.ReadFile(f)
	// PatchAgentStringField stores strings quoted.
	assert.Contains(t, string(data), "backend")
	assert.Contains(t, string(data), "codex")
}

func TestPatchAgentStringFieldRemoveWhenEmpty(t *testing.T) {
	f := writeTmp(t, "---\nagent:\n  backend: codex\n---\n\nBody.\n")
	require.NoError(t, workflow.PatchAgentStringField(f, "backend", ""))

	data, _ := os.ReadFile(f)
	assert.NotContains(t, string(data), "backend")
}

func TestPatchAgentStringFieldInsertWhenMissing(t *testing.T) {
	f := writeTmp(t, "---\nagent:\n  max_turns: 40\n---\n\nBody.\n")
	require.NoError(t, workflow.PatchAgentStringField(f, "backend", "codex"))

	data, _ := os.ReadFile(f)
	assert.Contains(t, string(data), "backend")
	assert.Contains(t, string(data), "codex")
}

func TestPatchAgentStringFieldNoFrontMatterErrors(t *testing.T) {
	f := writeTmp(t, "Just body.\n")
	err := workflow.PatchAgentStringField(f, "backend", "codex")
	require.Error(t, err)
}

func TestPatchAgentStringFieldMissingFileErrors(t *testing.T) {
	err := workflow.PatchAgentStringField("/no/such/file.md", "backend", "codex")
	require.Error(t, err)
}
