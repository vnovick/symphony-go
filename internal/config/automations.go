package config

const (
	AutomationTriggerCron              = "cron"
	AutomationTriggerInputRequired     = "input_required"
	AutomationTriggerTrackerComment    = "tracker_comment_added"
	AutomationTriggerIssueEnteredState = "issue_entered_state"
	AutomationTriggerIssueMovedBacklog = "issue_moved_to_backlog"
	AutomationTriggerRunFailed         = "run_failed"
	AutomationFilterMatchAll           = "all"
	AutomationFilterMatchAny           = "any"
)

type AutomationTriggerConfig struct {
	Type     string
	Cron     string
	Timezone string
	State    string
}

type AutomationFilterConfig struct {
	MatchMode         string
	States            []string
	LabelsAny         []string
	IdentifierRegex   string
	Limit             int
	InputContextRegex string
}

type AutomationPolicyConfig struct {
	AutoResume bool
}

type AutomationConfig struct {
	ID           string
	Enabled      bool
	Profile      string
	Instructions string
	Trigger      AutomationTriggerConfig
	Filter       AutomationFilterConfig
	Policy       AutomationPolicyConfig
}

type ScheduleFilterConfig struct {
	States          []string
	LabelsAny       []string
	IdentifierRegex string
	Limit           int
}

type ScheduleConfig struct {
	ID       string
	Enabled  bool
	Cron     string
	Timezone string
	Profile  string
	Filter   ScheduleFilterConfig
}

func parseAutomations(raw any) []AutomationConfig {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	automations := make([]AutomationConfig, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := strField(m, "id", "")
		profile := strField(m, "profile", "")
		trigger := nestedMap(m, "trigger")
		triggerType := strField(trigger, "type", "")
		if id == "" || profile == "" || triggerType == "" {
			continue
		}
		filter := nestedMap(m, "filter")
		policy := nestedMap(m, "policy")
		limit := intField(filter, "limit", 0)
		if limit < 0 {
			limit = 0
		}
		automations = append(automations, AutomationConfig{
			ID:           id,
			Enabled:      boolField(m, "enabled", true),
			Profile:      profile,
			Instructions: strField(m, "instructions", ""),
			Trigger: AutomationTriggerConfig{
				Type:     triggerType,
				Cron:     strField(trigger, "cron", ""),
				Timezone: strField(trigger, "timezone", ""),
				State:    strField(trigger, "state", ""),
			},
			Filter: AutomationFilterConfig{
				MatchMode:         normalizeAutomationMatchMode(strField(filter, "match_mode", "")),
				States:            strSliceField(filter, "states", nil),
				LabelsAny:         strSliceField(filter, "labels_any", nil),
				IdentifierRegex:   strField(filter, "identifier_regex", ""),
				Limit:             limit,
				InputContextRegex: strField(filter, "input_context_regex", ""),
			},
			Policy: AutomationPolicyConfig{
				AutoResume: boolField(policy, "auto_resume", false),
			},
		})
	}
	if len(automations) == 0 {
		return nil
	}
	return automations
}

func normalizeAutomationMatchMode(value string) string {
	switch value {
	case AutomationFilterMatchAny:
		return AutomationFilterMatchAny
	case "", AutomationFilterMatchAll:
		return AutomationFilterMatchAll
	default:
		return AutomationFilterMatchAll
	}
}

func parseSchedules(raw any) []ScheduleConfig {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	schedules := make([]ScheduleConfig, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := strField(m, "id", "")
		cronExpr := strField(m, "cron", "")
		profile := strField(m, "profile", "")
		if id == "" || cronExpr == "" || profile == "" {
			continue
		}
		filter := nestedMap(m, "filter")
		limit := intField(filter, "limit", 0)
		if limit < 0 {
			limit = 0
		}
		schedules = append(schedules, ScheduleConfig{
			ID:       id,
			Enabled:  boolField(m, "enabled", true),
			Cron:     cronExpr,
			Timezone: strField(m, "timezone", ""),
			Profile:  profile,
			Filter: ScheduleFilterConfig{
				States:          strSliceField(filter, "states", nil),
				LabelsAny:       strSliceField(filter, "labels_any", nil),
				IdentifierRegex: strField(filter, "identifier_regex", ""),
				Limit:           limit,
			},
		})
	}
	if len(schedules) == 0 {
		return nil
	}
	return schedules
}

func legacySchedulesToAutomations(schedules []ScheduleConfig) []AutomationConfig {
	if len(schedules) == 0 {
		return nil
	}
	automations := make([]AutomationConfig, 0, len(schedules))
	for _, schedule := range schedules {
		automations = append(automations, AutomationConfig{
			ID:      schedule.ID,
			Enabled: schedule.Enabled,
			Profile: schedule.Profile,
			Trigger: AutomationTriggerConfig{
				Type:     AutomationTriggerCron,
				Cron:     schedule.Cron,
				Timezone: schedule.Timezone,
			},
			Filter: AutomationFilterConfig{
				States:          schedule.Filter.States,
				LabelsAny:       schedule.Filter.LabelsAny,
				IdentifierRegex: schedule.Filter.IdentifierRegex,
				Limit:           schedule.Filter.Limit,
			},
		})
	}
	return automations
}
