package main

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/orchestrator"
	"github.com/vnovick/itervox/internal/schedule"
	"github.com/vnovick/itervox/internal/tracker"
)

type compiledAutomation struct {
	cfg            config.AutomationConfig
	expr           schedule.Expression
	location       *time.Location
	identifierRe   *regexp.Regexp
	inputContextRe *regexp.Regexp
}

type observedAutomationIssue struct {
	State     string
	InBacklog bool
}

type observedAutomationComment struct {
	LatestCommentID  string
	CommentCreatedAt string
}

type automationPollState struct {
	issues          map[string]observedAutomationIssue
	trackerComments map[string]map[string]observedAutomationComment
}

type compiledAutomationSet struct {
	cron          []compiledAutomation
	polledEvents  []compiledAutomation
	inputRequired []orchestrator.InputRequiredAutomation
	runFailed     []orchestrator.RunFailedAutomation
}

func startAutomations(ctx context.Context, cfg *config.Config, tr tracker.Tracker, orch *orchestrator.Orchestrator) {
	compiled := compileAutomations(cfg)
	orch.SetInputRequiredAutomations(compiled.inputRequired)
	orch.SetRunFailedAutomations(compiled.runFailed)
	if len(compiled.cron) == 0 && len(compiled.polledEvents) == 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		lastFired := make(map[string]string, len(compiled.cron))
		pollState := automationPollState{
			issues:          make(map[string]observedAutomationIssue),
			trackerComments: make(map[string]map[string]observedAutomationComment),
		}
		runOnce := func(now time.Time) {
			for _, entry := range compiled.cron {
				if !entry.cfg.Enabled {
					continue
				}
				localNow := now.In(entry.location)
				minuteKey := localNow.Format("2006-01-02T15:04")
				if lastFired[entry.cfg.ID] == minuteKey {
					continue
				}
				if !entry.expr.Matches(localNow) {
					continue
				}
				lastFired[entry.cfg.ID] = minuteKey
				execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				runCronAutomation(execCtx, cfg, tr, orch, entry, now)
				cancel()
			}
			if len(compiled.polledEvents) > 0 {
				execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				pollState = pollAutomationEvents(execCtx, cfg, tr, orch, compiled.polledEvents, pollState, now)
				cancel()
			}
		}

		runOnce(time.Now())
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				runOnce(now)
			}
		}
	}()
}

func compileAutomations(cfg *config.Config) compiledAutomationSet {
	var compiled compiledAutomationSet
	if len(cfg.Automations) == 0 {
		return compiled
	}
	compiled.cron = make([]compiledAutomation, 0, len(cfg.Automations))
	compiled.polledEvents = make([]compiledAutomation, 0, len(cfg.Automations))
	compiled.inputRequired = make([]orchestrator.InputRequiredAutomation, 0, len(cfg.Automations))
	compiled.runFailed = make([]orchestrator.RunFailedAutomation, 0, len(cfg.Automations))
	for _, entry := range cfg.Automations {
		if !entry.Enabled {
			continue
		}
		profile, ok := cfg.Agent.Profiles[entry.Profile]
		if !ok {
			slog.Warn("automation: skipping rule with unknown profile", "automation", entry.ID, "profile", entry.Profile)
			continue
		}
		if !config.ProfileEnabled(profile) {
			slog.Warn("automation: skipping rule with disabled profile", "automation", entry.ID, "profile", entry.Profile)
			continue
		}
		var identifierRe *regexp.Regexp
		if entry.Filter.IdentifierRegex != "" {
			re, err := regexp.Compile(entry.Filter.IdentifierRegex)
			if err != nil {
				slog.Warn("automation: invalid identifier regex", "automation", entry.ID, "regex", entry.Filter.IdentifierRegex, "error", err)
				continue
			}
			identifierRe = re
		}
		switch entry.Trigger.Type {
		case config.AutomationTriggerCron:
			expr, err := schedule.Parse(entry.Trigger.Cron)
			if err != nil {
				slog.Warn("automation: invalid cron expression", "automation", entry.ID, "cron", entry.Trigger.Cron, "error", err)
				continue
			}
			location := time.Local
			if entry.Trigger.Timezone != "" {
				loc, err := time.LoadLocation(entry.Trigger.Timezone)
				if err != nil {
					slog.Warn("automation: invalid timezone", "automation", entry.ID, "timezone", entry.Trigger.Timezone, "error", err)
					continue
				}
				location = loc
			}
			compiled.cron = append(compiled.cron, compiledAutomation{
				cfg:          entry,
				expr:         expr,
				location:     location,
				identifierRe: identifierRe,
			})
		case config.AutomationTriggerInputRequired:
			var inputContextRe *regexp.Regexp
			if entry.Filter.InputContextRegex != "" {
				re, err := regexp.Compile(entry.Filter.InputContextRegex)
				if err != nil {
					slog.Warn("automation: invalid input context regex", "automation", entry.ID, "regex", entry.Filter.InputContextRegex, "error", err)
					continue
				}
				inputContextRe = re
			}
			compiled.inputRequired = append(compiled.inputRequired, orchestrator.InputRequiredAutomation{
				ID:                entry.ID,
				ProfileName:       entry.Profile,
				Instructions:      entry.Instructions,
				MatchMode:         entry.Filter.MatchMode,
				States:            entry.Filter.States,
				LabelsAny:         entry.Filter.LabelsAny,
				IdentifierRegex:   identifierRe,
				InputContextRegex: inputContextRe,
				AutoResume:        entry.Policy.AutoResume,
			})
		case config.AutomationTriggerTrackerComment,
			config.AutomationTriggerIssueEnteredState,
			config.AutomationTriggerIssueMovedBacklog:
			compiled.polledEvents = append(compiled.polledEvents, compiledAutomation{
				cfg:            entry,
				identifierRe:   identifierRe,
				inputContextRe: inputContextReFor(entry),
			})
		case config.AutomationTriggerRunFailed:
			compiled.runFailed = append(compiled.runFailed, orchestrator.RunFailedAutomation{
				ID:              entry.ID,
				ProfileName:     entry.Profile,
				Instructions:    entry.Instructions,
				MatchMode:       entry.Filter.MatchMode,
				States:          entry.Filter.States,
				LabelsAny:       entry.Filter.LabelsAny,
				IdentifierRegex: identifierRe,
			})
		default:
			slog.Warn("automation: unsupported trigger type", "automation", entry.ID, "type", entry.Trigger.Type)
		}
	}
	return compiled
}

func runCronAutomation(
	ctx context.Context,
	cfg *config.Config,
	tr tracker.Tracker,
	orch *orchestrator.Orchestrator,
	entry compiledAutomation,
	now time.Time,
) {
	states := cronAutomationFetchStates(cfg, entry)
	issues, err := tr.FetchIssuesByStates(ctx, states)
	if err != nil {
		slog.Warn("automation: fetch issues failed", "automation", entry.cfg.ID, "error", err)
		return
	}

	snap := orch.Snapshot()
	matches := make([]domain.Issue, 0, len(issues))
	for _, issue := range issues {
		if shouldSkipAutomatedIssue(snap, issue) {
			continue
		}
		if !matchesAutomationFilter(issue, entry, "") {
			continue
		}
		matches = append(matches, issue)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Identifier < matches[j].Identifier
	})

	limit := entry.cfg.Filter.Limit
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	if len(matches) == 0 {
		return
	}

	count := 0
	for _, issue := range matches {
		if orch.DispatchAutomation(issue, orchestrator.AutomationDispatch{
			AutomationID: entry.cfg.ID,
			ProfileName:  entry.cfg.Profile,
			Instructions: entry.cfg.Instructions,
			AutoResume:   entry.cfg.Policy.AutoResume,
			Trigger: orchestrator.AutomationTriggerContext{
				Type:         config.AutomationTriggerCron,
				FiredAt:      now,
				AutomationID: entry.cfg.ID,
				Cron:         entry.cfg.Trigger.Cron,
				Timezone:     entry.cfg.Trigger.Timezone,
				CurrentState: issue.State,
			},
		}) {
			count++
		}
	}
	if count > 0 {
		slog.Info("automation: queued issues", "automation", entry.cfg.ID, "count", count, "profile", entry.cfg.Profile)
	}
}

func shouldSkipAutomatedIssue(state orchestrator.State, issue domain.Issue) bool {
	if _, claimed := state.Claimed[issue.ID]; claimed {
		return true
	}
	if _, paused := state.PausedIdentifiers[issue.Identifier]; paused {
		return true
	}
	if _, waiting := state.InputRequiredIssues[issue.Identifier]; waiting {
		return true
	}
	if _, pending := state.PendingInputResumes[issue.Identifier]; pending {
		return true
	}
	return false
}

func matchesAutomationFilter(issue domain.Issue, entry compiledAutomation, inputContext string) bool {
	checks := make([]bool, 0, 4)
	if entry.identifierRe != nil {
		checks = append(checks, entry.identifierRe.MatchString(issue.Identifier))
	}
	if len(entry.cfg.Filter.States) > 0 {
		checks = append(checks, containsFold(entry.cfg.Filter.States, issue.State))
	}
	if len(entry.cfg.Filter.LabelsAny) > 0 {
		issueLabels := make([]string, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			issueLabels = append(issueLabels, strings.ToLower(label))
		}
		ok := false
		for _, wanted := range entry.cfg.Filter.LabelsAny {
			if slices.Contains(issueLabels, strings.ToLower(wanted)) {
				ok = true
				break
			}
		}
		checks = append(checks, ok)
	}
	if entry.inputContextRe != nil {
		checks = append(checks, entry.inputContextRe.MatchString(inputContext))
	}
	if len(checks) == 0 {
		return true
	}
	if entry.cfg.Filter.MatchMode == config.AutomationFilterMatchAny {
		for _, check := range checks {
			if check {
				return true
			}
		}
		return false
	}
	for _, check := range checks {
		if !check {
			return false
		}
	}
	return true
}

func inputContextReFor(entry config.AutomationConfig) *regexp.Regexp {
	if entry.Filter.InputContextRegex == "" {
		return nil
	}
	re, err := regexp.Compile(entry.Filter.InputContextRegex)
	if err != nil {
		slog.Warn("automation: invalid input context regex", "automation", entry.ID, "regex", entry.Filter.InputContextRegex, "error", err)
		return nil
	}
	return re
}

func cronAutomationFetchStates(cfg *config.Config, entry compiledAutomation) []string {
	if entry.cfg.Filter.MatchMode != config.AutomationFilterMatchAny && len(entry.cfg.Filter.States) > 0 {
		return append([]string{}, entry.cfg.Filter.States...)
	}
	return deduplicateStates(cfg.Tracker.BacklogStates, cfg.Tracker.ActiveStates, entry.cfg.Filter.States, "")
}

func pollAutomationEvents(
	ctx context.Context,
	cfg *config.Config,
	tr tracker.Tracker,
	orch *orchestrator.Orchestrator,
	entries []compiledAutomation,
	prev automationPollState,
	now time.Time,
) automationPollState {
	states := automationPollStates(cfg, entries)
	if len(states) == 0 {
		return prev
	}
	issues, err := tr.FetchIssuesByStates(ctx, states)
	if err != nil {
		slog.Warn("automation: poll-event fetch failed", "error", err)
		return prev
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Identifier < issues[j].Identifier
	})

	next := automationPollState{
		issues:          make(map[string]observedAutomationIssue, len(issues)),
		trackerComments: make(map[string]map[string]observedAutomationComment, len(prev.trackerComments)),
	}

	snap := orch.Snapshot()
	detailCache := make(map[string]*domain.Issue)
	getDetail := func(issue domain.Issue) *domain.Issue {
		if detailed, ok := detailCache[issue.ID]; ok {
			return detailed
		}
		detailed, err := tr.FetchIssueDetail(ctx, issue.ID)
		if err != nil {
			slog.Warn("automation: fetch issue detail failed", "automation_issue", issue.Identifier, "error", err)
			detailCache[issue.ID] = nil
			return nil
		}
		detailCache[issue.ID] = detailed
		return detailed
	}

	for _, issue := range issues {
		next.issues[issue.ID] = observedAutomationIssue{
			State:     issue.State,
			InBacklog: containsFold(cfg.Tracker.BacklogStates, issue.State),
		}
	}
	for _, entry := range entries {
		matches := make([]struct {
			issue   domain.Issue
			trigger orchestrator.AutomationTriggerContext
		}, 0)
		var prevComments map[string]observedAutomationComment
		var nextComments map[string]observedAutomationComment
		if entry.cfg.Trigger.Type == config.AutomationTriggerTrackerComment {
			prevComments = prev.trackerComments[entry.cfg.ID]
			nextComments = make(map[string]observedAutomationComment)
			next.trackerComments[entry.cfg.ID] = nextComments
		}
		for _, issue := range issues {
			if shouldSkipAutomatedIssue(snap, issue) {
				continue
			}
			prevIssue, seenBefore := prev.issues[issue.ID]
			if !seenBefore {
				continue
			}
			switch entry.cfg.Trigger.Type {
			case config.AutomationTriggerIssueEnteredState:
				if strings.EqualFold(prevIssue.State, issue.State) || !strings.EqualFold(issue.State, entry.cfg.Trigger.State) {
					continue
				}
				if !matchesAutomationFilter(issue, entry, "") {
					continue
				}
				matches = append(matches, struct {
					issue   domain.Issue
					trigger orchestrator.AutomationTriggerContext
				}{
					issue: issue,
					trigger: orchestrator.AutomationTriggerContext{
						Type:          config.AutomationTriggerIssueEnteredState,
						FiredAt:       now,
						AutomationID:  entry.cfg.ID,
						TriggerState:  entry.cfg.Trigger.State,
						PreviousState: prevIssue.State,
						CurrentState:  issue.State,
					},
				})
			case config.AutomationTriggerIssueMovedBacklog:
				if prevIssue.InBacklog || !containsFold(cfg.Tracker.BacklogStates, issue.State) {
					continue
				}
				if !matchesAutomationFilter(issue, entry, "") {
					continue
				}
				matches = append(matches, struct {
					issue   domain.Issue
					trigger orchestrator.AutomationTriggerContext
				}{
					issue: issue,
					trigger: orchestrator.AutomationTriggerContext{
						Type:          config.AutomationTriggerIssueMovedBacklog,
						FiredAt:       now,
						AutomationID:  entry.cfg.ID,
						PreviousState: prevIssue.State,
						CurrentState:  issue.State,
					},
				})
			case config.AutomationTriggerTrackerComment:
				if !matchesAutomationFilter(issue, entry, "") {
					continue
				}
				detailed := getDetail(issue)
				if detailed == nil {
					continue
				}
				commentSnapshot := observedAutomationComment{}
				comment, ok := latestAutomationComment(detailed.Comments)
				if ok {
					commentSnapshot = observedAutomationCommentFromComment(comment)
				}
				nextComments[issue.ID] = commentSnapshot
				prevComment, seenBefore := prevComments[issue.ID]
				if !seenBefore {
					continue
				}
				if !hasNewAutomationComment(prevComment, commentSnapshot) {
					continue
				}
				if !ok {
					continue
				}
				if isAutomationManagedComment(comment) {
					continue
				}
				trigger := orchestrator.AutomationTriggerContext{
					Type:              config.AutomationTriggerTrackerComment,
					FiredAt:           now,
					AutomationID:      entry.cfg.ID,
					CurrentState:      issue.State,
					CommentID:         comment.ID,
					CommentBody:       comment.Body,
					CommentAuthorID:   comment.AuthorID,
					CommentAuthorName: comment.AuthorName,
				}
				if comment.CreatedAt != nil {
					trigger.CommentCreatedAt = comment.CreatedAt.Format(time.RFC3339)
				}
				matches = append(matches, struct {
					issue   domain.Issue
					trigger orchestrator.AutomationTriggerContext
				}{issue: issue, trigger: trigger})
			}
		}
		limit := entry.cfg.Filter.Limit
		if limit > 0 && len(matches) > limit {
			matches = matches[:limit]
		}
		for _, match := range matches {
			_ = orch.DispatchAutomation(match.issue, orchestrator.AutomationDispatch{
				AutomationID: entry.cfg.ID,
				ProfileName:  entry.cfg.Profile,
				Instructions: entry.cfg.Instructions,
				Trigger:      match.trigger,
			})
		}
	}

	return next
}

func automationPollStates(cfg *config.Config, entries []compiledAutomation) []string {
	states := deduplicateStates(cfg.Tracker.BacklogStates, cfg.Tracker.ActiveStates, cfg.Tracker.TerminalStates, cfg.Tracker.CompletionState)
	for _, entry := range entries {
		states = append(states, entry.cfg.Filter.States...)
		if entry.cfg.Trigger.State != "" {
			states = append(states, entry.cfg.Trigger.State)
		}
	}
	seen := make(map[string]struct{}, len(states))
	out := make([]string, 0, len(states))
	for _, state := range states {
		if state == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(state)]; ok {
			continue
		}
		seen[strings.ToLower(state)] = struct{}{}
		out = append(out, state)
	}
	return out
}

func latestAutomationComment(comments []domain.Comment) (domain.Comment, bool) {
	if len(comments) == 0 {
		return domain.Comment{}, false
	}
	return comments[len(comments)-1], true
}

func observedAutomationCommentFromComment(comment domain.Comment) observedAutomationComment {
	snapshot := observedAutomationComment{LatestCommentID: comment.ID}
	if comment.CreatedAt != nil {
		snapshot.CommentCreatedAt = comment.CreatedAt.Format(time.RFC3339)
	}
	return snapshot
}

func hasNewAutomationComment(prev, current observedAutomationComment) bool {
	if current.LatestCommentID == "" && current.CommentCreatedAt == "" {
		return false
	}
	if prev.LatestCommentID == "" && prev.CommentCreatedAt == "" {
		return true
	}
	if current.LatestCommentID != "" {
		return !strings.EqualFold(prev.LatestCommentID, current.LatestCommentID)
	}
	return current.CommentCreatedAt != prev.CommentCreatedAt
}

func markAutomationComment(body string) string {
	return tracker.MarkManagedComment(body)
}

func isAutomationManagedComment(comment domain.Comment) bool {
	return tracker.IsManagedComment(comment) ||
		strings.HasPrefix(comment.Body, "🤖 **Agent needs your input**") ||
		strings.EqualFold(strings.TrimSpace(comment.AuthorName), "Itervox")
}

func containsFold(values []string, target string) bool {
	target = strings.ToLower(target)
	for _, value := range values {
		if strings.ToLower(value) == target {
			return true
		}
	}
	return false
}
