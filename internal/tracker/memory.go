package tracker

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/vnovick/symphony-go/internal/domain"
)

// MemoryTracker is an in-memory Tracker implementation for tests and reconciliation.
type MemoryTracker struct {
	mu             sync.RWMutex
	issues         []domain.Issue
	activeStates   []string
	terminalStates []string
	injectedError  error
}

// NewMemoryTracker constructs a MemoryTracker with the given issues and state config.
func NewMemoryTracker(issues []domain.Issue, activeStates, terminalStates []string) *MemoryTracker {
	cp := make([]domain.Issue, len(issues))
	copy(cp, issues)
	return &MemoryTracker{
		issues:         cp,
		activeStates:   activeStates,
		terminalStates: terminalStates,
	}
}

// InjectError causes all subsequent calls to return the given error.
// Pass nil to clear.
func (m *MemoryTracker) InjectError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.injectedError = err
}

// SetIssueState updates the state of an issue by ID.
func (m *MemoryTracker) SetIssueState(id, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.issues {
		if m.issues[i].ID == id {
			m.issues[i].State = state
			return
		}
	}
}

// FetchCandidateIssues returns issues whose state (case-insensitive) is in activeStates.
func (m *MemoryTracker) FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.injectedError != nil {
		return nil, m.injectedError
	}
	var result []domain.Issue
	for _, issue := range m.issues {
		if m.isActive(issue.State) {
			result = append(result, issue)
		}
	}
	return result, nil
}

// FetchIssuesByStates returns issues whose state matches any of the given state names (case-insensitive).
func (m *MemoryTracker) FetchIssuesByStates(ctx context.Context, stateNames []string) ([]domain.Issue, error) {
	if len(stateNames) == 0 {
		return []domain.Issue{}, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.injectedError != nil {
		return nil, m.injectedError
	}
	wantSet := make(map[string]bool, len(stateNames))
	for _, s := range stateNames {
		wantSet[strings.ToLower(s)] = true
	}
	var result []domain.Issue
	for _, issue := range m.issues {
		if wantSet[strings.ToLower(issue.State)] {
			result = append(result, issue)
		}
	}
	return result, nil
}

// FetchIssueStatesByIDs returns issues matching the given IDs.
func (m *MemoryTracker) FetchIssueStatesByIDs(ctx context.Context, issueIDs []string) ([]domain.Issue, error) {
	if len(issueIDs) == 0 {
		return []domain.Issue{}, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.injectedError != nil {
		return nil, m.injectedError
	}
	idSet := make(map[string]bool, len(issueIDs))
	for _, id := range issueIDs {
		idSet[id] = true
	}
	var result []domain.Issue
	for _, issue := range m.issues {
		if idSet[issue.ID] {
			result = append(result, issue)
		}
	}
	return result, nil
}

// CreateComment is a no-op for the in-memory tracker.
func (m *MemoryTracker) CreateComment(_ context.Context, _, _ string) error {
	return nil
}

// UpdateIssueState updates the in-memory state for testing.
func (m *MemoryTracker) UpdateIssueState(_ context.Context, issueID, stateName string) error {
	m.SetIssueState(issueID, stateName)
	return nil
}

// SetIssueBranch updates the BranchName field on the in-memory issue.
func (m *MemoryTracker) SetIssueBranch(_ context.Context, issueID, branchName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.issues {
		if m.issues[i].ID == issueID {
			b := branchName
			m.issues[i].BranchName = &b
			return nil
		}
	}
	return nil // issue not found — non-fatal
}

// FetchIssueDetail returns the issue from storage if it exists, else an error.
func (m *MemoryTracker) FetchIssueDetail(_ context.Context, issueID string) (*domain.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, issue := range m.issues {
		if issue.ID == issueID {
			cp := issue
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("issue %s not found", issueID)
}

// FetchIssueByIdentifier returns the issue matching the human-readable identifier.
func (m *MemoryTracker) FetchIssueByIdentifier(_ context.Context, identifier string) (*domain.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, issue := range m.issues {
		if issue.Identifier == identifier {
			cp := issue
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("issue %s not found", identifier)
}

func (m *MemoryTracker) isActive(state string) bool {
	lower := strings.ToLower(state)
	for _, s := range m.activeStates {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return false
}
