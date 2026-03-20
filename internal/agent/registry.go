package agent

import (
	"fmt"
	"strings"
)

// RunnerKind is a typed string identifying an agent backend.
type RunnerKind string

// Well-known runner kinds.
const (
	RunnerClaudeCode RunnerKind = "claude-code"
	RunnerCodex      RunnerKind = "codex"
	RunnerGemini     RunnerKind = "gemini"
	RunnerOpenCode   RunnerKind = "opencode"
)

// allRunnerKinds is the ordered list of valid runner kinds.
var allRunnerKinds = []RunnerKind{RunnerClaudeCode, RunnerCodex, RunnerGemini, RunnerOpenCode}

// RunnerKindFrom converts a raw string to RunnerKind, defaulting to claude-code.
func RunnerKindFrom(s string) RunnerKind {
	k := RunnerKind(strings.ToLower(strings.TrimSpace(s)))
	if k.Valid() {
		return k
	}
	return RunnerClaudeCode
}

// Valid returns true if k is a recognised runner kind.
func (k RunnerKind) Valid() bool {
	switch k {
	case RunnerClaudeCode, RunnerCodex, RunnerGemini, RunnerOpenCode:
		return true
	}
	return false
}

// RunnerKindFromLabel extracts a RunnerKind from an "agent:<runner>" label.
// Returns the kind and true if found, or "" and false otherwise.
func RunnerKindFromLabel(label string) (RunnerKind, bool) {
	lower := strings.ToLower(label)
	if !strings.HasPrefix(lower, "agent:") {
		return "", false
	}
	name := strings.TrimSpace(lower[len("agent:"):])
	k := RunnerKind(name)
	if !k.Valid() {
		return "", false
	}
	return k, true
}

// ResolveRunnerFromLabels scans issue labels for an "agent:<runner>" override.
// Returns the runner kind if found, or empty RunnerKind if no override.
func ResolveRunnerFromLabels(labels []string) RunnerKind {
	for _, label := range labels {
		if kind, ok := RunnerKindFromLabel(label); ok {
			return kind
		}
	}
	return ""
}

// RunnerRegistry maps runner kinds to Runner implementations.
type RunnerRegistry struct {
	runners    map[RunnerKind]Runner
	defaultKey RunnerKind
}

// NewSingleRunnerRegistry creates a registry backed by a single Runner for all kinds.
// Useful for testing where a FakeRunner should handle all runner kinds.
func NewSingleRunnerRegistry(r Runner) *RunnerRegistry {
	return &RunnerRegistry{
		runners: map[RunnerKind]Runner{
			RunnerClaudeCode: r,
			RunnerCodex:      r,
			RunnerGemini:     r,
			RunnerOpenCode:   r,
		},
		defaultKey: RunnerClaudeCode,
	}
}

// NewRunnerRegistry creates a registry pre-populated with all built-in runners.
func NewRunnerRegistry(defaultRunner string) *RunnerRegistry {
	dk := RunnerKindFrom(defaultRunner)
	return &RunnerRegistry{
		runners: map[RunnerKind]Runner{
			RunnerClaudeCode: NewClaudeRunner(),
			RunnerCodex:      NewCodexRunner(),
			RunnerGemini:     NewGeminiRunner(),
			RunnerOpenCode:   NewOpenCodeRunner(),
		},
		defaultKey: dk,
	}
}

// Get returns the runner for the given kind, falling back to the default.
func (r *RunnerRegistry) Get(kind RunnerKind) Runner {
	if runner, ok := r.runners[kind]; ok {
		return runner
	}
	return r.runners[r.defaultKey]
}

// Default returns the default runner.
func (r *RunnerRegistry) Default() Runner {
	return r.runners[r.defaultKey]
}

// DefaultKind returns the default runner kind.
func (r *RunnerRegistry) DefaultKind() RunnerKind {
	return r.defaultKey
}

// DefaultCommand returns the default CLI command for a runner kind.
func DefaultCommand(kind RunnerKind) string {
	switch kind {
	case RunnerClaudeCode:
		return "claude"
	case RunnerCodex:
		return "codex"
	case RunnerGemini:
		return "gemini"
	case RunnerOpenCode:
		return "opencode"
	default:
		return "claude"
	}
}

// BackendName returns the display name for state/UI.
func BackendName(kind RunnerKind) string {
	if kind == RunnerClaudeCode {
		return "claude"
	}
	if kind == "" {
		return "claude"
	}
	return string(kind)
}

// ValidateRunnerName checks that a runner name string is valid.
func ValidateRunnerName(name string) error {
	if name == "" {
		return nil
	}
	k := RunnerKind(strings.ToLower(strings.TrimSpace(name)))
	if !k.Valid() {
		names := make([]string, len(allRunnerKinds))
		for i, rk := range allRunnerKinds {
			names[i] = string(rk)
		}
		return fmt.Errorf("unknown agent runner %q (valid: %s)", name, strings.Join(names, ", "))
	}
	return nil
}
