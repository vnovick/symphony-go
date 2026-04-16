package agentactions

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"
)

type Grant struct {
	IssueIdentifier  string
	RunSessionID     string
	AllowedActions   []string
	CreateIssueState string
	ExpiresAt        time.Time
}

type Store struct {
	mu     sync.Mutex
	grants map[string]Grant
}

func NewStore() *Store {
	return &Store{
		grants: make(map[string]Grant),
	}
}

func (s *Store) Issue(issueIdentifier, runSessionID string, allowedActions []string, createIssueState string, ttl time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(ttl)
	if ttl <= 0 {
		expiresAt = time.Now().Add(time.Hour)
	}

	actions := slices.Clone(allowedActions)
	slices.Sort(actions)
	s.grants[token] = Grant{
		IssueIdentifier:  issueIdentifier,
		RunSessionID:     runSessionID,
		AllowedActions:   actions,
		CreateIssueState: createIssueState,
		ExpiresAt:        expiresAt,
	}
	return token, nil
}

func (s *Store) Revoke(token string) {
	if s == nil || token == "" {
		return
	}
	s.mu.Lock()
	delete(s.grants, token)
	s.mu.Unlock()
}

func (s *Store) Validate(token, issueIdentifier, action string, now time.Time) (Grant, string, bool) {
	if s == nil || token == "" {
		return Grant{}, "missing_token", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	grant, ok := s.grants[token]
	if !ok {
		return Grant{}, "unknown_token", false
	}
	if now.After(grant.ExpiresAt) {
		delete(s.grants, token)
		return Grant{}, "expired_token", false
	}
	if grant.IssueIdentifier != issueIdentifier {
		return Grant{}, "issue_mismatch", false
	}
	if !slices.Contains(grant.AllowedActions, action) {
		return Grant{}, "action_not_allowed", false
	}
	return grant, "", true
}
