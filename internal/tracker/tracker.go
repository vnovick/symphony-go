package tracker

import (
	"context"

	"github.com/vnovick/symphony-go/internal/domain"
)

// Tracker is the interface all tracker adapters must implement.
type Tracker interface {
	// FetchCandidateIssues returns issues in active states for the configured project.
	FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error)

	// FetchIssuesByStates returns issues matching the given state names.
	// Empty stateNames returns empty slice without any API call.
	FetchIssuesByStates(ctx context.Context, stateNames []string) ([]domain.Issue, error)

	// FetchIssueStatesByIDs returns the current state snapshot for the given issue IDs.
	// Empty issueIDs returns empty slice without any API call.
	FetchIssueStatesByIDs(ctx context.Context, issueIDs []string) ([]domain.Issue, error)

	// CreateComment posts a comment on the given issue. Errors are logged and
	// non-fatal — callers should not abort a session on comment failure.
	CreateComment(ctx context.Context, issueID, body string) error

	// UpdateIssueState transitions the issue to the named state.
	// For Linear this resolves the state name to an ID; for GitHub it manages labels.
	// Errors are logged and non-fatal.
	UpdateIssueState(ctx context.Context, issueID, stateName string) error

	// FetchIssueDetail returns a single issue with full details including comments.
	// Used before rendering the agent prompt.
	FetchIssueDetail(ctx context.Context, issueID string) (*domain.Issue, error)

	// SetIssueBranch records the feature branch name on the tracker issue so
	// that retried workers can resume from the same branch instead of starting
	// over from the default branch. Errors are non-fatal — callers log and ignore.
	SetIssueBranch(ctx context.Context, issueID, branchName string) error
}
