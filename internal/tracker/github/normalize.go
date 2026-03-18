package github

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vnovick/symphony-go/internal/domain"
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
	number, ok := toIntVal(numberRaw)
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
		CreatedAt:  parseTime(raw["created_at"]),
		UpdatedAt:  parseTime(raw["updated_at"]),
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

// deriveState computes the Symphony state string for a GitHub issue.
// Closed issues are always "closed".
// Open issues: first matching active or terminal label wins.
// Open issues with no matching label return "" (not eligible).
func deriveState(raw map[string]any, activeStates, terminalStates []string) string {
	ghState, _ := raw["state"].(string)
	if strings.ToLower(ghState) == "closed" {
		return "closed"
	}
	labels := extractLabels(raw)
	// Check active labels first
	for _, label := range labels {
		for _, active := range activeStates {
			if strings.EqualFold(label, active) {
				return label
			}
		}
	}
	// Check terminal labels
	for _, label := range labels {
		for _, terminal := range terminalStates {
			if strings.EqualFold(label, terminal) {
				return label
			}
		}
	}
	return ""
}

func parseTime(v any) *time.Time {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func toIntVal(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
