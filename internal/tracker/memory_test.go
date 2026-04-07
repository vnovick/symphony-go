package tracker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/tracker"
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

func TestMemoryTrackerCreateComment(t *testing.T) {
	mem := tracker.NewMemoryTracker(nil, nil, nil)
	// CreateComment is a no-op; it must not error.
	err := mem.CreateComment(context.Background(), "id-1", "a comment")
	require.NoError(t, err)
}

func TestMemoryTrackerUpdateIssueStateViaMethod(t *testing.T) {
	issues := []domain.Issue{makeIssue("id-1", "ENG-1", "Todo")}
	mem := tracker.NewMemoryTracker(issues, []string{"Todo"}, []string{"Done"})

	err := mem.UpdateIssueState(context.Background(), "id-1", "Done")
	require.NoError(t, err)

	result, err := mem.FetchIssueStatesByIDs(context.Background(), []string{"id-1"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Done", result[0].State)
}

func TestMemoryTrackerSetIssueBranch(t *testing.T) {
	issues := []domain.Issue{makeIssue("id-1", "ENG-1", "In Progress")}
	mem := tracker.NewMemoryTracker(issues, []string{"In Progress"}, nil)

	err := mem.SetIssueBranch(context.Background(), "id-1", "feature/branch")
	require.NoError(t, err)

	// Verify via FetchIssueDetail.
	issue, err := mem.FetchIssueDetail(context.Background(), "id-1")
	require.NoError(t, err)
	require.NotNil(t, issue.BranchName)
	assert.Equal(t, "feature/branch", *issue.BranchName)
}

func TestMemoryTrackerSetIssueBranchUnknownID(t *testing.T) {
	mem := tracker.NewMemoryTracker(nil, nil, nil)
	// Non-fatal: should not error when issue is not found.
	err := mem.SetIssueBranch(context.Background(), "missing", "branch")
	require.NoError(t, err)
}

func TestMemoryTrackerFetchIssueDetail(t *testing.T) {
	issues := []domain.Issue{makeIssue("id-1", "ENG-1", "Todo")}
	mem := tracker.NewMemoryTracker(issues, nil, nil)

	issue, err := mem.FetchIssueDetail(context.Background(), "id-1")
	require.NoError(t, err)
	assert.Equal(t, "id-1", issue.ID)
	assert.Equal(t, "ENG-1", issue.Identifier)
}

func TestMemoryTrackerFetchIssueDetailNotFound(t *testing.T) {
	mem := tracker.NewMemoryTracker(nil, nil, nil)
	_, err := mem.FetchIssueDetail(context.Background(), "missing")
	require.Error(t, err)
}

// --- ParseTime and ToIntVal tests ---

func TestParseTimeValidRFC3339(t *testing.T) {
	s := "2024-01-15T10:30:00Z"
	got := tracker.ParseTime(s)
	require.NotNil(t, got)
	assert.Equal(t, 2024, got.Year())
	assert.Equal(t, time.January, got.Month())
	assert.Equal(t, 15, got.Day())
}

func TestParseTimeNilForNonString(t *testing.T) {
	assert.Nil(t, tracker.ParseTime(nil))
	assert.Nil(t, tracker.ParseTime(42))
	assert.Nil(t, tracker.ParseTime(3.14))
}

func TestParseTimeNilForEmptyString(t *testing.T) {
	assert.Nil(t, tracker.ParseTime(""))
}

func TestParseTimeNilForMalformed(t *testing.T) {
	assert.Nil(t, tracker.ParseTime("not-a-timestamp"))
}

func TestToIntValInt(t *testing.T) {
	v, ok := tracker.ToIntVal(int(7))
	assert.True(t, ok)
	assert.Equal(t, 7, v)
}

func TestToIntValInt64(t *testing.T) {
	v, ok := tracker.ToIntVal(int64(42))
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestToIntValFloat64(t *testing.T) {
	v, ok := tracker.ToIntVal(float64(3))
	assert.True(t, ok)
	assert.Equal(t, 3, v)
}

func TestToIntValFalseForString(t *testing.T) {
	_, ok := tracker.ToIntVal("7")
	assert.False(t, ok)
}

func TestToIntValFalseForNil(t *testing.T) {
	_, ok := tracker.ToIntVal(nil)
	assert.False(t, ok)
}
