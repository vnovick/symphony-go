package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListClaudeModels_FallsBackWithoutKey(t *testing.T) {
	// When ANTHROPIC_API_KEY is not set, should return the default list.
	t.Setenv("ANTHROPIC_API_KEY", "")
	models := ListClaudeModels()
	assert.Equal(t, DefaultClaudeModels, models, "should return default models when no API key")
	assert.True(t, len(models) > 0, "default list should not be empty")
}

func TestListCodexModels_FallsBackWithoutKey(t *testing.T) {
	// When OPENAI_API_KEY is not set, should return the default list.
	t.Setenv("OPENAI_API_KEY", "")
	models := ListCodexModels()
	assert.Equal(t, DefaultCodexModels, models, "should return default models when no API key")
	assert.True(t, len(models) > 0, "default list should not be empty")
}

func TestListClaudeModels_FallsBackOnInvalidKey(t *testing.T) {
	// With an invalid key, the API returns 401 and we fall back to defaults.
	t.Setenv("ANTHROPIC_API_KEY", "invalid-key-xxx")
	models := ListClaudeModels()
	assert.Equal(t, DefaultClaudeModels, models, "should return default models on API failure")
}

func TestListCodexModels_FallsBackOnInvalidKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "invalid-key-xxx")
	models := ListCodexModels()
	assert.Equal(t, DefaultCodexModels, models, "should return default models on API failure")
}

func TestDefaultClaudeModels_HasExpectedEntries(t *testing.T) {
	ids := make([]string, len(DefaultClaudeModels))
	for i, m := range DefaultClaudeModels {
		ids[i] = m.ID
	}
	assert.Contains(t, ids, "claude-sonnet-4-6")
	assert.Contains(t, ids, "claude-opus-4-6")
}

func TestDefaultCodexModels_HasExpectedEntries(t *testing.T) {
	ids := make([]string, len(DefaultCodexModels))
	for i, m := range DefaultCodexModels {
		ids[i] = m.ID
	}
	assert.Contains(t, ids, "gpt-5.2-codex")
}

func TestModelOption_Fields(t *testing.T) {
	m := ModelOption{ID: "test-model", Label: "Test Model"}
	assert.Equal(t, "test-model", m.ID)
	assert.Equal(t, "Test Model", m.Label)
}
