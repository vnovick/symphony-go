package orchestrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrependEnvToCommand_PreservesBackendHintPrefix(t *testing.T) {
	command := "@@itervox-backend=codex /tmp/codex-wrapper --flag"

	got := prependEnvToCommand(command, map[string]string{
		"ITERVOX_ACTION_TOKEN": "token value",
		"PATH":                 "/tmp/bin:/usr/bin",
	})

	assert.True(t, strings.HasPrefix(got, "@@itervox-backend=codex "), "backend hint should stay at the front so runner dispatch remains stable")
	assert.Contains(t, got, "ITERVOX_ACTION_TOKEN='token value'")
	assert.Contains(t, got, "PATH='/tmp/bin:/usr/bin'")
	assert.Contains(t, got, "/tmp/codex-wrapper --flag")
}
