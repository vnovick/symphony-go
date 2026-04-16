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

func TestAgentProfileAllowedActionsField(t *testing.T) {
	content := minimal(`agent:
  profiles:
    responder:
      command: claude --model claude-sonnet-4-6
      allowed_actions:
        - comment
        - provide_input
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Agent.Profiles)
	assert.Equal(t, []string{"comment", "provide_input"}, cfg.Agent.Profiles["responder"].AllowedActions)
}

func TestAgentProfileCreateIssueStateField(t *testing.T) {
	content := minimal(`agent:
  profiles:
    triage:
      command: claude --model claude-sonnet-4-6
      allowed_actions:
        - create_issue
      create_issue_state: Todo
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Agent.Profiles)
	assert.Equal(t, []string{"create_issue"}, cfg.Agent.Profiles["triage"].AllowedActions)
	assert.Equal(t, "Todo", cfg.Agent.Profiles["triage"].CreateIssueState)
}

func TestAgentProfileEnabledField(t *testing.T) {
	content := minimal(`agent:
  profiles:
    active:
      command: claude --model claude-sonnet-4-6
    paused:
      command: codex --model gpt-5.3-codex
      enabled: false
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Agent.Profiles)
	assert.True(t, config.ProfileEnabled(cfg.Agent.Profiles["active"]))
	assert.False(t, config.ProfileEnabled(cfg.Agent.Profiles["paused"]))
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

func TestAutomationsParsed(t *testing.T) {
	content := minimal(`automations:
  - id: backlog-review
    enabled: true
    profile: reviewer
    instructions: "Review backlog issues and comment with missing details."
    trigger:
      type: cron
      cron: "0 9 * * 1"
      timezone: "Asia/Jerusalem"
    filter:
      match_mode: any
      states: ["Backlog","Todo"]
      labels_any: ["bug"]
      identifier_regex: "^ENG-"
      limit: 2
  - id: moved-to-backlog
    enabled: true
    profile: pm
    instructions: "Review why the issue returned to backlog."
    trigger:
      type: issue_moved_to_backlog
  - id: qa-state-entry
    enabled: true
    profile: qa
    instructions: "Run QA when the issue enters Ready for QA."
    trigger:
      type: issue_entered_state
      state: "Ready for QA"
  - id: comment-triage
    enabled: true
    profile: reviewer
    instructions: "React to new tracker comments."
    trigger:
      type: tracker_comment_added
  - id: failed-run
    enabled: true
    profile: reviewer
    instructions: "Summarise the failed run and suggest next action."
    trigger:
      type: run_failed
  - id: input-responder
    enabled: true
    profile: input-responder
    instructions: "Answer low-risk blocked-run questions."
    trigger:
      type: input_required
    filter:
      input_context_regex: "continue|branch"
    policy:
      auto_resume: true
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Len(t, cfg.Automations, 6)
	assert.Equal(t, "backlog-review", cfg.Automations[0].ID)
	assert.Equal(t, "cron", cfg.Automations[0].Trigger.Type)
	assert.Equal(t, "0 9 * * 1", cfg.Automations[0].Trigger.Cron)
	assert.Equal(t, "Asia/Jerusalem", cfg.Automations[0].Trigger.Timezone)
	assert.Equal(t, "reviewer", cfg.Automations[0].Profile)
	assert.Equal(t, "any", cfg.Automations[0].Filter.MatchMode)
	assert.Equal(t, []string{"Backlog", "Todo"}, cfg.Automations[0].Filter.States)
	assert.Equal(t, []string{"bug"}, cfg.Automations[0].Filter.LabelsAny)
	assert.Equal(t, "^ENG-", cfg.Automations[0].Filter.IdentifierRegex)
	assert.Equal(t, 2, cfg.Automations[0].Filter.Limit)
	assert.Equal(t, "issue_moved_to_backlog", cfg.Automations[1].Trigger.Type)
	assert.Equal(t, "issue_entered_state", cfg.Automations[2].Trigger.Type)
	assert.Equal(t, "Ready for QA", cfg.Automations[2].Trigger.State)
	assert.Equal(t, "tracker_comment_added", cfg.Automations[3].Trigger.Type)
	assert.Equal(t, "run_failed", cfg.Automations[4].Trigger.Type)
	assert.Equal(t, "input_required", cfg.Automations[5].Trigger.Type)
	assert.Equal(t, "continue|branch", cfg.Automations[5].Filter.InputContextRegex)
	assert.True(t, cfg.Automations[5].Policy.AutoResume)
}

func TestLegacySchedulesParsedAsCronAutomations(t *testing.T) {
	content := minimal(`schedules:
  - id: weekday-review
    enabled: true
    cron: "0 9 * * 1-5"
    timezone: "UTC"
    profile: reviewer
    filter:
      states: ["Backlog"]
      labels_any: ["triage"]
`)
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Len(t, cfg.Automations, 1)
	assert.Equal(t, "weekday-review", cfg.Automations[0].ID)
	assert.Equal(t, "cron", cfg.Automations[0].Trigger.Type)
	assert.Equal(t, "0 9 * * 1-5", cfg.Automations[0].Trigger.Cron)
	assert.Equal(t, "UTC", cfg.Automations[0].Trigger.Timezone)
	assert.Equal(t, "reviewer", cfg.Automations[0].Profile)
	assert.Equal(t, []string{"Backlog"}, cfg.Automations[0].Filter.States)
	assert.Equal(t, []string{"triage"}, cfg.Automations[0].Filter.LabelsAny)
}
