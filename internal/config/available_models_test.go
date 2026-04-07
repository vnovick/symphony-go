package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAvailableModels_ValidInput(t *testing.T) {
	raw := map[string]any{
		"claude": []any{
			map[string]any{"id": "claude-sonnet-4-6", "label": "Sonnet 4.6"},
			map[string]any{"id": "claude-opus-4-6", "label": "Opus 4.6"},
		},
		"codex": []any{
			map[string]any{"id": "gpt-5.2-codex", "label": "GPT-5.2 Codex"},
		},
	}
	result := parseAvailableModels(raw)
	require.NotNil(t, result)
	assert.Len(t, result["claude"], 2)
	assert.Len(t, result["codex"], 1)
	assert.Equal(t, "claude-sonnet-4-6", result["claude"][0].ID)
	assert.Equal(t, "Sonnet 4.6", result["claude"][0].Label)
}

func TestParseAvailableModels_EmptyMap(t *testing.T) {
	assert.Nil(t, parseAvailableModels(nil))
	assert.Nil(t, parseAvailableModels(map[string]any{}))
}

func TestParseAvailableModels_MissingLabelFallsBackToID(t *testing.T) {
	raw := map[string]any{
		"claude": []any{
			map[string]any{"id": "claude-sonnet-4-6"},
		},
	}
	result := parseAvailableModels(raw)
	require.NotNil(t, result)
	assert.Equal(t, "claude-sonnet-4-6", result["claude"][0].Label)
}

func TestParseAvailableModels_SkipsInvalidEntries(t *testing.T) {
	raw := map[string]any{
		"claude": []any{
			map[string]any{"id": "valid", "label": "Valid"},
			map[string]any{"label": "no-id"},
			"not-a-map",
		},
	}
	result := parseAvailableModels(raw)
	require.NotNil(t, result)
	assert.Len(t, result["claude"], 1)
}

func TestLoad_ParsesAvailableModels(t *testing.T) {
	content := `---
tracker:
  kind: linear
  api_key: test
agent:
  command: claude
  available_models:
    claude:
      - { id: "claude-sonnet-4-6", label: "Sonnet 4.6" }
      - { id: "claude-opus-4-6", label: "Opus 4.6" }
    codex:
      - { id: "gpt-5.2-codex", label: "GPT-5.2 Codex" }
---
prompt body
`
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Agent.AvailableModels)
	assert.Len(t, cfg.Agent.AvailableModels["claude"], 2)
	assert.Len(t, cfg.Agent.AvailableModels["codex"], 1)
	assert.Equal(t, "claude-sonnet-4-6", cfg.Agent.AvailableModels["claude"][0].ID)
	assert.Equal(t, "Sonnet 4.6", cfg.Agent.AvailableModels["claude"][0].Label)
}

func TestLoad_NoAvailableModels(t *testing.T) {
	content := `---
tracker:
  kind: linear
  api_key: test
agent:
  command: claude
---
prompt
`
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Nil(t, cfg.Agent.AvailableModels)
}
