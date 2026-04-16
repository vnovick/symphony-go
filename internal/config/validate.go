package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/osteele/liquid"
)

var supportedTrackerKinds = map[string]bool{
	"linear": true,
	"github": true,
}

// ErrAutoClearAutoReviewConflict reports that workspace cleanup and automatic
// reviewer dispatch were enabled together in a way that would race.
var ErrAutoClearAutoReviewConflict = errors.New("workspace.auto_clear and agent.auto_review cannot both be enabled")

// ErrAutoReviewRequiresReviewerProfile reports that automatic review was
// enabled without a configured reviewer profile.
var ErrAutoReviewRequiresReviewerProfile = errors.New("agent.auto_review requires agent.reviewer_profile to be set")

// ValidateReviewerAutoReview rejects configurations where auto-review was
// enabled without a reviewer profile to dispatch.
func ValidateReviewerAutoReview(reviewerProfile string, autoReview bool) error {
	if autoReview && strings.TrimSpace(reviewerProfile) == "" {
		return fmt.Errorf("%w: set agent.reviewer_profile or disable agent.auto_review", ErrAutoReviewRequiresReviewerProfile)
	}
	return nil
}

// ValidateAutoClearAutoReview rejects configurations where automatic review is
// enabled for a reviewer profile while workspace auto-clear is also enabled.
func ValidateAutoClearAutoReview(autoClear bool, reviewerProfile string, autoReview bool) error {
	if autoClear && autoReview && strings.TrimSpace(reviewerProfile) != "" {
		return fmt.Errorf("%w: disable either workspace.auto_clear or agent.auto_review", ErrAutoClearAutoReviewConflict)
	}
	return nil
}

// ValidateDispatch runs the spec §6.3 dispatch preflight checks against an
// already-loaded Config. Call Load first; this function does not re-read the file.
func ValidateDispatch(cfg *Config) error {
	// Check 1: tracker.kind present and supported
	if cfg.Tracker.Kind == "" {
		return fmt.Errorf("missing tracker.kind: must be one of: linear, github")
	}
	if !supportedTrackerKinds[cfg.Tracker.Kind] {
		return fmt.Errorf("unsupported_tracker_kind: %q (must be linear or github)", cfg.Tracker.Kind)
	}

	// Check 3: tracker.api_key present after $VAR resolution
	if cfg.Tracker.APIKey == "" {
		return fmt.Errorf("missing tracker.api_key: must be set or resolved from $VAR")
	}

	// Check 4: tracker.project_slug present (required for GitHub; optional for Linear)
	if cfg.Tracker.Kind == "github" && cfg.Tracker.ProjectSlug == "" {
		return fmt.Errorf("missing tracker.project_slug: required for GitHub (owner/repo)")
	}

	// Check 5: agent.command present and non-empty
	if cfg.Agent.Command == "" {
		return fmt.Errorf("missing agent.command: must be non-empty (default: claude)")
	}

	// Check 6: reviewer_prompt is a valid Liquid template (if non-empty and not the default)
	if rp := cfg.Agent.ReviewerPrompt; rp != "" {
		eng := liquid.NewEngine()
		if _, err := eng.ParseTemplate([]byte(rp)); err != nil {
			return fmt.Errorf("agent.reviewer_prompt: invalid Liquid template: %w", err)
		}
	}

	// Check 7: ssh_hosts must not start with '-' or contain whitespace (prevents SSH flag injection)
	for _, host := range cfg.Agent.SSHHosts {
		if strings.HasPrefix(host, "-") || strings.ContainsAny(host, " \t") {
			return fmt.Errorf("invalid ssh host %q: must not start with '-' or contain whitespace", host)
		}
	}

	// Check 8: profile commands must not contain shell metacharacters.
	// Profile commands are passed as the first argument to bash -lc, so
	// unescaped `;`, `|`, `&`, `` ` ``, `$`, `(`, `)`, `<`, `>` allow
	// shell code injection via a crafted WORKFLOW.md. Commands are user-
	// supplied from WORKFLOW.md, but a clear validation error is better than
	// a silent foot-gun.
	const shellMetachars = ";|&`$()><"
	for name, profile := range cfg.Agent.Profiles {
		if strings.ContainsAny(profile.Command, shellMetachars) {
			return fmt.Errorf("invalid profile %q: command %q contains shell metacharacters (%s); use a wrapper script instead",
				name, profile.Command, shellMetachars)
		}
	}

	if err := ValidateReviewerAutoReview(cfg.Agent.ReviewerProfile, cfg.Agent.AutoReview); err != nil {
		return err
	}
	if err := ValidateAutoClearAutoReview(
		cfg.Workspace.AutoClearWorkspace,
		cfg.Agent.ReviewerProfile,
		cfg.Agent.AutoReview,
	); err != nil {
		return err
	}

	return nil
}
