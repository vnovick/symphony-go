package github

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/tracker"
)

var blockerRe = regexp.MustCompile(`(?i)blocked\s+by\s+#(\d+)`)

// normalizeIssue converts a raw GitHub REST API issue map to a domain.Issue.
// derivedState is the computed state string (from label/closed logic).
// Returns nil if required fields are missing.
func normalizeIssue(raw map[string]any, derivedState string) *domain.Issue {
	numberRaw, ok := raw["number"]
	if !ok {
		return nil
	}
	number, ok := tracker.ToIntVal(numberRaw)
	if !ok {
		return nil
	}
	title, _ := raw["title"].(string)
	if title == "" {
		return nil
	}

	id := strconv.Itoa(number)
	identifier := fmt.Sprintf("#%d", number)

	issue := &domain.Issue{
		ID:         id,
		Identifier: identifier,
		Title:      title,
		State:      derivedState,
		Labels:     extractLabels(raw),
		BlockedBy:  extractBlockers(raw),
		CreatedAt:  tracker.ParseTime(raw["created_at"]),
		UpdatedAt:  tracker.ParseTime(raw["updated_at"]),
	}

	if body, ok := raw["body"].(string); ok && body != "" {
		issue.Description = &body
	}
	if htmlURL, ok := raw["html_url"].(string); ok && htmlURL != "" {
		issue.URL = &htmlURL
	}
	// Priority: map p0–p3 labels to integers 0–3; nil otherwise
	if prio := priorityFromLabels(issue.Labels); prio >= 0 {
		issue.Priority = &prio
	}
	// branch_name: always nil for GitHub
	issue.BranchName = nil

	return issue
}

func extractLabels(raw map[string]any) []string {
	labelsRaw, ok := raw["labels"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(labelsRaw))
	for _, l := range labelsRaw {
		label, ok := l.(map[string]any)
		if !ok {
			continue
		}
		name, ok := label["name"].(string)
		if !ok || name == "" {
			continue
		}
		result = append(result, strings.ToLower(name))
	}
	return result
}

func extractBlockers(raw map[string]any) []domain.BlockerRef {
	body, ok := raw["body"].(string)
	if !ok || body == "" {
		return nil
	}
	matches := blockerRe.FindAllStringSubmatch(body, -1)
	result := make([]domain.BlockerRef, 0, len(matches))
	for _, m := range matches {
		num := m[1]
		id := num
		ident := "#" + num
		ref := domain.BlockerRef{
			ID:         &id,
			Identifier: &ident,
		}
		result = append(result, ref)
	}
	return result
}

func priorityFromLabels(labels []string) int {
	for _, l := range labels {
		switch l {
		case "p0":
			return 0
		case "p1":
			return 1
		case "p2":
			return 2
		case "p3":
			return 3
		}
	}
	return -1
}

// deriveState computes the Itervox state string for a GitHub issue.
// Closed issues: prefer a matching terminal label if present, otherwise return
// the first configured terminal state (so the reconciler treats it as terminal
// regardless of which label the user applied or whether they applied one at all).
// Open issues: first matching active or terminal label wins.
// Open issues with no matching label return "" (not eligible).
func deriveState(raw map[string]any, activeStates, terminalStates []string) string {
	ghState, _ := raw["state"].(string)
	labels := extractLabels(raw)
	if strings.ToLower(ghState) == "closed" {
		// Prefer a terminal label if the user applied one (e.g. "done", "cancelled").
		for _, label := range labels {
			for _, terminal := range terminalStates {
				if strings.EqualFold(label, terminal) {
					return terminal
				}
			}
		}
		// No matching terminal label — fall back to the first configured terminal
		// state so the reconciler still recognises this as a terminal event.
		if len(terminalStates) > 0 {
			return terminalStates[0]
		}
		return "closed"
	}
	// Check active labels first
	for _, label := range labels {
		for _, active := range activeStates {
			if strings.EqualFold(label, active) {
				return active
			}
		}
	}
	// Check terminal labels
	for _, label := range labels {
		for _, terminal := range terminalStates {
			if strings.EqualFold(label, terminal) {
				return terminal
			}
		}
	}
	return ""
}
