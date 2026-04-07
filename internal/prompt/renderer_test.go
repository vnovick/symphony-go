package prompt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/prompt"
)

func baseIssue() domain.Issue {
	desc := "Fix the bug"
	prio := 1
	branch := "eng-1-fix"
	url := "https://linear.app/org/issue/ENG-1"
	now := time.Now()
	return domain.Issue{
		ID:          "issue-id-1",
		Identifier:  "ENG-1",
		Title:       "Fix the bug",
		Description: &desc,
		Priority:    &prio,
		State:       "In Progress",
		BranchName:  &branch,
		URL:         &url,
		Labels:      []string{"bug", "backend"},
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
}

func TestRenderBasicTemplate(t *testing.T) {
	tmpl := "Issue: {{ issue.identifier }} - {{ issue.title }}"
	result, err := prompt.Render(tmpl, baseIssue(), nil)
	require.NoError(t, err)
	assert.Equal(t, "Issue: ENG-1 - Fix the bug", result)
}

func TestRenderWithAttempt(t *testing.T) {
	tmpl := "Attempt: {{ attempt }}"
	attempt := 3
	result, err := prompt.Render(tmpl, baseIssue(), &attempt)
	require.NoError(t, err)
	assert.Equal(t, "Attempt: 3", result)
}

func TestRenderNilAttempt(t *testing.T) {
	tmpl := "{% if attempt %}retry {{ attempt }}{% else %}first run{% endif %}"
	result, err := prompt.Render(tmpl, baseIssue(), nil)
	require.NoError(t, err)
	assert.Equal(t, "first run", result)
}

func TestRenderIssueDescription(t *testing.T) {
	tmpl := "{% if issue.description %}{{ issue.description }}{% else %}no desc{% endif %}"
	result, err := prompt.Render(tmpl, baseIssue(), nil)
	require.NoError(t, err)
	assert.Equal(t, "Fix the bug", result)
}

func TestRenderNilDescription(t *testing.T) {
	tmpl := "{% if issue.description %}{{ issue.description }}{% else %}no desc{% endif %}"
	issue := baseIssue()
	issue.Description = nil
	result, err := prompt.Render(tmpl, issue, nil)
	require.NoError(t, err)
	assert.Equal(t, "no desc", result)
}

func TestRenderLabelsIterable(t *testing.T) {
	tmpl := "{% for label in issue.labels %}{{ label }} {% endfor %}"
	result, err := prompt.Render(tmpl, baseIssue(), nil)
	require.NoError(t, err)
	assert.Equal(t, "bug backend ", result)
}

func TestRenderUnknownVariableIsError(t *testing.T) {
	tmpl := "{{ unknown_var }}"
	_, err := prompt.Render(tmpl, baseIssue(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template_render_error")
}

func TestRenderBadSyntaxIsParseError(t *testing.T) {
	tmpl := "{% for %}" // unterminated for block causes parse error
	_, err := prompt.Render(tmpl, baseIssue(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template_parse_error")
}

func TestRenderEmptyTemplateReturnsDefault(t *testing.T) {
	result, err := prompt.Render("", baseIssue(), nil)
	require.NoError(t, err)
	assert.Equal(t, prompt.DefaultPrompt, result)
}

func TestRenderWhitespaceOnlyTemplateReturnsDefault(t *testing.T) {
	result, err := prompt.Render("   \n  ", baseIssue(), nil)
	require.NoError(t, err)
	assert.Equal(t, prompt.DefaultPrompt, result)
}

func TestRenderIncludesComments(t *testing.T) {
	now := time.Now()
	issue := domain.Issue{
		ID:         "i1",
		Identifier: "ENG-1",
		Title:      "Fix bug",
		State:      "Todo",
		Comments: []domain.Comment{
			{Body: "Please fix the crash", AuthorName: "alice", CreatedAt: &now},
		},
	}
	tmpl := "{% for c in issue.comments %}{{c.author_name}}: {{c.body}}{% endfor %}"
	out, err := prompt.Render(tmpl, issue, nil)
	require.NoError(t, err)
	assert.Equal(t, "alice: Please fix the crash", out)
}
