package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BareDir is the directory name for the bare clone inside workspace root.
const BareDir = ".bare"

// BarePath returns the absolute path to the bare clone directory.
func BarePath(root string) string {
	return filepath.Join(root, BareDir)
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// EnsureBareClone ensures a bare git clone exists at <root>/.bare/.
// If the directory already exists and contains a HEAD file, it is reused.
// If cloneURL is empty and the bare dir does not exist, an error is returned.
// Returns the absolute path to the bare clone.
func EnsureBareClone(ctx context.Context, root, cloneURL string) (string, error) {
	barePath := BarePath(root)

	// Already exists — reuse.
	if info, err := os.Stat(filepath.Join(barePath, "HEAD")); err == nil && !info.IsDir() {
		slog.Debug("bare: reusing existing bare clone", "path", barePath)
		return barePath, nil
	}

	if cloneURL == "" {
		return "", fmt.Errorf("bare: %s does not exist and no clone_url configured", barePath)
	}

	// Ensure parent exists.
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("bare: mkdir root: %w", err)
	}

	slog.Info("bare: cloning repository", "url", cloneURL, "path", barePath)
	cmd := exec.CommandContext(ctx, "git", "clone", "--bare", cloneURL, barePath)
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("bare: git clone --bare: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// git clone --bare does not set a fetch refspec by default, so
	// "git fetch" would be a no-op. Configure one that maps remote branches
	// directly to refs/heads/* (not refs/remotes/origin/*). This keeps HEAD
	// valid since HEAD points to refs/heads/<default_branch>.
	refspecCmd := exec.CommandContext(ctx, "git", "-C", barePath,
		"config", "remote.origin.fetch", "+refs/heads/*:refs/heads/*")
	refspecCmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	if out, err := refspecCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("bare: set fetch refspec: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return barePath, nil
}

// FetchBare runs git fetch --all --prune in the bare clone to update all remote refs.
func FetchBare(ctx context.Context, barePath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", barePath, "fetch", "--all", "--prune")
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bare: fetch: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
