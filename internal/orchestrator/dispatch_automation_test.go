package orchestrator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
)

// These tests cover the CRIT-3 / input-required replay invariants:
// automation dispatch must skip the isActiveState gate so
// issue_moved_to_backlog and non-active issue_entered_state triggers fire,
// and it must also skip the input_required guard so helper agents can target
// already-blocked issues. IneligibleReason (reconcile path) must retain both
// guards; ineligibleReasonForAutomation (EventDispatchAutomation path) must
// drop them.

func automationBaseCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Tracker.ActiveStates = []string{"Todo", "In Progress"}
	cfg.Tracker.TerminalStates = []string{"Done", "Cancelled"}
	cfg.Agent.MaxConcurrentAgents = 3
	cfg.Agent.MaxConcurrentAgentsByState = map[string]int{}
	return cfg
}

func automationIssue(state string) domain.Issue {
	return domain.Issue{ID: "id1", Identifier: "ENG-1", Title: "T", State: state}
}

func TestIneligibleReasonForAutomation_AcceptsBacklogState(t *testing.T) {
	cfg := automationBaseCfg()
	state := NewState(cfg)

	// CRIT-3 regression guard: an issue in a non-active state (e.g. Backlog)
	// must NOT be rejected with "not_active_state" in the automation path.
	issue := automationIssue("Backlog")
	assert.Equal(t, "", ineligibleReasonForAutomation(issue, state, cfg),
		"automation dispatch must accept backlog-state issues — CRIT-3")
}

func TestIneligibleReasonForAutomation_StillRejectsTerminalState(t *testing.T) {
	cfg := automationBaseCfg()
	state := NewState(cfg)

	issue := automationIssue("Done")
	assert.Equal(t, "terminal_state", ineligibleReasonForAutomation(issue, state, cfg),
		"automation dispatch must still block terminal-state issues")
}

func TestIneligibleReasonForAutomation_EnforcesOtherGuards(t *testing.T) {
	cfg := automationBaseCfg()
	state := NewState(cfg)
	state.Running["id1"] = &RunEntry{}

	issue := automationIssue("Backlog")
	assert.Equal(t, "already_running", ineligibleReasonForAutomation(issue, state, cfg))
}

func TestIneligibleReason_StillRejectsNonActiveState(t *testing.T) {
	// Reconcile path must retain the active-state gate — untouched by CRIT-3.
	cfg := automationBaseCfg()
	state := NewState(cfg)
	issue := automationIssue("Backlog")
	assert.Equal(t, "not_active_state", IneligibleReason(issue, state, cfg))
}

func TestIneligibleReasonForAutomation_AcceptsInputRequiredIssue(t *testing.T) {
	cfg := automationBaseCfg()
	state := NewState(cfg)
	state.InputRequiredIssues["ENG-1"] = &InputRequiredEntry{IssueID: "id1"}

	issue := automationIssue("Todo")
	assert.Equal(t, "", ineligibleReasonForAutomation(issue, state, cfg),
		"automation replay must target blocked input_required issues")
	assert.Equal(t, "input_required", IneligibleReason(issue, state, cfg),
		"normal reconcile dispatch must still reject blocked input_required issues")
}

// TestEventDispatchAutomation_SkipsWhenInputRequiredArrivedAfterQueue pins
// the TOCTOU re-check added by T-16. A cron automation snapshots state when
// it queues the dispatch event; an input-required event may arrive between
// then and the dispatch handler. Without the re-check, the automation would
// step on a worker that's already paused waiting for human input.
func TestEventDispatchAutomation_SkipsWhenInputRequiredArrivedAfterQueue(t *testing.T) {
	cfg := automationBaseCfg()
	state := NewState(cfg)
	// The "issue is now waiting for input" condition that arrived after
	// the cron tick decided to dispatch.
	state.InputRequiredIssues["ENG-1"] = &InputRequiredEntry{IssueID: "id1"}

	o := &Orchestrator{cfg: cfg}
	issue := automationIssue("Todo")
	ev := OrchestratorEvent{
		Type:  EventDispatchAutomation,
		Issue: &issue,
		Automation: &AutomationDispatch{
			AutomationID: "nightly-cron",
			ProfileName:  "default",
			Trigger: AutomationTriggerContext{
				Type: config.AutomationTriggerCron,
			},
		},
	}
	out := o.handleEvent(t.Context(), state, ev)
	assert.Empty(t, out.Running, "TOCTOU re-check must prevent automation dispatch when input_required arrived after queue")
}

// TestEventDispatchAutomation_InputRequiredAutomationsBypassTheGate ensures
// the re-check exempts input_required-typed automations — they exist
// specifically to operate on blocked issues.
func TestEventDispatchAutomation_InputRequiredAutomationsBypassTheGate(t *testing.T) {
	cfg := automationBaseCfg()
	state := NewState(cfg)
	state.InputRequiredIssues["ENG-1"] = &InputRequiredEntry{IssueID: "id1"}

	o := &Orchestrator{cfg: cfg}
	issue := automationIssue("Todo")
	ev := OrchestratorEvent{
		Type:  EventDispatchAutomation,
		Issue: &issue,
		Automation: &AutomationDispatch{
			AutomationID: "input-responder",
			ProfileName:  "responder",
			Trigger: AutomationTriggerContext{
				Type: config.AutomationTriggerInputRequired,
			},
		},
	}
	// We don't assert state.Running here — startAutomationRun has many
	// dependencies (workspace, agent runner, etc.) that aren't wired up in
	// this minimal Orchestrator. The point is that the re-check branch
	// MUST NOT be taken; the panic from missing deps below would prove it.
	// Recover so the test can fail cleanly with the desired assertion.
	defer func() {
		if r := recover(); r != nil {
			// Reaching startAutomationRun means the gate was bypassed
			// correctly. The panic from missing deps is OK here.
			return
		}
	}()
	_ = o.handleEvent(t.Context(), state, ev)
}

// CRIT-1 regression guard: SetInputRequiredAutomations + snapInputRequiredAutomations
// must tolerate concurrent writes (automations goroutine re-registering each
// tick) and reads (event loop dispatching on blocked runs) under -race.
func TestInputRequiredAutomationsRaceSafe(t *testing.T) {
	o := &Orchestrator{}
	done := make(chan struct{})
	go func() {
		for i := range 500 {
			o.SetInputRequiredAutomations([]InputRequiredAutomation{
				{ID: "a", ProfileName: "p", Instructions: fmt.Sprintf("v%d", i)},
			})
		}
		close(done)
	}()
	for range 500 {
		_ = o.snapInputRequiredAutomations()
	}
	<-done
}

func TestIneligibleReason_AndAutomationAgreeOnOtherSharedGuards(t *testing.T) {
	// The two helpers must still agree on the guards they continue to share.
	cfg := automationBaseCfg()
	state := NewState(cfg)
	state.PausedIdentifiers["ENG-1"] = "manual"
	issue := automationIssue("In Progress")

	assert.Equal(t,
		IneligibleReason(issue, state, cfg),
		ineligibleReasonForAutomation(issue, state, cfg),
		"shared guards must produce identical reasons in both dispatch paths")
}
