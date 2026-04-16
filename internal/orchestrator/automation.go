package orchestrator

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vnovick/itervox/internal/agent"
	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/domain"
)

type AutomationTriggerContext struct {
	Type              string
	FiredAt           time.Time
	AutomationID      string
	Cron              string
	Timezone          string
	TriggerState      string
	InputContext      string
	BlockedProfile    string
	BlockedBackend    string
	PreviousState     string
	CurrentState      string
	CommentID         string
	CommentBody       string
	CommentAuthorID   string
	CommentAuthorName string
	CommentCreatedAt  string
	ErrorMessage      string
	WillRetry         bool
	RetryAttempt      int
	RetryBackoffMs    int
}

type AutomationDispatch struct {
	AutomationID string
	ProfileName  string
	Instructions string
	Trigger      AutomationTriggerContext
	AutoResume   bool
}

type InputRequiredAutomation struct {
	ID                string
	ProfileName       string
	Instructions      string
	MatchMode         string
	States            []string
	LabelsAny         []string
	IdentifierRegex   *regexp.Regexp
	InputContextRegex *regexp.Regexp
	AutoResume        bool
}

type RunFailedAutomation struct {
	ID              string
	ProfileName     string
	Instructions    string
	MatchMode       string
	States          []string
	LabelsAny       []string
	IdentifierRegex *regexp.Regexp
}

// SetInputRequiredAutomations installs the compiled input-required automation
// rules. Must be called before Run; config reload restarts the process.
func (o *Orchestrator) SetInputRequiredAutomations(automations []InputRequiredAutomation) {
	o.inputRequiredAutomations = append([]InputRequiredAutomation(nil), automations...)
}

// SetRunFailedAutomations installs the compiled terminal-failure automation
// rules. Must be called before Run; config reload restarts the process.
func (o *Orchestrator) SetRunFailedAutomations(automations []RunFailedAutomation) {
	o.runFailedAutomations = append([]RunFailedAutomation(nil), automations...)
}

// DispatchAutomation queues an automation worker through the event loop.
// Safe to call from any goroutine.
func (o *Orchestrator) DispatchAutomation(issue domain.Issue, automation AutomationDispatch) bool {
	select {
	case o.events <- OrchestratorEvent{
		Type:       EventDispatchAutomation,
		Issue:      &issue,
		Automation: &automation,
	}:
		return true
	default:
		slog.Warn("orchestrator: automation dispatch event channel full", "identifier", issue.Identifier, "automation", automation.AutomationID)
		return false
	}
}

func (o *Orchestrator) dispatchMatchingInputRequiredAutomations(
	ctx context.Context,
	state *State,
	issue domain.Issue,
	entry *InputRequiredEntry,
	now time.Time,
) {
	if entry == nil || len(o.inputRequiredAutomations) == 0 {
		return
	}
	for _, automation := range o.inputRequiredAutomations {
		if !matchesInputRequiredAutomation(issue, automation, entry.Context) {
			continue
		}
		o.startAutomationRun(ctx, state, issue, now, AutomationDispatch{
			AutomationID: automation.ID,
			ProfileName:  automation.ProfileName,
			Instructions: automation.Instructions,
			AutoResume:   automation.AutoResume,
			Trigger: AutomationTriggerContext{
				Type:           config.AutomationTriggerInputRequired,
				FiredAt:        now,
				AutomationID:   automation.ID,
				InputContext:   entry.Context,
				BlockedProfile: entry.ProfileName,
				BlockedBackend: entry.Backend,
			},
		})
	}
}

func matchesInputRequiredAutomation(issue domain.Issue, automation InputRequiredAutomation, inputContext string) bool {
	return matchesAutomationFilter(
		issue,
		automation.MatchMode,
		automation.States,
		automation.LabelsAny,
		automation.IdentifierRegex,
		automation.InputContextRegex,
		inputContext,
	)
}

func (o *Orchestrator) dispatchMatchingRunFailedAutomations(
	ctx context.Context,
	state *State,
	issue domain.Issue,
	now time.Time,
	errorMessage string,
	attempt int,
) {
	if len(o.runFailedAutomations) == 0 {
		return
	}
	for _, automation := range o.runFailedAutomations {
		if !matchesAutomationFilter(
			issue,
			automation.MatchMode,
			automation.States,
			automation.LabelsAny,
			automation.IdentifierRegex,
			nil,
			"",
		) {
			continue
		}
		o.startAutomationRun(ctx, state, issue, now, AutomationDispatch{
			AutomationID: automation.ID,
			ProfileName:  automation.ProfileName,
			Instructions: automation.Instructions,
			Trigger: AutomationTriggerContext{
				Type:         config.AutomationTriggerRunFailed,
				FiredAt:      now,
				AutomationID: automation.ID,
				CurrentState: issue.State,
				ErrorMessage: errorMessage,
				WillRetry:    false,
				RetryAttempt: attempt,
			},
		})
	}
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

func matchesAutomationFilter(
	issue domain.Issue,
	matchMode string,
	states []string,
	labelsAny []string,
	identifierRegex *regexp.Regexp,
	inputContextRegex *regexp.Regexp,
	inputContext string,
) bool {
	checks := make([]bool, 0, 4)
	if identifierRegex != nil {
		checks = append(checks, identifierRegex.MatchString(issue.Identifier))
	}
	if len(states) > 0 {
		checks = append(checks, containsFold(states, issue.State))
	}
	if len(labelsAny) > 0 {
		labelMatch := false
		for _, wanted := range labelsAny {
			if containsFold(issue.Labels, wanted) {
				labelMatch = true
				break
			}
		}
		checks = append(checks, labelMatch)
	}
	if inputContextRegex != nil {
		checks = append(checks, inputContextRegex.MatchString(inputContext))
	}
	if len(checks) == 0 {
		return true
	}
	if matchMode == config.AutomationFilterMatchAny {
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

func (o *Orchestrator) startAutomationRun(
	ctx context.Context,
	state *State,
	issue domain.Issue,
	now time.Time,
	automation AutomationDispatch,
) {
	if automation.ProfileName == "" {
		return
	}
	if _, running := state.Running[issue.ID]; running {
		return
	}
	if _, claimed := state.Claimed[issue.ID]; claimed {
		return
	}
	if AvailableSlots(*state) <= 0 {
		return
	}

	o.cfgMu.RLock()
	profile, ok := o.cfg.Agent.Profiles[automation.ProfileName]
	defaultCommand := o.cfg.Agent.Command
	defaultBackend := o.cfg.Agent.Backend
	hosts := append([]string{}, o.cfg.Agent.SSHHosts...)
	dispatchStrategy := o.cfg.Agent.DispatchStrategy
	o.cfgMu.RUnlock()

	if !ok {
		slog.Warn("orchestrator: automation profile not found", "identifier", issue.Identifier, "profile", automation.ProfileName, "automation", automation.AutomationID)
		return
	}
	if !config.ProfileEnabled(profile) {
		slog.Warn("orchestrator: automation profile disabled", "identifier", issue.Identifier, "profile", automation.ProfileName, "automation", automation.AutomationID)
		return
	}

	workerCtx, workerCancel := context.WithCancel(ctx)
	workerHost := o.selectWorkerHost(hosts, dispatchStrategy, *state)

	agentCommand := defaultCommand
	backend := agent.BackendFromCommand(agentCommand)
	if defaultBackend != "" {
		backend = defaultBackend
	}
	runnerCommand := agentCommand
	if profile.Command != "" {
		agentCommand = profile.Command
		runnerCommand = agentCommand
		backend = agent.BackendFromCommand(agentCommand)
	}
	if profile.Backend != "" {
		backend = profile.Backend
		runnerCommand = agent.CommandWithBackendHint(agentCommand, profile.Backend)
	}

	if o.DryRun {
		workerCancel()
		slog.Info("orchestrator: [DRY-RUN] would dispatch automation",
			"identifier", issue.Identifier,
			"automation", automation.AutomationID,
			"profile", automation.ProfileName,
			"worker_host", workerHost,
			"backend", backend)
		state.Claimed[issue.ID] = struct{}{}
		return
	}

	state.Claimed[issue.ID] = struct{}{}
	attempt := 0
	state.Running[issue.ID] = &RunEntry{
		Issue:        issue,
		WorkerHost:   workerHost,
		Backend:      backend,
		Kind:         "automation",
		StartedAt:    now,
		RetryAttempt: &attempt,
		WorkerCancel: workerCancel,
	}

	o.workerCancelsMu.Lock()
	o.workerCancels[issue.Identifier] = workerCancel
	o.workerCancelsMu.Unlock()

	slog.Info("orchestrator: dispatching automation worker",
		"identifier", issue.Identifier,
		"automation", automation.AutomationID,
		"profile", automation.ProfileName,
		"backend", backend,
	)

	go o.runWorker(workerCtx, issue, attempt, workerHost, runnerCommand, backend, automation.ProfileName, false, nil, &automation)
}

func filterAllowedActionsForAutomation(actions []string, automation *AutomationDispatch) []string {
	normalized := config.NormalizeAllowedActions(actions)
	if automation == nil {
		return normalized
	}
	if automation.Trigger.Type != config.AutomationTriggerInputRequired || automation.AutoResume {
		return normalized
	}
	return slices.DeleteFunc(normalized, func(action string) bool {
		return action == config.AgentActionProvideInput
	})
}
