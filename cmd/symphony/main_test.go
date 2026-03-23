package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDotEnv_LoadsSymphonyDotEnv(t *testing.T) {
	dir := t.TempDir()
	symphonyDir := filepath.Join(dir, ".symphony")
	require.NoError(t, os.MkdirAll(symphonyDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(symphonyDir, ".env"),
		[]byte("TEST_DOTENV_SYMPHONY=from_symphony\n"),
		0o600,
	))

	orig, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	require.NoError(t, os.Unsetenv("TEST_DOTENV_SYMPHONY"))

	loadDotEnv()
	assert.Equal(t, "from_symphony", os.Getenv("TEST_DOTENV_SYMPHONY"))
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
