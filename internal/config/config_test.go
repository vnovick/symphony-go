package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/config"
)

func workflowWithContent(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "WORKFLOW.md")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))
	return f
}

func minimal(extras string) string {
	return "---\ntracker:\n  kind: linear\n  api_key: test-key\n  project_slug: my-project\n" + extras + "---\n\nPrompt.\n"
}

func TestDefaults(t *testing.T) {
	path := workflowWithContent(t, minimal(""))
	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "linear", cfg.Tracker.Kind)
	assert.Equal(t, "https://api.linear.app/graphql", cfg.Tracker.Endpoint)
	assert.Equal(t, []string{"Todo", "In Progress"}, cfg.Tracker.ActiveStates)
	assert.Equal(t, []string{"Closed", "Cancelled", "Canceled", "Duplicate", "Done"}, cfg.Tracker.TerminalStates)
	assert.Equal(t, 30000, cfg.Polling.IntervalMs)
	assert.Equal(t, 10, cfg.Agent.MaxConcurrentAgents)
	assert.Equal(t, 20, cfg.Agent.MaxTurns)
	assert.Equal(t, 300000, cfg.Agent.MaxRetryBackoffMs)
	assert.Equal(t, 5, cfg.Agent.MaxRetries)
	assert.Equal(t, "", cfg.Tracker.FailedState)
	assert.Equal(t, "claude", cfg.Agent.Command)
	assert.Equal(t, config.DefaultResumePrompt, cfg.Agent.ResumePrompt)
	assert.Equal(t, 3600000, cfg.Agent.TurnTimeoutMs)
	assert.Equal(t, 30000, cfg.Agent.ReadTimeoutMs)
	assert.Equal(t, 300000, cfg.Agent.StallTimeoutMs)
	assert.Equal(t, 60000, cfg.Hooks.TimeoutMs)
	assert.Nil(t, cfg.Server.Port)
}

func TestTrackerKindRequired(t *testing.T) {
	content := "---\ntracker:\n  api_key: key\n  project_slug: slug\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err) // Load no longer validates
	err = config.ValidateDispatch(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracker.kind")
}

func TestEnvVarResolution(t *testing.T) {
	t.Setenv("TEST_API_KEY", "resolved-key")
	content := "---\ntracker:\n  kind: linear\n  api_key: $TEST_API_KEY\n  project_slug: proj\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "resolved-key", cfg.Tracker.APIKey)
}

func TestEnvVarMissingBecomesEmpty(t *testing.T) {
	_ = os.Unsetenv("NONEXISTENT_VAR_XYZ")
	content := "---\ntracker:\n  kind: linear\n  api_key: $NONEXISTENT_VAR_XYZ\n  project_slug: proj\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Tracker.APIKey)
}

func TestTildeExpansionOnWorkspaceRoot(t *testing.T) {
	content := "---\ntracker:\n  kind: linear\n  api_key: key\n  project_slug: proj\nworkspace:\n  root: ~/itervox_ws\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, home+"/itervox_ws", cfg.Workspace.Root)
}

func TestAgentCommandNotTildeExpanded(t *testing.T) {
	content := "---\ntracker:\n  kind: linear\n  api_key: key\n  project_slug: proj\nagent:\n  command: ~/bin/claude\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	// agent.command is NOT path-expanded per spec
	assert.Equal(t, "~/bin/claude", cfg.Agent.Command)
}

func TestMaxRetriesExplicit(t *testing.T) {
	content := minimal("agent:\n  max_retries: 10\n")
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.Agent.MaxRetries)
}

func TestMaxRetriesZeroMeansUnlimited(t *testing.T) {
	content := minimal("agent:\n  max_retries: 0\n")
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Agent.MaxRetries)
}

func TestFailedStateExplicit(t *testing.T) {
	content := "---\ntracker:\n  kind: linear\n  api_key: test-key\n  project_slug: my-project\n  failed_state: \"Backlog\"\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "Backlog", cfg.Tracker.FailedState)
}

func TestMaxConcurrentAgentsByStateNormalized(t *testing.T) {
	content := minimal("agent:\n  max_concurrent_agents_by_state:\n    Todo: 3\n    IN PROGRESS: 2\n")
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 3, cfg.Agent.MaxConcurrentAgentsByState["todo"])
	assert.Equal(t, 2, cfg.Agent.MaxConcurrentAgentsByState["in progress"])
	_, hasTodo := cfg.Agent.MaxConcurrentAgentsByState["Todo"]
	assert.False(t, hasTodo, "original-case key should not be present")
}

func TestMaxConcurrentAgentsByStateInvalidIgnored(t *testing.T) {
	content := minimal("agent:\n  max_concurrent_agents_by_state:\n    todo: -1\n    inprog: 0\n")
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Empty(t, cfg.Agent.MaxConcurrentAgentsByState)
}

func TestHooksTimeoutNonPositiveFallsBackToDefault(t *testing.T) {
	content := minimal("hooks:\n  timeout_ms: 0\n")
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 60000, cfg.Hooks.TimeoutMs)
}

func TestWorkspaceRootDefault(t *testing.T) {
	path := workflowWithContent(t, minimal(""))
	cfg, err := config.Load(path)
	require.NoError(t, err)
	// Primary default: ~/.itervox/workspaces
	// Fallback (no home dir): <os.TempDir()>/itervox_workspaces
	// Both paths end in "workspaces".
	assert.Contains(t, cfg.Workspace.Root, "workspaces")
}

func TestPromptTemplate(t *testing.T) {
	path := workflowWithContent(t, minimal(""))
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "Prompt.", cfg.PromptTemplate)
}

func TestAgentProfileBackendField(t *testing.T) {
	content := minimal(`agent:
  profiles:
    codex-fast:
      command: codex --model o4-mini
      backend: codex
    inferred:
      command: codex --model o3
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Agent.Profiles)
	assert.Equal(t, "codex", cfg.Agent.Profiles["codex-fast"].Backend)
	assert.Equal(t, "", cfg.Agent.Profiles["inferred"].Backend)
}

func TestAgentBackendField(t *testing.T) {
	content := minimal(`agent:
  command: run-codex-wrapper
  backend: codex
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "run-codex-wrapper", cfg.Agent.Command)
	assert.Equal(t, "codex", cfg.Agent.Backend)
}

func TestAgentResumePromptField(t *testing.T) {
	content := minimal(`agent:
  resume_prompt: "Continue {{ issue.identifier }}"
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "Continue {{ issue.identifier }}", cfg.Agent.ResumePrompt)
}

func TestValidateDispatchRejectsInvalidResumePromptTemplate(t *testing.T) {
	content := minimal(`agent:
  resume_prompt: "{% for %}"
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent.resume_prompt")
}

func TestWorktreeDefaultsFalse(t *testing.T) {
	path := workflowWithContent(t, minimal(""))
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.False(t, cfg.Workspace.Worktree)
}

func TestWorktreeParsedTrue(t *testing.T) {
	content := minimal("workspace:\n  worktree: true\n")
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.Workspace.Worktree)
}

func TestWorkspaceCloneURL(t *testing.T) {
	content := "---\ntracker:\n  kind: linear\n  api_key: key\n  project_slug: proj\nworkspace:\n  root: /tmp/ws\n  worktree: true\n  clone_url: git@github.com:org/repo.git\n  base_branch: develop\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "git@github.com:org/repo.git", cfg.Workspace.CloneURL)
	assert.Equal(t, "develop", cfg.Workspace.BaseBranch)
}

func TestWorkspaceCloneURLDefault(t *testing.T) {
	content := "---\ntracker:\n  kind: linear\n  api_key: key\n  project_slug: proj\nworkspace:\n  root: /tmp/ws\n  worktree: true\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Workspace.CloneURL)
	assert.Equal(t, "main", cfg.Workspace.BaseBranch)
}
