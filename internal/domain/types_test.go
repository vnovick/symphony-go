package domain_test

import (
	"testing"

	"github.com/vnovick/symphony-go/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestIssueZeroValue(t *testing.T) {
	var issue domain.Issue
	assert.Equal(t, "", issue.ID)
	assert.Equal(t, "", issue.Title)
	assert.Nil(t, issue.Description)
	assert.Nil(t, issue.Priority)
	assert.Nil(t, issue.BranchName)
	assert.Nil(t, issue.URL)
	assert.Empty(t, issue.Labels)
	assert.Empty(t, issue.BlockedBy)
	assert.Nil(t, issue.CreatedAt)
	assert.Nil(t, issue.UpdatedAt)
}

func TestBlockerRef(t *testing.T) {
	id := "abc"
	ident := "ENG-1"
	state := "Done"
	ref := domain.BlockerRef{ID: &id, Identifier: &ident, State: &state}
	assert.Equal(t, "abc", *ref.ID)
	assert.Equal(t, "ENG-1", *ref.Identifier)
	assert.Equal(t, "Done", *ref.State)
}
