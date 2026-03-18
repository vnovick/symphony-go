package workspace

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeRe = regexp.MustCompile(`[^A-Za-z0-9._\-]`)

// SanitizeKey replaces any character not in [A-Za-z0-9._-] with underscore.
func SanitizeKey(identifier string) string {
	return unsafeRe.ReplaceAllString(identifier, "_")
}

// WorkspacePath returns the absolute path for the given identifier under root.
// The identifier is sanitized before joining.
func WorkspacePath(root, identifier string) string { //nolint:revive
	return filepath.Join(root, SanitizeKey(identifier))
}

// AssertContained verifies that path is strictly contained within root using
// filepath.EvalSymlinks to resolve symlinks. Returns an error if path equals
// root, escapes root via symlink, or is otherwise outside root.
func AssertContained(root, path string) error {
	canonRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("workspace_path_unreadable: root %q: %w", root, err)
	}
	canonPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("workspace_symlink_escape: path %q could not be resolved: %w", path, err)
	}
	rootPrefix := canonRoot + string(filepath.Separator)
	if canonPath == canonRoot {
		return fmt.Errorf("workspace_equals_root: path equals workspace root %q", canonRoot)
	}
	if !strings.HasPrefix(canonPath+string(filepath.Separator), rootPrefix) {
		return fmt.Errorf("workspace_outside_root: %q is not under %q", canonPath, canonRoot)
	}
	return nil
}
