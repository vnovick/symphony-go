package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/config"
)

func validWorkflowPath(t *testing.T) string {
	t.Helper()
	return workflowWithContent(t, minimal(""))
}

func TestValidateDispatchPassesForValidConfig(t *testing.T) {
	path := validWorkflowPath(t)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	assert.NoError(t, err)
}

func TestValidateDispatchFailsMissingTrackerKind(t *testing.T) {
	content := "---\ntracker:\n  api_key: key\n  project_slug: proj\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracker.kind")
}

func TestValidateDispatchFailsUnsupportedTrackerKind(t *testing.T) {
	content := "---\ntracker:\n  kind: jira\n  api_key: key\n  project_slug: proj\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported_tracker_kind")
}

func TestValidateDispatchFailsMissingAPIKey(t *testing.T) {
	_ = os.Unsetenv("MISSING_KEY_XYZ")
	content := "---\ntracker:\n  kind: linear\n  api_key: $MISSING_KEY_XYZ\n  project_slug: proj\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracker.api_key")
}

func TestValidateDispatchLinearOKWithoutProjectSlug(t *testing.T) {
	// Linear project_slug is optional — project is selected via TUI/dashboard.
	content := "---\ntracker:\n  kind: linear\n  api_key: mykey\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	assert.NoError(t, err)
}

func TestValidateDispatchGitHubFailsMissingProjectSlug(t *testing.T) {
	// GitHub project_slug (owner/repo) is required — it identifies the target repo.
	content := "---\ntracker:\n  kind: github\n  api_key: ghtoken\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracker.project_slug")
}

func TestValidateDispatchFailsMissingAgentCommand(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tracker.Kind = "linear"
	cfg.Tracker.APIKey = "key"
	cfg.Tracker.ProjectSlug = "proj"
	cfg.Agent.Command = "" // explicitly blank

	err := config.ValidateDispatch(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent.command")
}

func TestValidateDispatchGitHubKindAccepted(t *testing.T) {
	content := "---\ntracker:\n  kind: github\n  api_key: ghtoken\n  project_slug: owner/repo\n---\n\nPrompt.\n"
	path := workflowWithContent(t, content)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	err = config.ValidateDispatch(cfg)
	assert.NoError(t, err)
}
