package main

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/agent/agenttest"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/orchestrator"
)

func TestCompileAutomations_SplitsCronAndInputRequired(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Profiles: map[string]config.AgentProfile{
				"qa":              {Command: "claude"},
				"input-responder": {Command: "claude"},
				"reviewer":        {Command: "claude"},
			},
		},
		Automations: []config.AutomationConfig{
			{
				ID:      "qa-ready",
				Enabled: true,
				Profile: "qa",
				Trigger: config.AutomationTriggerConfig{
					Type:     "cron",
					Cron:     "0 */2 * * *",
					Timezone: "UTC",
				},
			},
			{
				ID:      "input-responder",
				Enabled: true,
				Profile: "input-responder",
				Trigger: config.AutomationTriggerConfig{
					Type: "input_required",
				},
				Filter: config.AutomationFilterConfig{
					InputContextRegex: "continue|branch",
				},
			},
			{
				ID:      "state-entry",
				Enabled: true,
				Profile: "qa",
				Trigger: config.AutomationTriggerConfig{
					Type:  "issue_entered_state",
					State: "Ready for QA",
				},
			},
			{
				ID:      "comment-watch",
				Enabled: true,
				Profile: "reviewer",
				Trigger: config.AutomationTriggerConfig{
					Type: "tracker_comment_added",
				},
			},
			{
				ID:      "failed-run",
				Enabled: true,
				Profile: "reviewer",
				Trigger: config.AutomationTriggerConfig{
					Type: "run_failed",
				},
			},
		},
	}

	compiled := compileAutomations(cfg)
	require.Len(t, compiled.cron, 1)
	require.Len(t, compiled.inputRequired, 1)
	require.Len(t, compiled.polledEvents, 2)
	require.Len(t, compiled.runFailed, 1)
	assert.Equal(t, "qa-ready", compiled.cron[0].cfg.ID)
	assert.Equal(t, "input-responder", compiled.inputRequired[0].ID)
	assert.Equal(t, "input-responder", compiled.inputRequired[0].ProfileName)
}

func TestMatchesAutomationFilter_ChecksLabelsAndInputContext(t *testing.T) {
	entry := compiledAutomation{
		cfg: config.AutomationConfig{
			ID: "input-responder",
			Filter: config.AutomationFilterConfig{
				MatchMode:         "all",
				LabelsAny:         []string{"qa", "triage"},
				InputContextRegex: "continue|branch",
			},
		},
		inputContextRe: regexp.MustCompile("continue|branch"),
	}

	issue := domain.Issue{
		Identifier: "ENG-42",
		Labels:     []string{"triage"},
	}

	assert.True(t, matchesAutomationFilter(issue, entry, "Continue with the existing branch"))
	assert.False(t, matchesAutomationFilter(issue, entry, "Need approval for production migration"))
	assert.False(t, matchesAutomationFilter(domain.Issue{Identifier: "ENG-42", Labels: []string{"docs"}}, entry, "Continue with the existing branch"))
}

func TestMatchesAutomationFilter_AnyMatchMode(t *testing.T) {
	entry := compiledAutomation{
		cfg: config.AutomationConfig{
			ID: "comment-watch",
			Filter: config.AutomationFilterConfig{
				MatchMode:       "any",
				LabelsAny:       []string{"triage"},
				IdentifierRegex: "^ENG-42$",
			},
		},
		identifierRe: regexp.MustCompile("^ENG-42$"),
	}

	assert.True(t, matchesAutomationFilter(domain.Issue{Identifier: "ENG-42"}, entry, ""))
	assert.True(t, matchesAutomationFilter(domain.Issue{Identifier: "ENG-99", Labels: []string{"triage"}}, entry, ""))
	assert.False(t, matchesAutomationFilter(domain.Issue{Identifier: "ENG-99", Labels: []string{"docs"}}, entry, ""))
}

func TestCronAutomationFetchStates_IncludesExplicitStatesInAnyMode(t *testing.T) {
	cfg := &config.Config{
		Tracker: config.TrackerConfig{
			BacklogStates: []string{"Backlog"},
			ActiveStates:  []string{"Todo", "In Progress"},
		},
	}
	entry := compiledAutomation{
		cfg: config.AutomationConfig{
			Filter: config.AutomationFilterConfig{
				MatchMode: config.AutomationFilterMatchAny,
				States:    []string{"Needs Clarification", "Ready for QA"},
			},
		},
	}

	states := cronAutomationFetchStates(cfg, entry)

	assert.ElementsMatch(t, []string{"Backlog", "Todo", "In Progress", "Needs Clarification", "Ready for QA"}, states)
}

func TestAutomationPollStates_IncludesFilterStates(t *testing.T) {
	cfg := &config.Config{
		Tracker: config.TrackerConfig{
			BacklogStates:   []string{"Backlog"},
			ActiveStates:    []string{"Todo", "In Progress"},
			TerminalStates:  []string{"Done"},
			CompletionState: "Done",
		},
	}
	entries := []compiledAutomation{
		{
			cfg: config.AutomationConfig{
				ID: "comment-watch",
				Trigger: config.AutomationTriggerConfig{
					Type: config.AutomationTriggerTrackerComment,
				},
				Filter: config.AutomationFilterConfig{
					MatchMode: config.AutomationFilterMatchAny,
					States:    []string{"Needs Clarification"},
				},
			},
		},
		{
			cfg: config.AutomationConfig{
				ID: "state-entry",
				Trigger: config.AutomationTriggerConfig{
					Type:  config.AutomationTriggerIssueEnteredState,
					State: "Ready for QA",
				},
				Filter: config.AutomationFilterConfig{
					States: []string{"Ready for QA", "Needs Clarification"},
				},
			},
		},
	}

	states := automationPollStates(cfg, entries)

	assert.ElementsMatch(t, []string{"Backlog", "Todo", "In Progress", "Done", "Needs Clarification", "Ready for QA"}, states)
}

func TestAutomationManagedCommentMarkers(t *testing.T) {
	body := "QA failed. Moving back to Todo."
	marked := markAutomationComment(body)

	assert.Contains(t, marked, body)
	assert.True(t, isAutomationManagedComment(domain.Comment{Body: marked}))
	assert.True(t, isAutomationManagedComment(domain.Comment{AuthorName: "Itervox"}))
	assert.False(t, isAutomationManagedComment(domain.Comment{Body: body, AuthorName: "alice"}))
}

type pollTracker struct {
	mu          sync.Mutex
	poll        int
	issuesByRun [][]domain.Issue
	detailByRun []map[string]domain.Issue
}

func (t *pollTracker) FetchCandidateIssues(context.Context) ([]domain.Issue, error) {
	return nil, nil
}

func (t *pollTracker) FetchIssuesByStates(context.Context, []string) ([]domain.Issue, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.issuesByRun) == 0 {
		return nil, nil
	}
	index := t.poll
	if index >= len(t.issuesByRun) {
		index = len(t.issuesByRun) - 1
	}
	t.poll++
	return append([]domain.Issue(nil), t.issuesByRun[index]...), nil
}

func (t *pollTracker) FetchIssueStatesByIDs(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}

func (t *pollTracker) CreateComment(context.Context, string, string) (*domain.Comment, error) {
	return nil, nil
}

func (t *pollTracker) CreateIssue(context.Context, string, string, string, string) (*domain.Issue, error) {
	return nil, nil
}

func (t *pollTracker) UpdateIssueState(context.Context, string, string) error {
	return nil
}

func (t *pollTracker) FetchIssueDetail(_ context.Context, issueID string) (*domain.Issue, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.detailByRun) == 0 {
		return nil, fmt.Errorf("issue %s not found", issueID)
	}
	index := t.poll - 1
	if index < 0 {
		index = 0
	}
	if index >= len(t.detailByRun) {
		index = len(t.detailByRun) - 1
	}
	issue, ok := t.detailByRun[index][issueID]
	if !ok {
		return nil, fmt.Errorf("issue %s not found", issueID)
	}
	cp := issue
	return &cp, nil
}

func (t *pollTracker) FetchIssueByIdentifier(_ context.Context, identifier string) (*domain.Issue, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, batch := range t.issuesByRun {
		for _, issue := range batch {
			if issue.Identifier == identifier {
				cp := issue
				return &cp, nil
			}
		}
	}
	return nil, fmt.Errorf("issue %s not found", identifier)
}

func (t *pollTracker) SetIssueBranch(context.Context, string, string) error {
	return nil
}

type doneRunner struct {
	agent.Runner
	done chan struct{}
}

func (r *doneRunner) RunTurn(ctx context.Context, log agent.Logger, onProgress func(agent.TurnResult), sessionID *string, prompt, workspacePath, command, workerHost, logDir string, readTimeoutMs, turnTimeoutMs int) (agent.TurnResult, error) {
	res, err := r.Runner.RunTurn(ctx, log, onProgress, sessionID, prompt, workspacePath, command, workerHost, logDir, readTimeoutMs, turnTimeoutMs)
	select {
	case r.done <- struct{}{}:
	default:
	}
	return res, err
}

func TestPollAutomationEvents_TrackerCommentRequiresNewEligibleComment(t *testing.T) {
	cfg := &config.Config{
		Polling: config.PollingConfig{IntervalMs: 50},
		Tracker: config.TrackerConfig{
			BacklogStates:   []string{"Backlog"},
			ActiveStates:    []string{"Todo"},
			TerminalStates:  []string{"Done"},
			CompletionState: "Done",
		},
		Agent: config.AgentConfig{
			Command:             "claude",
			MaxConcurrentAgents: 1,
			Profiles: map[string]config.AgentProfile{
				"pm": {Command: "claude"},
			},
			TurnTimeoutMs: 60000,
			ReadTimeoutMs: 30000,
		},
	}
	entries := []compiledAutomation{
		{
			cfg: config.AutomationConfig{
				ID:      "comment-watch",
				Enabled: true,
				Profile: "pm",
				Trigger: config.AutomationTriggerConfig{
					Type: config.AutomationTriggerTrackerComment,
				},
				Filter: config.AutomationFilterConfig{
					LabelsAny: []string{"triage"},
				},
			},
		},
	}

	comment1 := domain.Comment{ID: "c1", Body: "old comment", AuthorName: "alice"}
	comment2 := domain.Comment{ID: "c2", Body: "new comment", AuthorName: "alice"}
	comment2Edited := domain.Comment{ID: "c2", Body: "edited new comment", AuthorName: "alice"}
	issueNoMatch := domain.Issue{ID: "id-1", Identifier: "ENG-1", Title: "Triage me", State: "Todo", Labels: []string{"docs"}}
	issueMatch := domain.Issue{ID: "id-1", Identifier: "ENG-1", Title: "Triage me", State: "Todo", Labels: []string{"triage"}}

	tr := &pollTracker{
		issuesByRun: [][]domain.Issue{
			{issueNoMatch},
			{issueMatch},
			{issueMatch},
			{issueMatch},
		},
		detailByRun: []map[string]domain.Issue{
			{"id-1": {ID: "id-1", Identifier: "ENG-1", Title: "Triage me", State: "Todo", Labels: []string{"docs"}, Comments: []domain.Comment{comment1}}},
			{"id-1": {ID: "id-1", Identifier: "ENG-1", Title: "Triage me", State: "Todo", Labels: []string{"triage"}, Comments: []domain.Comment{comment1}}},
			{"id-1": {ID: "id-1", Identifier: "ENG-1", Title: "Triage me", State: "Todo", Labels: []string{"triage"}, Comments: []domain.Comment{comment2}}},
			{"id-1": {ID: "id-1", Identifier: "ENG-1", Title: "Triage me", State: "Todo", Labels: []string{"triage"}, Comments: []domain.Comment{comment2Edited}}},
		},
	}

	runner := &doneRunner{
		Runner: agenttest.NewFakeRunner([]agent.StreamEvent{
			{Type: "system", SessionID: "s1"},
			{Type: "result", SessionID: "s1"},
		}),
		done: make(chan struct{}, 4),
	}
	orch := orchestrator.New(cfg, tr, runner, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go orch.Run(ctx) //nolint:errcheck
	time.Sleep(20 * time.Millisecond)

	state := automationPollState{issues: make(map[string]observedAutomationIssue)}
	state = pollAutomationEvents(ctx, cfg, tr, orch, entries, state, time.Now())
	state = pollAutomationEvents(ctx, cfg, tr, orch, entries, state, time.Now().Add(time.Minute))

	select {
	case <-runner.done:
		t.Fatal("stale comment should not dispatch when issue only became newly eligible")
	default:
	}

	state = pollAutomationEvents(ctx, cfg, tr, orch, entries, state, time.Now().Add(2*time.Minute))
	require.Eventually(t, func() bool {
		return len(orch.RunHistory()) == 1
	}, 3*time.Second, 25*time.Millisecond, "expected new comment id to dispatch automation")

	pollAutomationEvents(ctx, cfg, tr, orch, entries, state, time.Now().Add(3*time.Minute))
	time.Sleep(250 * time.Millisecond)
	assert.Len(t, orch.RunHistory(), 1, "editing the latest comment body with the same id should not dispatch again")
}

func TestPollAutomationEvents_DropsIssueSnapshotsWhenIssueIsAbsent(t *testing.T) {
	entries := []compiledAutomation{
		{
			cfg: config.AutomationConfig{
				ID:      "qa-ready",
				Enabled: true,
				Profile: "qa",
				Trigger: config.AutomationTriggerConfig{
					Type:  config.AutomationTriggerIssueEnteredState,
					State: "Ready for QA",
				},
			},
		},
	}
	cfg := &config.Config{
		Tracker: config.TrackerConfig{
			ActiveStates:    []string{"Todo"},
			TerminalStates:  []string{"Done"},
			CompletionState: "Done",
		},
		Agent: config.AgentConfig{
			Profiles: map[string]config.AgentProfile{
				"qa": {Command: "claude"},
			},
		},
		Automations: []config.AutomationConfig{
			entries[0].cfg,
		},
	}
	todoIssue := domain.Issue{ID: "id-1", Identifier: "ENG-1", Title: "QA me", State: "Todo"}
	readyIssue := domain.Issue{ID: "id-1", Identifier: "ENG-1", Title: "QA me", State: "Ready for QA"}

	tr := &pollTracker{
		issuesByRun: [][]domain.Issue{
			{todoIssue},
			{},
			{readyIssue},
			{todoIssue},
			{readyIssue},
		},
		detailByRun: []map[string]domain.Issue{
			{"id-1": todoIssue},
			{},
			{"id-1": readyIssue},
			{"id-1": todoIssue},
			{"id-1": readyIssue},
		},
	}

	runner := &doneRunner{
		Runner: agenttest.NewFakeRunner([]agent.StreamEvent{
			{Type: "system", SessionID: "s1"},
			{Type: "result", SessionID: "s1"},
		}),
		done: make(chan struct{}, 4),
	}
	orch := orchestrator.New(cfg, tr, runner, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go orch.Run(ctx) //nolint:errcheck
	time.Sleep(20 * time.Millisecond)

	state := automationPollState{issues: make(map[string]observedAutomationIssue)}
	state = pollAutomationEvents(ctx, cfg, tr, orch, entries, state, time.Now())
	require.Contains(t, state.issues, "id-1")

	state = pollAutomationEvents(ctx, cfg, tr, orch, entries, state, time.Now().Add(time.Minute))
	assert.Empty(t, state.issues, "issues absent from the current poll should not keep stale snapshots")
}
