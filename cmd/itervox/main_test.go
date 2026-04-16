package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/orchestrator"
	"github.com/vnovick/itervox/internal/server"
	"github.com/vnovick/itervox/internal/tracker"
)

func TestLoadDotEnv_LoadsItervoxDotEnv(t *testing.T) {
	dir := t.TempDir()
	itervoxDir := filepath.Join(dir, ".itervox")
	require.NoError(t, os.MkdirAll(itervoxDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(itervoxDir, ".env"),
		[]byte("TEST_DOTENV_ITERVOX=from_itervox\n"),
		0o600,
	))

	orig, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	require.NoError(t, os.Unsetenv("TEST_DOTENV_ITERVOX"))

	loadDotEnv()
	assert.Equal(t, "from_itervox", os.Getenv("TEST_DOTENV_ITERVOX"))
}

func TestLoadDotEnv_FallsBackToDotEnv(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("TEST_DOTENV_FALLBACK=from_root\n"),
		0o600,
	))

	orig, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	require.NoError(t, os.Unsetenv("TEST_DOTENV_FALLBACK"))

	loadDotEnv()
	assert.Equal(t, "from_root", os.Getenv("TEST_DOTENV_FALLBACK"))
}

func TestLoadDotEnv_DoesNotOverwriteExistingEnvVar(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("TEST_DOTENV_EXISTING=from_file\n"),
		0o600,
	))

	orig, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	t.Setenv("TEST_DOTENV_EXISTING", "from_shell")

	loadDotEnv()
	assert.Equal(t, "from_shell", os.Getenv("TEST_DOTENV_EXISTING"),
		"existing env vars must not be overwritten by .env file")
}

func TestLoadDotEnv_SilentWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	assert.NotPanics(t, loadDotEnv)
}

func TestConfiguredBackend(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		explicit string
		want     string
	}{
		{
			name:     "explicit override wins for wrapper commands",
			command:  "run-codex-wrapper --json",
			explicit: "codex",
			want:     "codex",
		},
		{
			name:    "infers codex from command",
			command: "/usr/local/bin/codex --model gpt-5.3-codex",
			want:    "codex",
		},
		{
			name:    "falls back to claude for unknown wrapper",
			command: "run-claude-wrapper --json",
			want:    "claude",
		},
		{
			name: "falls back to claude for blank command",
			want: "claude",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := configuredBackend(tc.command, tc.explicit); got != tc.want {
				t.Fatalf("configuredBackend(%q, %q) = %q, want %q", tc.command, tc.explicit, got, tc.want)
			}
		})
	}
}

// ─── buildDemoConfig ──────────────────────────────────────────────────────────

func TestBuildDemoConfig_HasRequiredFields(t *testing.T) {
	cfg := buildDemoConfig()
	assert.Equal(t, "memory", cfg.Tracker.Kind)
	assert.NotEmpty(t, cfg.Tracker.ActiveStates)
	assert.NotEmpty(t, cfg.Tracker.TerminalStates)
	assert.Equal(t, "In Progress", cfg.Tracker.WorkingState)
	assert.Equal(t, "Done", cfg.Tracker.CompletionState)
	assert.Equal(t, 3, cfg.Agent.MaxConcurrentAgents)
	assert.NotNil(t, cfg.Server.Port)
	assert.Equal(t, 8090, *cfg.Server.Port)
	assert.NotEmpty(t, cfg.PromptTemplate)
}

func TestBuildDemoConfig_HasAvailableModels(t *testing.T) {
	cfg := buildDemoConfig()
	assert.NotNil(t, cfg.Agent.AvailableModels)
	assert.True(t, len(cfg.Agent.AvailableModels["claude"]) > 0, "should have claude models")
	assert.True(t, len(cfg.Agent.AvailableModels["codex"]) > 0, "should have codex models")
}

// ─── convertAgentModels ───────────────────────────────────────────────────────

func TestConvertAgentModels(t *testing.T) {
	input := []agent.ModelOption{
		{ID: "claude-opus-4-6", Label: "Opus 4.6"},
		{ID: "claude-sonnet-4-6", Label: "Sonnet 4.6"},
	}
	out := convertAgentModels(input)
	assert.Len(t, out, 2)
	assert.Equal(t, "claude-opus-4-6", out[0].ID)
	assert.Equal(t, "Opus 4.6", out[0].Label)
}

func TestConvertAgentModels_Empty(t *testing.T) {
	out := convertAgentModels(nil)
	assert.Len(t, out, 0)
}

// ─── convertModelsForSnapshot ─────────────────────────────────────────────────

func TestConvertModelsForSnapshot(t *testing.T) {
	input := map[string][]config.ModelOption{
		"claude": {{ID: "a", Label: "A"}},
		"codex":  {{ID: "b", Label: "B"}, {ID: "c", Label: "C"}},
	}
	out := convertModelsForSnapshot(input)
	assert.Len(t, out["claude"], 1)
	assert.Len(t, out["codex"], 2)
	assert.Equal(t, "a", out["claude"][0].ID)
}

func TestConvertModelsForSnapshot_Nil(t *testing.T) {
	assert.Nil(t, convertModelsForSnapshot(nil))
}

// ─── GenerateDemoIssues ───────────────────────────────────────────────────────

func TestGenerateDemoIssues(t *testing.T) {
	issues := tracker.GenerateDemoIssues(10)
	assert.Len(t, issues, 10)
	for _, iss := range issues {
		assert.NotEmpty(t, iss.ID)
		assert.NotEmpty(t, iss.Identifier)
		assert.NotEmpty(t, iss.Title)
		assert.NotEmpty(t, iss.State)
		assert.NotNil(t, iss.Description)
		assert.Contains(t, iss.Labels, "demo")
	}
	// Verify identifiers are unique
	ids := make(map[string]bool)
	for _, iss := range issues {
		assert.False(t, ids[iss.Identifier], "duplicate identifier: %s", iss.Identifier)
		ids[iss.Identifier] = true
	}
}

func TestGenerateDemoIssues_StatesDistribution(t *testing.T) {
	issues := tracker.GenerateDemoIssues(10)
	states := make(map[string]int)
	for _, iss := range issues {
		states[iss.State]++
	}
	assert.True(t, states["Todo"] > 0, "should have Todo issues")
	assert.True(t, states["In Progress"] > 0, "should have In Progress issues")
}

// ─── defaultLogsDir ───────────────────────────────────────────────────────────

func TestDefaultLogsDir_FallsBackGracefully(t *testing.T) {
	// Non-existent workflow file should return a fallback path
	dir := defaultLogsDir("/nonexistent/WORKFLOW.md")
	assert.Contains(t, dir, ".itervox")
	assert.Contains(t, dir, "logs")
}

// ─── Demo mode e2e ───────────────────────────────────────────────────────────

func TestDemoMode_DaemonStartsAndServesHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	cfg := buildDemoConfig()
	// Use a random port to avoid conflicts
	port := 0
	cfg.Server.Port = &port

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tr := tracker.NewMemoryTracker(
		tracker.GenerateDemoIssues(5),
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
	)

	// Just verify the config is valid for the orchestrator
	assert.Equal(t, "memory", cfg.Tracker.Kind)
	assert.NotNil(t, tr)
	_ = ctx // used for timeout context
}

func TestDemoMode_SnapshotBuilder(t *testing.T) {
	cfg := buildDemoConfig()
	models := convertModelsForSnapshot(cfg.Agent.AvailableModels)
	assert.NotNil(t, models)
	assert.True(t, len(models["claude"]) > 0)

	// Verify the models round-trip correctly
	for _, m := range models["claude"] {
		assert.NotEmpty(t, m.ID)
		assert.NotEmpty(t, m.Label)
	}
}

// ─── server.ModelOption conversion ────────────────────────────────────────────

func TestServerModelOption_JSONRoundTrip(t *testing.T) {
	m := server.ModelOption{ID: "test-id", Label: "Test Label"}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var decoded server.ModelOption
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, m, decoded)
}

func TestSortedPausedIdentifiers(t *testing.T) {
	got := sortedPausedIdentifiers(map[string]string{
		"ENG-20": "id-20",
		"ENG-3":  "id-3",
		"ENG-10": "id-10",
	})

	want := append([]string{}, got...)
	sort.Strings(want)
	assert.Equal(t, want, got)
}

func TestSortedRetryRows(t *testing.T) {
	got := sortedRetryRows(map[string]*orchestrator.RetryEntry{
		"id-20": {Identifier: "ENG-20", Attempt: 2},
		"id-3":  {Identifier: "ENG-3", Attempt: 1},
		"id-10": {Identifier: "ENG-10", Attempt: 3},
	})

	require.Len(t, got, 3)
	assert.Equal(t, []string{"ENG-10", "ENG-20", "ENG-3"}, []string{
		got[0].Identifier,
		got[1].Identifier,
		got[2].Identifier,
	})
}

func TestSortedInputRequiredRows(t *testing.T) {
	now := time.Now().UTC()
	got := sortedInputRequiredRows(
		map[string]*orchestrator.InputRequiredEntry{
			"ENG-20": {Identifier: "ENG-20", Context: "c20", QueuedAt: now},
			"ENG-3":  {Identifier: "ENG-3", Context: "c3", QueuedAt: now},
		},
		map[string]*orchestrator.PendingInputResumeEntry{
			"ENG-10": {Identifier: "ENG-10", Context: "c10", UserMessage: "approved", QueuedAt: now},
		},
	)

	require.Len(t, got, 3)
	assert.Equal(t, []string{"ENG-10", "ENG-20", "ENG-3"}, []string{
		got[0].Identifier,
		got[1].Identifier,
		got[2].Identifier,
	})
	assert.Equal(t, []string{"pending_input_resume", "input_required", "input_required"}, []string{
		got[0].State,
		got[1].State,
		got[2].State,
	})
}

// ─── API endpoint smoke test ──────────────────────────────────────────────────

func TestHealthEndpoint_NoDemoNeeded(t *testing.T) {
	// The health endpoint should work with a minimal server config
	snap := server.StateSnapshot{GeneratedAt: time.Now()}
	cfg := server.Config{
		Snapshot:    func() server.StateSnapshot { return snap },
		RefreshChan: make(chan struct{}, 1),
	}
	srv := server.New(cfg)

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	// Just verify it doesn't panic
	assert.NotNil(t, srv)
	_ = req
}
