package tracker_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/domain"
	"github.com/vnovick/symphony-go/internal/tracker"
)

func makeIssue(id, identifier, state string) domain.Issue {
	return domain.Issue{
		ID:         id,
		Identifier: identifier,
		Title:      "Issue " + identifier,
		State:      state,
	}
}

func TestMemoryTrackerFetchCandidateIssues(t *testing.T) {
	issues := []domain.Issue{
		makeIssue("1", "ENG-1", "Todo"),
		makeIssue("2", "ENG-2", "In Progress"),
		makeIssue("3", "ENG-3", "Done"),
	}
	mem := tracker.NewMemoryTracker(issues, []string{"Todo", "In Progress"}, []string{"Done"})

	candidates, err := mem.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	assert.Len(t, candidates, 2)
	ids := []string{candidates[0].ID, candidates[1].ID}
	assert.ElementsMatch(t, []string{"1", "2"}, ids)
}

func TestMemoryTrackerFetchIssuesByStates(t *testing.T) {
	issues := []domain.Issue{
		makeIssue("1", "ENG-1", "Todo"),
		makeIssue("2", "ENG-2", "Done"),
		makeIssue("3", "ENG-3", "Cancelled"),
	}
	mem := tracker.NewMemoryTracker(issues, []string{"Todo"}, []string{"Done", "Cancelled"})

	result, err := mem.FetchIssuesByStates(context.Background(), []string{"Done", "Cancelled"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestMemoryTrackerFetchIssuesByStatesEmptyReturnsEmpty(t *testing.T) {
	mem := tracker.NewMemoryTracker(nil, nil, nil)
	result, err := mem.FetchIssuesByStates(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMemoryTrackerFetchIssueStatesByIDs(t *testing.T) {
	issues := []domain.Issue{
		makeIssue("id-1", "ENG-1", "Todo"),
		makeIssue("id-2", "ENG-2", "In Progress"),
	}
	mem := tracker.NewMemoryTracker(issues, []string{"Todo", "In Progress"}, nil)

	result, err := mem.FetchIssueStatesByIDs(context.Background(), []string{"id-1"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "id-1", result[0].ID)
	assert.Equal(t, "Todo", result[0].State)
}

func TestMemoryTrackerFetchIssueStatesByIDsEmptyReturnsEmpty(t *testing.T) {
	mem := tracker.NewMemoryTracker(nil, nil, nil)
	result, err := mem.FetchIssueStatesByIDs(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMemoryTrackerUpdateIssueState(t *testing.T) {
	issues := []domain.Issue{
		makeIssue("id-1", "ENG-1", "Todo"),
	}
	mem := tracker.NewMemoryTracker(issues, []string{"Todo", "In Progress"}, []string{"Done"})

	mem.SetIssueState("id-1", "Done")

	result, err := mem.FetchIssueStatesByIDs(context.Background(), []string{"id-1"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Done", result[0].State)

	// No longer a candidate
	candidates, err := mem.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestMemoryTrackerFetchCandidateIssuesCaseInsensitive(t *testing.T) {
	issues := []domain.Issue{
		makeIssue("1", "ENG-1", "todo"),
		makeIssue("2", "ENG-2", "IN PROGRESS"),
	}
	mem := tracker.NewMemoryTracker(issues, []string{"Todo", "In Progress"}, nil)
	candidates, err := mem.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	assert.Len(t, candidates, 2)
}

func TestMemoryTrackerInjectError(t *testing.T) {
	mem := tracker.NewMemoryTracker(nil, nil, nil)
	mem.InjectError(assert.AnError)
	_, err := mem.FetchCandidateIssues(context.Background())
	assert.Error(t, err)
	_, err = mem.FetchIssuesByStates(context.Background(), []string{"Todo"})
	assert.Error(t, err)
	_, err = mem.FetchIssueStatesByIDs(context.Background(), []string{"id-1"})
	assert.Error(t, err)
}
