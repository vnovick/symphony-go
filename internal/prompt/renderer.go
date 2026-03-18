package prompt

import (
	"fmt"
	"strings"
	"time"

	"github.com/osteele/liquid"
	"github.com/vnovick/symphony-go/internal/domain"
)

// DefaultPrompt is used when the workflow prompt body is empty.
const DefaultPrompt = "You are working on an issue."

// Render renders a Liquid template with issue and attempt variables.
// Returns template_parse_error on bad syntax, template_render_error on unknown vars/filters.
func Render(tmpl string, issue domain.Issue, attempt *int) (string, error) {
	if strings.TrimSpace(tmpl) == "" {
		return DefaultPrompt, nil
	}

	engine := liquid.NewEngine()
	engine.StrictVariables()

	tpl, err := engine.ParseTemplate([]byte(tmpl))
	if err != nil {
		return "", fmt.Errorf("template_parse_error: %w", err)
	}

	bindings := map[string]any{
		"issue":   issueToMap(issue),
		"attempt": attemptValue(attempt),
	}

	out, err := tpl.Render(bindings)
	if err != nil {
		return "", fmt.Errorf("template_render_error: %w", err)
	}

	return string(out), nil
}

func attemptValue(attempt *int) any {
	if attempt == nil {
		return nil
	}
	return *attempt
}

// issueToMap converts an Issue to a string-keyed map for Liquid template consumption.
func issueToMap(issue domain.Issue) map[string]any {
	return map[string]any{
		"id":          issue.ID,
		"identifier":  issue.Identifier,
		"title":       issue.Title,
		"description": derefString(issue.Description),
		"priority":    derefInt(issue.Priority),
		"state":       issue.State,
		"branch_name": derefString(issue.BranchName),
		"url":         derefString(issue.URL),
		"labels":      labelsValue(issue.Labels),
		"blocked_by":  blockersValue(issue.BlockedBy),
		"created_at":  timeValue(issue.CreatedAt),
		"updated_at":  timeValue(issue.UpdatedAt),
	}
}

func derefString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func derefInt(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}

func timeValue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func labelsValue(labels []string) []any {
	if labels == nil {
		return []any{}
	}
	out := make([]any, len(labels))
	for i, l := range labels {
		out[i] = l
	}
	return out
}

func blockersValue(blockers []domain.BlockerRef) []any {
	out := make([]any, len(blockers))
	for i, b := range blockers {
		out[i] = map[string]any{
			"id":         derefString(b.ID),
			"identifier": derefString(b.Identifier),
			"state":      derefString(b.State),
		}
	}
	return out
}
