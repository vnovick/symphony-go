package orchestrator

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
)

// IsRateLimitFailure must classify common vendor 429 / quota error messages
// and ignore unrelated errors so a generic crash never accidentally swaps
// the agent.
func TestIsRateLimitFailure_Classifier(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{"empty", "", false},
		{"generic crash", "panic: runtime error", false},
		{"compile error", "syntax error near unexpected token", false},
		{"anthropic 429 body", "Error: HTTP 429: rate_limit_exceeded", true},
		{"openai quota", "RateLimitError: You exceeded your current quota", true},
		{"plain rate limit phrase", "rate limit reached, please retry later", true},
		{"too many requests", "Too Many Requests", true},
		{"case-insensitive", "RATE_LIMIT_EXCEEDED on us-central-1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsRateLimitFailure(tc.msg))
		})
	}
}

// Gap §5.1 — operator-configurable error-pattern list. Empty fallback uses
// the default list; non-empty replaces it; case-insensitive substring match.
func TestIsRateLimitFailureWithPatterns_OperatorOverride(t *testing.T) {
	// Empty patterns → defaults still hit on "rate_limit_exceeded".
	assert.True(t, IsRateLimitFailureWithPatterns("Error: rate_limit_exceeded", nil))
	// Custom list rejects the default phrasing if not included.
	assert.False(t, IsRateLimitFailureWithPatterns(
		"Error: rate_limit_exceeded",
		[]string{"my-vendor-throttle"},
	))
	// Custom list catches a vendor-specific shape.
	assert.True(t, IsRateLimitFailureWithPatterns(
		"upstream returned ANTHROPIC-OVERLOAD-503",
		[]string{"anthropic-overload"},
	))
	// Empty strings inside the list are skipped (defensive).
	assert.True(t, IsRateLimitFailureWithPatterns(
		"hit my-throttle",
		[]string{"", "my-throttle", ""},
	))
}

// SetRateLimitedAutomations / snapRateLimitedAutomations under -race: same
// invariant as the input-required and pr_opened registries. Concurrent writers
// and readers must not race on the slice header.
func TestRateLimitedAutomations_RaceSafe(t *testing.T) {
	o := &Orchestrator{cfg: &config.Config{}}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 100 {
				o.SetRateLimitedAutomations([]RateLimitedAutomation{
					{ID: "rl", ProfileName: "fallback", SwitchToProfile: "claude-fallback"},
				})
				_ = o.snapRateLimitedAutomations()
			}
		})
	}
	wg.Wait()
	assert.Len(t, o.snapRateLimitedAutomations(), 1)
}

// allowRateLimitSwitch must enforce the rolling-window cap. Once cap is hit,
// further calls must return false until older stamps fall out of the window.
func TestAllowRateLimitSwitch_RollingWindowCap(t *testing.T) {
	o := &Orchestrator{cfg: &config.Config{}}
	o.cfg.Agent.MaxSwitchesPerIssuePerWindow = 2
	o.cfg.Agent.SwitchWindowHours = 6

	now := time.Now()
	require.True(t, o.allowRateLimitSwitch("issue-1", now), "first switch within cap")
	o.recordRateLimitSwitch("issue-1", now)
	require.True(t, o.allowRateLimitSwitch("issue-1", now), "second switch within cap")
	o.recordRateLimitSwitch("issue-1", now)
	assert.False(t, o.allowRateLimitSwitch("issue-1", now),
		"third switch must be rejected once cap is reached within the window")

	// Advance past the window — old stamps fall off, switch becomes available.
	future := now.Add(7 * time.Hour)
	assert.True(t, o.allowRateLimitSwitch("issue-1", future),
		"after the window, the cap should reset")
}

// MaxSwitchesPerIssuePerWindow == 0 means "unlimited" — operator opt-out.
func TestAllowRateLimitSwitch_ZeroMeansUnlimited(t *testing.T) {
	o := &Orchestrator{cfg: &config.Config{}}
	o.cfg.Agent.MaxSwitchesPerIssuePerWindow = 0
	now := time.Now()
	for range 100 {
		require.True(t, o.allowRateLimitSwitch("issue-1", now))
		o.recordRateLimitSwitch("issue-1", now)
	}
}

// Cooldown must mute a per-(issue, profile) tuple until the deadline passes.
// This prevents thrash when an operator's claude profile and codex profile
// are both throttled at the same time.
func TestRateLimitCooldown_MutesPerProfileTuple(t *testing.T) {
	o := &Orchestrator{cfg: &config.Config{}}
	now := time.Now()
	o.setRateLimitCooldown("issue-1|claude-coder", now.Add(30*time.Minute))

	until, muted := o.rateLimitCooldownUntil("issue-1|claude-coder")
	require.True(t, muted)
	assert.True(t, now.Before(until), "muted until 30min ahead")

	// Different (issue, profile) tuple — not muted.
	_, otherMuted := o.rateLimitCooldownUntil("issue-1|codex-coder")
	assert.False(t, otherMuted, "cooldown is per-(issue, profile), not per-issue")
}

// dispatchMatchingRateLimitedAutomations must emit an EventDispatchAutomation
// with the rate-limit-specific trigger context fields populated. We don't run
// the orchestrator's event loop — we drain the events channel directly.
func TestDispatchMatchingRateLimitedAutomations_PopulatesTriggerContext(t *testing.T) {
	o := &Orchestrator{
		cfg:    &config.Config{},
		events: make(chan OrchestratorEvent, 8),
	}
	o.cfg.Agent.MaxSwitchesPerIssuePerWindow = 5
	o.cfg.Agent.SwitchWindowHours = 6
	o.SetRateLimitedAutomations([]RateLimitedAutomation{
		{
			ID:              "switch-when-claude-throttled",
			ProfileName:     "fallback",
			SwitchToProfile: "codex-coder",
			SwitchToBackend: "codex",
			AutoResume:      true,
			Cooldown:        30 * time.Minute,
		},
	})

	state := State{IssueProfiles: map[string]string{"ENG-1": "claude-coder"}}
	issue := domain.Issue{ID: "id1", Identifier: "ENG-1", State: "In Progress"}
	o.dispatchMatchingRateLimitedAutomations(
		context.Background(), &state, issue, time.Now(),
		"claude-coder", "claude", "rate_limit_exceeded", 5,
		180_000, 22_000,
	)

	require.Len(t, o.events, 1, "rule must dispatch exactly one event")
	ev := <-o.events
	require.Equal(t, EventDispatchAutomation, ev.Type)
	require.NotNil(t, ev.Automation)
	assert.Equal(t, config.AutomationTriggerRateLimited, ev.Automation.Trigger.Type)
	assert.Equal(t, "claude-coder", ev.Automation.Trigger.FailedProfile)
	assert.Equal(t, "claude", ev.Automation.Trigger.FailedBackend)
	assert.Equal(t, 180_000, ev.Automation.Trigger.PromptTokensTotal)
	assert.Equal(t, 22_000, ev.Automation.Trigger.CompletionTokensTotal)
	assert.Equal(t, "codex-coder", ev.Automation.Trigger.SwitchedToProfile)
	assert.Equal(t, "codex", ev.Automation.Trigger.SwitchedToBackend)
	assert.True(t, ev.Automation.AutoResume)
}

// AutoResume + SwitchToProfile must override state.IssueProfiles so the
// next dispatch picks up the new profile. SwitchToBackend likewise
// overrides state.IssueBackends. AutoResume=false must NOT override.
func TestDispatchMatchingRateLimitedAutomations_AutoSwitchOverrides(t *testing.T) {
	o := &Orchestrator{
		cfg:    &config.Config{},
		events: make(chan OrchestratorEvent, 8),
	}
	o.cfg.Agent.MaxSwitchesPerIssuePerWindow = 5
	o.cfg.Agent.SwitchWindowHours = 6
	o.SetRateLimitedAutomations([]RateLimitedAutomation{
		{
			ID:              "auto",
			ProfileName:     "fallback",
			SwitchToProfile: "codex-coder",
			SwitchToBackend: "codex",
			AutoResume:      true,
		},
	})

	state := State{IssueProfiles: map[string]string{}, IssueBackends: map[string]string{}}
	issue := domain.Issue{ID: "id1", Identifier: "ENG-1", State: "In Progress"}
	o.dispatchMatchingRateLimitedAutomations(
		context.Background(), &state, issue, time.Now(),
		"claude-coder", "claude", "rate_limit_exceeded", 5, 1, 1,
	)

	assert.Equal(t, "codex-coder", state.IssueProfiles["ENG-1"], "auto-switch must override profile")
	assert.Equal(t, "codex", state.IssueBackends["ENG-1"], "auto-switch must override backend")
}

// Gap §1.3 — the auto-switch must mark the issue in
// AutoSwitchedIdentifiers so the succeeded-exit branch can later revert
// the override without disturbing operator-set overrides.
func TestDispatchMatchingRateLimitedAutomations_MarksAutoSwitched(t *testing.T) {
	o := &Orchestrator{
		cfg:    &config.Config{},
		events: make(chan OrchestratorEvent, 8),
	}
	o.cfg.Agent.MaxSwitchesPerIssuePerWindow = 5
	o.cfg.Agent.SwitchWindowHours = 6
	o.SetRateLimitedAutomations([]RateLimitedAutomation{
		{
			ID:              "auto",
			ProfileName:     "fallback",
			SwitchToProfile: "codex-coder",
			AutoResume:      true,
		},
	})

	state := State{IssueProfiles: map[string]string{}, IssueBackends: map[string]string{}, AutoSwitchedIdentifiers: map[string]struct{}{}}
	o.dispatchMatchingRateLimitedAutomations(
		context.Background(), &state,
		domain.Issue{ID: "id1", Identifier: "ENG-1"}, time.Now(),
		"claude-coder", "claude", "rate_limit_exceeded", 5, 0, 0,
	)
	_, marked := state.AutoSwitchedIdentifiers["ENG-1"]
	assert.True(t, marked, "auto-switch must mark the identifier so the override can be reverted later")
}

func TestDispatchMatchingRateLimitedAutomations_NoOverrideWhenAutoResumeFalse(t *testing.T) {
	o := &Orchestrator{
		cfg:    &config.Config{},
		events: make(chan OrchestratorEvent, 8),
	}
	o.cfg.Agent.MaxSwitchesPerIssuePerWindow = 5
	o.cfg.Agent.SwitchWindowHours = 6
	o.SetRateLimitedAutomations([]RateLimitedAutomation{
		{
			ID:              "manual",
			ProfileName:     "fallback",
			SwitchToProfile: "codex-coder",
			AutoResume:      false,
		},
	})
	state := State{IssueProfiles: map[string]string{}}
	o.dispatchMatchingRateLimitedAutomations(
		context.Background(), &state,
		domain.Issue{ID: "id1", Identifier: "ENG-1"}, time.Now(),
		"claude-coder", "claude", "rate_limit", 5, 0, 0,
	)
	assert.Empty(t, state.IssueProfiles, "auto_resume=false must not silently override")
}

// Gap §5.3 — auto-switched overrides must round-trip through disk so a
// daemon restart re-applies them. Save the overrides, construct a fresh
// orchestrator pointing at the same file, load → state must match.
func TestAutoSwitchedOverrides_PersistRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/auto_switched.json"

	// Step 1: a "writer" orchestrator persists three overrides.
	writer := &Orchestrator{cfg: &config.Config{}}
	writer.SetAutoSwitchedFile(path)
	autoSwitched := map[string]struct{}{
		"ENG-1": {},
		"ENG-2": {},
		"ENG-3": {},
	}
	profiles := map[string]string{
		"ENG-1": "codex-coder",
		"ENG-2": "claude-haiku",
		"ENG-3": "qa-fallback",
	}
	backends := map[string]string{
		"ENG-1": "codex",
		// ENG-2: no backend override
		"ENG-3": "claude",
	}
	writer.saveAutoSwitchedToDisk(autoSwitched, profiles, backends)

	// Step 2: a fresh "reader" orchestrator loads from the same file.
	reader := &Orchestrator{cfg: &config.Config{}}
	reader.SetAutoSwitchedFile(path)
	state := State{
		IssueProfiles:           map[string]string{},
		IssueBackends:           map[string]string{},
		AutoSwitchedIdentifiers: map[string]struct{}{},
	}
	state = reader.loadAutoSwitchedFromDisk(state)

	assert.Equal(t, "codex-coder", state.IssueProfiles["ENG-1"])
	assert.Equal(t, "claude-haiku", state.IssueProfiles["ENG-2"])
	assert.Equal(t, "qa-fallback", state.IssueProfiles["ENG-3"])
	assert.Equal(t, "codex", state.IssueBackends["ENG-1"])
	assert.Empty(t, state.IssueBackends["ENG-2"], "no backend override → not persisted")
	assert.Equal(t, "claude", state.IssueBackends["ENG-3"])
	for id := range autoSwitched {
		_, marked := state.AutoSwitchedIdentifiers[id]
		assert.True(t, marked, "AutoSwitchedIdentifiers must round-trip for %s", id)
	}
}

// Saving an empty map followed by loading must clear the on-disk state
// (the reader sees no overrides). Critical for the cleared-on-success path.
func TestAutoSwitchedOverrides_EmptyMapClearsFile(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/auto_switched.json"
	o := &Orchestrator{cfg: &config.Config{}}
	o.SetAutoSwitchedFile(path)
	o.saveAutoSwitchedToDisk(map[string]struct{}{"ENG-1": {}}, map[string]string{"ENG-1": "x"}, nil)
	o.saveAutoSwitchedToDisk(map[string]struct{}{}, map[string]string{}, nil)

	state := State{
		IssueProfiles:           map[string]string{},
		IssueBackends:           map[string]string{},
		AutoSwitchedIdentifiers: map[string]struct{}{},
	}
	state = o.loadAutoSwitchedFromDisk(state)
	assert.Empty(t, state.AutoSwitchedIdentifiers)
	assert.Empty(t, state.IssueProfiles)
}

// Gap §6.2 — TTL-based revert: overrides whose AutoSwitchedAt is older
// than the configured TTL get cleared from IssueProfiles + IssueBackends
// + AutoSwitchedIdentifiers + AutoSwitchedAt. Newer overrides survive.
// Operator-set overrides (no AutoSwitchedAt entry) are untouched.
func TestRevertExpiredAutoSwitches_DropsAgedOverrides(t *testing.T) {
	now := time.Now()
	state := &State{
		IssueProfiles: map[string]string{
			"OLD-1":      "codex-coder",
			"NEW-1":      "codex-coder",
			"OPERATOR-1": "operator-pinned",
		},
		IssueBackends: map[string]string{
			"OLD-1": "codex",
			"NEW-1": "codex",
		},
		AutoSwitchedIdentifiers: map[string]struct{}{
			"OLD-1": {},
			"NEW-1": {},
		},
		AutoSwitchedAt: map[string]time.Time{
			"OLD-1": now.Add(-25 * time.Hour),
			"NEW-1": now.Add(-30 * time.Minute),
		},
	}

	reverted := RevertExpiredAutoSwitches(state, 24*time.Hour, now)
	assert.Equal(t, 1, reverted, "only the OLD-1 override should be reverted")

	_, oldKept := state.IssueProfiles["OLD-1"]
	_, newKept := state.IssueProfiles["NEW-1"]
	_, operKept := state.IssueProfiles["OPERATOR-1"]
	assert.False(t, oldKept, "expired auto-switch must be reverted")
	assert.True(t, newKept, "fresh auto-switch must survive")
	assert.True(t, operKept, "operator-set override (no AutoSwitchedAt entry) must survive")
}

func TestRevertExpiredAutoSwitches_NoOpWhenTTLZero(t *testing.T) {
	state := &State{
		AutoSwitchedAt: map[string]time.Time{"X": time.Now().Add(-100 * time.Hour)},
	}
	assert.Zero(t, RevertExpiredAutoSwitches(state, 0, time.Now()))
}

// PruneRateLimitedMaps must drop switchHistory entries older than 2*window
// and cooldown entries whose deadline has passed (gap §1.1, §1.2).
func TestPruneRateLimitedMaps_EvictsStale(t *testing.T) {
	o := &Orchestrator{cfg: &config.Config{}}
	o.cfg.Agent.SwitchWindowHours = 6
	now := time.Now()

	// Old switch (12h ago = 2*window) → should be pruned.
	o.recordRateLimitSwitch("issue-old", now.Add(-13*time.Hour))
	// Recent switch → should be kept.
	o.recordRateLimitSwitch("issue-recent", now.Add(-1*time.Hour))
	// Expired cooldown → should be pruned.
	o.setRateLimitCooldown("a|p", now.Add(-1*time.Hour))
	// Active cooldown → should be kept.
	o.setRateLimitCooldown("b|p", now.Add(1*time.Hour))

	o.PruneRateLimitedMaps(now)

	o.switchHistoryMu.Lock()
	_, oldKept := o.switchHistory["issue-old"]
	_, recentKept := o.switchHistory["issue-recent"]
	o.switchHistoryMu.Unlock()
	assert.False(t, oldKept, "stale switchHistory entry should be evicted")
	assert.True(t, recentKept, "recent switchHistory entry should survive")

	o.rateLimitCooldownMu.Lock()
	_, expiredKept := o.rateLimitCooldown["a|p"]
	_, activeKept := o.rateLimitCooldown["b|p"]
	o.rateLimitCooldownMu.Unlock()
	assert.False(t, expiredKept, "expired cooldown entry should be evicted")
	assert.True(t, activeKept, "active cooldown entry should survive")
}

// Rules whose IdentifierRegex doesn't match the issue must not fire.
func TestDispatchMatchingRateLimitedAutomations_IdentifierFilter(t *testing.T) {
	o := &Orchestrator{
		cfg:    &config.Config{},
		events: make(chan OrchestratorEvent, 8),
	}
	o.cfg.Agent.MaxSwitchesPerIssuePerWindow = 5
	o.cfg.Agent.SwitchWindowHours = 6
	o.SetRateLimitedAutomations([]RateLimitedAutomation{
		{
			ID:              "eng-only",
			ProfileName:     "fallback",
			SwitchToProfile: "codex-coder",
			IdentifierRegex: regexp.MustCompile(`^ENG-`),
		},
	})

	state := State{IssueProfiles: map[string]string{}}
	o.dispatchMatchingRateLimitedAutomations(
		context.Background(), &state,
		domain.Issue{Identifier: "BUG-1"}, time.Now(),
		"claude-coder", "claude", "rate_limit", 5, 0, 0,
	)
	assert.Empty(t, o.events, "BUG-1 must not match an ENG-only rule")
}
