package domain

import "time"

// Comment — a comment on a tracker issue.
type Comment struct {
	Body       string
	CreatedAt  *time.Time
	AuthorName string
}

// Issue — normalized tracker record.
type Issue struct {
	ID          string
	Identifier  string
	Title       string
	Description *string
	Priority    *int
	State       string
	BranchName  *string
	URL         *string
	Labels      []string
	BlockedBy   []BlockerRef
	Comments    []Comment
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

// BlockerRef — a lightweight reference to a blocking issue.
type BlockerRef struct {
	ID         *string
	Identifier *string
	State      *string
}
