package orchestrator

import (
	"testing"

	"github.com/vnovick/itervox/internal/config"
)

func TestSetIssueBackend_RoundTrip(t *testing.T) {
	o := New(&config.Config{}, nil, nil, nil)

	// Initially empty
	if got := o.Snapshot().IssueBackends["PROJ-1"]; got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	// Set codex
	o.SetIssueBackend("PROJ-1", "codex")
	if got := o.Snapshot().IssueBackends["PROJ-1"]; got != "codex" {
		t.Fatalf("expected codex, got %q", got)
	}

	// Clear
	o.SetIssueBackend("PROJ-1", "")
	if got := o.Snapshot().IssueBackends["PROJ-1"]; got != "" {
		t.Fatalf("expected empty after clear, got %q", got)
	}
}

func TestSnapshot_IncludesIssueBackends(t *testing.T) {
	o := New(&config.Config{}, nil, nil, nil)
	o.SetIssueBackend("PROJ-1", "codex")

	snap := o.Snapshot()
	if snap.IssueBackends == nil {
		t.Fatal("expected IssueBackends in snapshot, got nil")
	}
	if got := snap.IssueBackends["PROJ-1"]; got != "codex" {
		t.Fatalf("expected codex in snapshot, got %q", got)
	}
}

func TestGetIssueBackend_OverridesProfile(t *testing.T) {
	o := New(&config.Config{}, nil, nil, nil)

	// Set both a profile and a backend override
	o.SetIssueProfile("PROJ-1", "reviewer")
	o.SetIssueBackend("PROJ-1", "codex")

	// The per-issue backend should win
	if got := o.Snapshot().IssueBackends["PROJ-1"]; got != "codex" {
		t.Fatalf("expected codex override, got %q", got)
	}
}
