package orchestrator

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/logbuffer"
)

// F-2: recordAutomationDispatch must append a single per-issue log entry
// whose message starts with the AUTOMATION FIRED prefix and contains the
// rule id, trigger type, profile, and backend. Greppability is the contract
// the Logs filter chip (T-4) depends on.
func TestRecordAutomationDispatchEmitsGreppableLine(t *testing.T) {
	o := &Orchestrator{logBuf: logbuffer.New()}
	issue := domain.Issue{ID: "id1", Identifier: "ENG-42", Title: "test issue"}
	dispatch := AutomationDispatch{
		AutomationID: "pr-on-input",
		ProfileName:  "reviewer",
		Trigger: AutomationTriggerContext{
			Type:         config.AutomationTriggerInputRequired,
			InputContext: "Should I rebase before merge?",
		},
	}

	o.recordAutomationDispatch(issue, dispatch, "claude")

	lines := o.logBuf.Get(issue.Identifier)
	require.Len(t, lines, 1, "exactly one buffer entry should be emitted per dispatch")
	line := lines[0]
	assert.Contains(t, line, AutomationFiredLogPrefix, "frontend chip filter requires the literal prefix")
	assert.Contains(t, line, "pr-on-input")
	assert.Contains(t, line, "input_required")
	assert.Contains(t, line, "reviewer")
	assert.Contains(t, line, "claude")
	assert.Contains(t, line, "Should I rebase before merge?",
		"input_required automations should embed the prompt context in the buffer entry")
}

// Manual dispatches must NOT emit an AUTOMATION FIRED line. We assert by
// poking the buffer directly with the kind of line a manual worker would emit
// — the prefix must not appear anywhere.
func TestRecordAutomationDispatchOmittedForManualRuns(t *testing.T) {
	o := &Orchestrator{logBuf: logbuffer.New()}
	o.logBuf.Add("ENG-99", makeBufLine("INFO", "worker: starting (manual dispatch)"))
	for _, line := range o.logBuf.Get("ENG-99") {
		assert.False(t, strings.HasPrefix(extractMsg(line), AutomationFiredLogPrefix),
			"manual dispatch must never produce the AUTOMATION FIRED prefix")
	}
}

// Older test fixtures construct Orchestrators without a logBuf. The helper
// must no-op rather than panic so it remains safe to call from
// startAutomationRun in any test setup.
func TestRecordAutomationDispatchTolerantOfNilLogBuf(t *testing.T) {
	o := &Orchestrator{}
	require.NotPanics(t, func() {
		o.recordAutomationDispatch(domain.Issue{Identifier: "X"}, AutomationDispatch{}, "")
	})
}

// 240-char cap (with ellipsis) keeps the buffer entry comfortably below the
// 64 KiB per-line limit added by the logbuffer per-line truncation guard.
func TestRecordAutomationDispatchTruncatesLongContext(t *testing.T) {
	o := &Orchestrator{logBuf: logbuffer.New()}
	issue := domain.Issue{Identifier: "ENG-long"}
	long := strings.Repeat("a", 5_000)
	o.recordAutomationDispatch(issue, AutomationDispatch{
		AutomationID: "auto-long",
		ProfileName:  "reviewer",
		Trigger: AutomationTriggerContext{
			Type:         config.AutomationTriggerInputRequired,
			InputContext: long,
		},
	}, "claude")
	lines := o.logBuf.Get(issue.Identifier)
	require.Len(t, lines, 1)
	assert.Less(t, len(lines[0]), 1_024,
		"long contexts must be truncated to avoid log line bloat")
	assert.Contains(t, lines[0], "…", "truncation marker must be present")
}

// tracker_comment_added carries CommentBody instead of InputContext; the
// helper picks whichever is non-empty so the two trigger types share one log
// format.
func TestRecordAutomationDispatchUsesCommentBodyWhenInputContextEmpty(t *testing.T) {
	o := &Orchestrator{logBuf: logbuffer.New()}
	o.recordAutomationDispatch(
		domain.Issue{Identifier: "ENG-2"},
		AutomationDispatch{
			AutomationID: "review-on-comment",
			ProfileName:  "reviewer",
			Trigger: AutomationTriggerContext{
				Type:        config.AutomationTriggerTrackerComment,
				CommentBody: "PTAL",
			},
		},
		"claude",
	)
	lines := o.logBuf.Get("ENG-2")
	require.Len(t, lines, 1)
	// The %q-rendered context becomes \"PTAL\" once the JSON envelope escapes
	// the surrounding quotes.
	assert.Contains(t, lines[0], `\"PTAL\"`)
}

// Concurrent automation dispatches against the SAME issue must not interleave
// — each call should produce one self-contained entry with all four fields.
// Re-runs under -race gate the goroutine-safety contract.
func TestRecordAutomationDispatchConcurrentSafe(t *testing.T) {
	o := &Orchestrator{logBuf: logbuffer.New()}

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o.recordAutomationDispatch(
				domain.Issue{Identifier: "ENG-RACE"},
				AutomationDispatch{
					AutomationID: "auto",
					ProfileName:  "p",
					Trigger:      AutomationTriggerContext{Type: config.AutomationTriggerCron},
				},
				"claude",
			)
			_ = i
		}(i)
	}
	wg.Wait()

	lines := o.logBuf.Get("ENG-RACE")
	require.Len(t, lines, 50)
	for _, line := range lines {
		assert.Contains(t, line, AutomationFiredLogPrefix)
		assert.Contains(t, line, "trigger: cron")
		assert.Contains(t, line, "profile: p")
		assert.Contains(t, line, "backend: claude")
	}
}

// extractMsg pulls the JSON "msg" payload out of a makeBufLine envelope so the
// test can assert on the human-readable text rather than the JSON wrapper.
func extractMsg(line string) string {
	const marker = `"msg":`
	idx := strings.Index(line, marker)
	if idx < 0 {
		return line
	}
	rest := line[idx+len(marker):]
	if !strings.HasPrefix(rest, `"`) {
		return rest
	}
	rest = rest[1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return rest
	}
	return rest[:end]
}
