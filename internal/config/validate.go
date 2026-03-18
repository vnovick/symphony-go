package config

import (
	"fmt"

	"github.com/osteele/liquid"
)

var supportedTrackerKinds = map[string]bool{
	"linear": true,
	"github": true,
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

	return nil
}
