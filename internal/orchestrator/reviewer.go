package orchestrator

import (
	"fmt"
)

// DispatchReviewer sends a reviewer dispatch event to the event loop for the
// given issue identifier. The reviewer runs as a regular worker with
// Kind="reviewer" using the configured ReviewerProfile (or the specified profile).
// Returns an error if no reviewer profile is configured.
// Safe to call from any goroutine.
func (o *Orchestrator) DispatchReviewer(identifier string) error {
	o.cfgMu.RLock()
	profile := o.cfg.Agent.ReviewerProfile
	o.cfgMu.RUnlock()

	if profile == "" {
		return fmt.Errorf("reviewer: no reviewer_profile configured")
	}

	select {
	case o.events <- OrchestratorEvent{
		Type:            EventDispatchReviewer,
		Identifier:      identifier,
		ReviewerProfile: profile,
	}:
		return nil
	default:
		return fmt.Errorf("reviewer: event channel full")
	}
}
