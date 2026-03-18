package workflow_test

import (
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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	wf, err := workflow.Load(f)
	require.NoError(t, err)
	assert.Empty(t, wf.Config)
	assert.Equal(t, "Some prompt.", wf.PromptTemplate)
}

func TestLoadPromptIsTrimmed(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	content := "---\ntracker:\n  kind: linear\n---\n\n\n  hello  \n\n"
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	wf, err := workflow.Load(f)
	require.NoError(t, err)
	assert.Equal(t, "hello", wf.PromptTemplate)
}

func TestPatchIntField(t *testing.T) {
	content := "---\nagent:\n  max_concurrent_agents: 3\n  max_turns: 60\n---\n\nPrompt body.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	err := workflow.PatchIntField(f, "max_concurrent_agents", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPatchIntFieldPreservesComments(t *testing.T) {
	content := "---\nagent:\n  max_concurrent_agents: 3 # set at runtime\n---\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	profiles := map[string]workflow.ProfileEntry{
		"fast":     {Command: "claude --model claude-haiku-4-5-20251001"},
		"thorough": {Command: "claude --model claude-opus-4-6"},
	}
	require.NoError(t, workflow.PatchProfilesBlock(f, profiles))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	assert.Contains(t, got, "  profiles:")
	assert.Contains(t, got, "    fast:")
	assert.Contains(t, got, "      command: claude --model claude-haiku-4-5-20251001")
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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	profiles := map[string]workflow.ProfileEntry{
		"fast": {Command: "claude --model claude-haiku-4-5-20251001"},
	}
	require.NoError(t, workflow.PatchProfilesBlock(f, profiles))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	assert.Contains(t, got, "    fast:")
	assert.Contains(t, got, "      command: claude --model claude-haiku-4-5-20251001")
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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	err := workflow.PatchStringSliceField(f, "nonexistent_key", []string{"a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPatchStringField_Replace(t *testing.T) {
	content := "---\ntracker:\n  completion_state: \"In Review\"\n---\n\nBody.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

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
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	err := workflow.PatchStringField(f, "nonexistent_key", "value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPatchProfilesBlock_PreservesOtherKeys(t *testing.T) {
	// Other agent keys and comments are unchanged.
	content := "---\n# Top comment\ntracker:\n  kind: linear\nagent:\n  # agent comment\n  max_concurrent_agents: 3\n  max_turns: 60\n  profiles:\n    old:\n      command: claude\nserver:\n  port: 8090\n---\n\nPrompt.\n"
	tmp := t.TempDir()
	f := filepath.Join(tmp, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0644))

	profiles := map[string]workflow.ProfileEntry{
		"fast": {Command: "claude --model claude-haiku-4-5-20251001"},
	}
	require.NoError(t, workflow.PatchProfilesBlock(f, profiles))

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	got := string(data)
	// New profile present
	assert.Contains(t, got, "    fast:")
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
