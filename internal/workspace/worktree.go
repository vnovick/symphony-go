package workspace

import (
	"context"
	"fmt"
	"log/slog"
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
	"":        true,
	"main":    true,
	"master":  true,
	"develop": true,
	"HEAD":    true,
}

// IsDefaultBranch reports whether branch is a well-known default branch name
// (empty string, "main", "master", "develop", or "HEAD"). These names are
// treated as "no feature branch" and should never be used as worktree branch names.
func IsDefaultBranch(branch string) bool {
	return defaultBranches[branch]
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
	if branchName != nil && !IsDefaultBranch(*branchName) {
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
		// Still assert containment: a symlink placed at wtPath pointing outside root
		// would bypass the check on the creation path.
		if err := AssertContained(root, wtPath); err != nil {
			return Workspace{}, err
		}
		return Workspace{Path: wtPath, Identifier: identifier, CreatedNow: false}, nil
	}

	// Ensure the worktrees/ parent directory exists.
	worktreesDir := filepath.Join(root, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		return Workspace{}, fmt.Errorf("worktree: create worktrees dir: %w", err)
	}

	// Best-effort fetch so that remote-only PR branches are available locally.
	fetchCmd := exec.CommandContext(ctx, "git", "-C", root, "fetch", "origin", branchName)
	if err := fetchCmd.Run(); err != nil {
		slog.Debug("worktree: fetch failed (best-effort)", "branch", branchName, "error", err)
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
// LANG=C and LC_ALL=C are forced so that the English error text ("already exists")
// is reliably produced on non-English systems and can be matched by ensureWorktree.
func runGitWorktreeAdd(ctx context.Context, root, wtPath, branchName string, createBranch bool) error {
	var args []string
	if createBranch {
		args = []string{"-C", root, "worktree", "add", wtPath, "-b", branchName}
	} else {
		args = []string{"-C", root, "worktree", "add", wtPath, branchName}
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// removeWorktree removes the git worktree for identifier and, if branchName is
// non-empty, deletes the branch from the base repo.
// Safe to call when the worktree does not exist (idempotent).
func (m *Manager) removeWorktree(ctx context.Context, identifier, branchName string) error {
	root := m.cfg.Workspace.Root
	wtPath := worktreePath(root, identifier)

	// git -C <root> worktree remove --force <wtPath>
	cmd := exec.CommandContext(ctx, "git", "-C", root, "worktree", "remove", "--force", wtPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		output := strings.TrimSpace(string(out))
		// These messages mean the worktree is already gone — treat as success.
		if !strings.Contains(output, "is not a working tree") &&
			!strings.Contains(output, "No such file or directory") {
			return fmt.Errorf("worktree: remove: %w: %s", err, output)
		}
	}

	// Prune stale metadata from .git/worktrees/
	_ = exec.CommandContext(ctx, "git", "-C", root, "worktree", "prune").Run()

	// Delete branch (best-effort, only when caller provides a name).
	if branchName != "" {
		_ = exec.CommandContext(ctx, "git", "-C", root, "branch", "-D", branchName).Run()
	}

	return nil
}
