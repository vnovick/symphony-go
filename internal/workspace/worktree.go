package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// defaultBranches is the set of branch names treated as "no feature branch".
// If issue.BranchName is one of these, ResolveWorktreeBranch falls back to
// the symphony/<slug> convention.
var defaultBranches = map[string]bool{
	"main":    true,
	"master":  true,
	"develop": true,
}

// SlugifyIdentifier lowercases the identifier and replaces any non-alphanumeric
// character with a hyphen, trimming leading/trailing hyphens and deduplicating consecutive hyphens.
// Examples: "ENG-123" → "eng-123", "My Issue #7" → "my-issue-7".
func SlugifyIdentifier(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	var b strings.Builder
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	// Deduplicate consecutive hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

// ResolveWorktreeBranch returns the git branch name to use for a worktree.
// Priority:
//  1. branchName if non-nil, non-empty, and not a default branch (main/master/develop)
//  2. "symphony/" + SlugifyIdentifier(identifier)
func ResolveWorktreeBranch(branchName *string, identifier string) string {
	if branchName != nil && *branchName != "" && !defaultBranches[*branchName] {
		return *branchName
	}
	return "symphony/" + SlugifyIdentifier(identifier)
}

// worktreePath returns the absolute path for the worktree for the given
// identifier: <root>/worktrees/<identifier>.
// This is unexported — callers within this package use it; the orchestrator
// gets paths via Manager.EnsureWorkspace return value.
func worktreePath(root, identifier string) string {
	return filepath.Join(root, "worktrees", identifier)
}

// ensureWorktree creates a git worktree at worktreePath(root, identifier) checked
// out on branchName. If the worktree directory already exists it is reused (retry
// case). If the branch already exists in the base repo but has no worktree, the
// branch is checked out without -b.
func (m *Manager) ensureWorktree(ctx context.Context, identifier, branchName string) (Workspace, error) {
	root := m.cfg.Workspace.Root
	wtPath := worktreePath(root, identifier)

	// Reuse existing worktree (retry case).
	if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
		return Workspace{Path: wtPath, Identifier: identifier, CreatedNow: false}, nil
	}

	// Ensure the worktrees/ parent directory exists.
	worktreesDir := filepath.Join(root, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		return Workspace{}, fmt.Errorf("worktree: create worktrees dir: %w", err)
	}

	// git -C <root> worktree add <wtPath> -b <branchName>
	if err := runGitWorktreeAdd(ctx, root, wtPath, branchName, true); err != nil {
		// Branch already exists: retry without -b to check it out into a new worktree.
		if strings.Contains(err.Error(), "already exists") {
			if err2 := runGitWorktreeAdd(ctx, root, wtPath, branchName, false); err2 != nil {
				return Workspace{}, fmt.Errorf("worktree: add (existing branch): %w", err2)
			}
		} else {
			return Workspace{}, fmt.Errorf("worktree: add: %w", err)
		}
	}

	// Safety: assert the resulting path is still under root.
	if err := AssertContained(root, wtPath); err != nil {
		_ = exec.Command("git", "-C", root, "worktree", "remove", "--force", wtPath).Run()
		return Workspace{}, err
	}

	return Workspace{Path: wtPath, Identifier: identifier, CreatedNow: true}, nil
}

// runGitWorktreeAdd runs git worktree add. If createBranch is true it passes
// -b <branchName>; otherwise checks out the existing branch directly.
func runGitWorktreeAdd(ctx context.Context, root, wtPath, branchName string, createBranch bool) error {
	var args []string
	if createBranch {
		args = []string{"-C", root, "worktree", "add", wtPath, "-b", branchName}
	} else {
		args = []string{"-C", root, "worktree", "add", wtPath, branchName}
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// removeWorktree removes the git worktree for identifier and, if branchName is
// non-empty, deletes the branch from the base repo.
// Safe to call when the worktree does not exist (idempotent).
func (m *Manager) removeWorktree(identifier, branchName string) error {
	root := m.cfg.Workspace.Root
	wtPath := worktreePath(root, identifier)

	// git -C <root> worktree remove --force <wtPath>
	cmd := exec.Command("git", "-C", root, "worktree", "remove", "--force", wtPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		output := strings.TrimSpace(string(out))
		// These messages mean the worktree is already gone — treat as success.
		if !strings.Contains(output, "is not a working tree") &&
			!strings.Contains(output, "No such file or directory") {
			return fmt.Errorf("worktree: remove: %w: %s", err, output)
		}
	}

	// Prune stale metadata from .git/worktrees/
	_ = exec.Command("git", "-C", root, "worktree", "prune").Run()

	// Delete branch (best-effort, only when caller provides a name).
	if branchName != "" {
		_ = exec.Command("git", "-C", root, "branch", "-D", branchName).Run()
	}

	return nil
}
