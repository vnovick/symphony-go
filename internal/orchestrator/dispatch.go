package orchestrator

import (
	"sort"
	"strings"

	"github.com/vnovick/symphony-go/internal/config"
	"github.com/vnovick/symphony-go/internal/domain"
)

// IneligibleReason returns a short string explaining why an issue cannot be
// dispatched, or "" if it is eligible. Useful for diagnostic logging.
func IneligibleReason(issue domain.Issue, state State, cfg *config.Config) string {
	if issue.ID == "" || issue.Identifier == "" || issue.Title == "" || issue.State == "" {
		return "missing_fields"
	}
	if !isActiveState(issue.State, state) {
		return "not_active_state"
	}
	if isTerminalState(issue.State, state) {
		return "terminal_state"
	}
	if _, paused := state.PausedIdentifiers[issue.Identifier]; paused {
		return "paused"
	}
	if _, discarding := state.DiscardingIdentifiers[issue.Identifier]; discarding {
		return "discarding"
	}
	if _, running := state.Running[issue.ID]; running {
		return "already_running"
	}
	if _, claimed := state.Claimed[issue.ID]; claimed {
		return "claimed"
	}
	if AvailableSlots(state) <= 0 {
		return "no_slots"
	}
	stateKey := strings.ToLower(issue.State)
	if limit, ok := cfg.Agent.MaxConcurrentAgentsByState[stateKey]; ok {
		if countRunningInState(state, issue.State) >= limit {
			return "per_state_limit"
		}
	}
	if strings.EqualFold(issue.State, "todo") {
		for _, blocker := range issue.BlockedBy {
			if blocker.State == nil {
				continue
			}
			if isTerminalState(*blocker.State, state) {
				continue
			}
			if blocker.Identifier != nil {
				if _, autoPaused := state.PausedIdentifiers[*blocker.Identifier]; autoPaused {
					continue
				}
			}
			id := ""
			if blocker.Identifier != nil {
				id = *blocker.Identifier
			}
			return "blocked_by:" + id
		}
	}
	return ""
}

// IsEligible returns true when an issue passes all dispatch eligibility checks.
func IsEligible(issue domain.Issue, state State, cfg *config.Config) bool {
	return IneligibleReason(issue, state, cfg) == ""
}

// SortForDispatch sorts issues: priority ASC (nil last), then created_at oldest first,
// then identifier lexicographic.
func SortForDispatch(issues []domain.Issue) []domain.Issue {
	out := make([]domain.Issue, len(issues))
	copy(out, issues)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Priority == nil && b.Priority == nil:
			// fall through
		case a.Priority == nil:
			return false
		case b.Priority == nil:
			return true
		case *a.Priority != *b.Priority:
			return *a.Priority < *b.Priority
		}
		switch {
		case a.CreatedAt == nil && b.CreatedAt == nil:
			// fall through
		case a.CreatedAt == nil:
			return false
		case b.CreatedAt == nil:
			return true
		case !a.CreatedAt.Equal(*b.CreatedAt):
			return a.CreatedAt.Before(*b.CreatedAt)
		}
		return a.Identifier < b.Identifier
	})
	return out
}

// AvailableSlots returns how many more agents can be dispatched globally.
// It reads state.MaxConcurrentAgents (snapshotted from cfg at the start of
// each tick under cfgMu) rather than cfg directly, so the event loop can call
// it lock-free throughout a tick.
func AvailableSlots(state State) int {
	n := state.MaxConcurrentAgents - len(state.Running)
	if n < 0 {
		return 0
	}
	return n
}

func isActiveState(s string, state State) bool {
	for _, a := range state.ActiveStates {
		if strings.EqualFold(s, a) {
			return true
		}
	}
	return false
}

func isTerminalState(s string, state State) bool {
	for _, t := range state.TerminalStates {
		if strings.EqualFold(s, t) {
			return true
		}
	}
	return false
}

func countRunningInState(state State, issueState string) int {
	count := 0
	for _, entry := range state.Running {
		if strings.EqualFold(entry.Issue.State, issueState) {
			count++
		}
	}
	return count
}
