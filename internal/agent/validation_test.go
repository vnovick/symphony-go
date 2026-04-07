package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/agent"
)

func TestValidateCLI_NotFound(t *testing.T) {
	cases := []struct {
		name     string
		validate func() error
		wantMsg  string
	}{
		{"claude", agent.ValidateClaudeCLI, "claude CLI not available"},
		{"codex", agent.ValidateCodexCLI, "codex CLI not available"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := os.Getenv("PATH")
			defer func() { _ = os.Setenv("PATH", orig) }()
			require.NoError(t, os.Setenv("PATH", t.TempDir()))
			err := tc.validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestValidateCLI_FakeBinary(t *testing.T) {
	cases := []struct {
		binName  string
		validate func() error
	}{
		{"claude", agent.ValidateClaudeCLI},
		{"codex", agent.ValidateCodexCLI},
	}
	for _, tc := range cases {
		t.Run(tc.binName, func(t *testing.T) {
			orig := os.Getenv("PATH")
			defer func() { _ = os.Setenv("PATH", orig) }()
			tmpDir := t.TempDir()
			script := "#!/bin/sh\necho 'version 1.0.0'"
			require.NoError(t, os.WriteFile(filepath.Join(tmpDir, tc.binName), []byte(script), 0o755))
			require.NoError(t, os.Setenv("PATH", tmpDir))
			assert.NoError(t, tc.validate())
		})
	}
}

func TestValidateCodexCLITimeout(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", origPath) }()

	// Create a fake codex binary that hangs
	tmpDir := t.TempDir()
	fakeCodex := filepath.Join(tmpDir, "codex")
	script := "#!/bin/sh\nsleep 10" // Will timeout after 5s
	require.NoError(t, os.WriteFile(fakeCodex, []byte(script), 0o755))

	require.NoError(t, os.Setenv("PATH", tmpDir))
	err := agent.ValidateCodexCLI()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}
