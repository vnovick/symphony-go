package orchestrator

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/logbuffer"
	"github.com/vnovick/itervox/internal/tracker"
)

func TestBuildSubAgentContext_UsesCodexToolName(t *testing.T) {
	ctx := buildSubAgentContext(map[string]config.AgentProfile{
		"research": {Prompt: "Investigate complex issues."},
	}, "", "codex")

	assert.Contains(t, ctx, "spawn_agent tool")
	assert.NotContains(t, ctx, "Task tool")
}

func TestBuildSubAgentContext_SkipsActiveProfile(t *testing.T) {
	ctx := buildSubAgentContext(map[string]config.AgentProfile{
		"active":    {Prompt: "Current worker"},
		"secondary": {Prompt: "Helper"},
	}, "active", "claude")

	assert.Contains(t, ctx, "secondary")
	assert.NotContains(t, ctx, "**active**")
}

func TestShouldUseClaudeResumePrompt(t *testing.T) {
	id := "sess-1"
	empty := ""

	assert.True(t, shouldUseClaudeResumePrompt("", &id))
	assert.True(t, shouldUseClaudeResumePrompt("claude", &id))
	assert.False(t, shouldUseClaudeResumePrompt("codex", &id))
	assert.False(t, shouldUseClaudeResumePrompt("claude", &empty))
	assert.False(t, shouldUseClaudeResumePrompt("claude", nil))
}

func TestRenderClaudeResumePrompt(t *testing.T) {
	got, err := renderClaudeResumePrompt("Continue {{ issue.identifier }}", domain.Issue{Identifier: "ENG-1"}, nil)
	assert.NoError(t, err)
	assert.Equal(t, "Continue ENG-1", got)
}

func TestRenderClaudeResumePromptDefault(t *testing.T) {
	got, err := renderClaudeResumePrompt("", domain.Issue{Identifier: "ENG-1"}, nil)
	assert.NoError(t, err)
	assert.Equal(t, config.DefaultResumePrompt, got)
}

func TestAppendPromptSections(t *testing.T) {
	got := appendPromptSections("Resume", "Profile context", "", "## Open PR Context\nPR: https://github.com/acme/repo/pull/1")

	assert.Equal(t, "Resume\n\nProfile context\n\n## Open PR Context\nPR: https://github.com/acme/repo/pull/1", got)
}

type promptCapturingRunner struct {
	prompt    string
	sessionID string
}

func (r *promptCapturingRunner) RunTurn(_ context.Context, _ agent.Logger, _ func(agent.TurnResult), sessionID *string, prompt, _, _, _, _ string, _, _ int) (agent.TurnResult, error) {
	r.prompt = prompt
	if sessionID != nil {
		r.sessionID = *sessionID
	}
	return agent.TurnResult{SessionID: r.sessionID, ResultText: "done"}, nil
}

func TestManualClaudeResumePromptIncludesFirstTurnContext(t *testing.T) {
	cfg := &config.Config{
		PromptTemplate: "Full prompt for {{ issue.identifier }}.",
	}
	cfg.Tracker.ActiveStates = []string{"In Progress"}
	cfg.Tracker.TerminalStates = []string{"Done"}
	cfg.Agent.MaxTurns = 1
	cfg.Agent.Command = "claude"
	cfg.Agent.ResumePrompt = "Resume {{ issue.identifier }}."
	cfg.Agent.AgentMode = "teams"
	cfg.Agent.Profiles = map[string]config.AgentProfile{
		"lead": {
			Command: "claude",
			Prompt:  "Lead role for {{ issue.identifier }}.",
		},
		"research": {
			Command: "claude",
			Prompt:  "Research helper.",
		},
	}
	issue := domain.Issue{ID: "id1", Identifier: "ENG-1", Title: "T", State: "In Progress"}
	mt := tracker.NewMemoryTracker([]domain.Issue{issue}, cfg.Tracker.ActiveStates, cfg.Tracker.TerminalStates)
	runner := &promptCapturingRunner{}
	o := New(cfg, mt, runner, nil)

	o.runWorker(context.Background(), issue, 0, "", "claude", "claude", "lead", true, "sess-1")

	assert.Equal(t, "sess-1", runner.sessionID)
	assert.Contains(t, runner.prompt, "Resume ENG-1.")
	assert.NotContains(t, runner.prompt, "Full prompt for ENG-1.")
	assert.Contains(t, runner.prompt, "Lead role for ENG-1.")
	assert.Contains(t, runner.prompt, "## Available Sub-Agents")
	assert.Contains(t, runner.prompt, "research")
}

// --- formatBufLine / makeBufLine (JSON output) ---

func TestFormatBufLine_IncludesLevelAndMessage(t *testing.T) {
	line := formatBufLine("INFO", "hello world", nil)
	assert.Contains(t, line, `"level":"INFO"`, "got: %q", line)
	assert.Contains(t, line, `"msg":"hello world"`)
	assert.Contains(t, line, `"time":`)
}

func TestFormatBufLine_IncludesKeyValuePairs(t *testing.T) {
	line := formatBufLine("WARN", "something failed", []any{"tool", "Bash", "description", "ls"})
	assert.Contains(t, line, `"tool":"Bash"`)
	assert.Contains(t, line, `"description":"ls"`)
}

func TestFormatBufLine_TextFieldPopulated(t *testing.T) {
	line := formatBufLine("INFO", "claude: text", []any{"text", "has spaces"})
	assert.Contains(t, line, `"text":"has spaces"`)
}

func TestMakeBufLine_IncludesLevelAndTime(t *testing.T) {
	line := makeBufLine("INFO", "hello")
	assert.Contains(t, line, `"level":"INFO"`, "got: %q", line)
	assert.Contains(t, line, `"msg":"hello"`)
	assert.Contains(t, line, `"time":`)
}

// --- bufLogger.Debug / Warn ---

func TestBufLogger_Debug_DoesNotWriteToBuffer(t *testing.T) {
	buf := logbuffer.New()
	l := &bufLogger{base: slog.Default(), buf: buf, identifier: "ENG-1"}
	l.Debug("debug message", "key", "val")
	// Debug should not add to the buffer.
	assert.Nil(t, buf.Get("ENG-1"))
}

func TestBufLogger_Warn_WritesToBuffer(t *testing.T) {
	buf := logbuffer.New()
	l := &bufLogger{base: slog.Default(), buf: buf, identifier: "ENG-1"}
	l.Warn("something went wrong", "key", "val")
	lines := buf.Get("ENG-1")
	assert.Len(t, lines, 1)
	assert.Contains(t, lines[0], "WARN")
	assert.Contains(t, lines[0], "something went wrong")
}

func TestBufLogger_Info_WritesToBuffer(t *testing.T) {
	buf := logbuffer.New()
	l := &bufLogger{base: slog.Default(), buf: buf, identifier: "ENG-2"}
	l.Info("task dispatched", "issue", "ENG-2")
	lines := buf.Get("ENG-2")
	assert.Len(t, lines, 1)
	assert.Contains(t, lines[0], "INFO")
}
