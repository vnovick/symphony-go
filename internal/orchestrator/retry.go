package orchestrator

import "time"

// BackoffMs computes the exponential backoff delay for failure-driven retries.
//
// Formula: min(10_000 × 2^(attempt-1), maxMs)
//
// The base of 10 s was chosen so the first retry fires quickly enough to catch
// transient tracker or subprocess errors, while subsequent attempts grow slowly
// enough that a sustained outage does not produce a flood of retries:
//
//	attempt 1 →  10 s
//	attempt 2 →  20 s
//	attempt 3 →  40 s
//	attempt 4 →  80 s
//	attempt 5 → 160 s
//	attempt 6 → 300 s  (capped by the default max_retry_backoff_ms = 300 000)
//
// The shift is capped at 30 so that 10_000 × 2^30 (~10 billion ms) still fits
// comfortably in a 64-bit int. Go guarantees 64-bit int on all current
// production targets, so no overflow can occur within this bound.
// maxMs is the ceiling; set via agent.max_retry_backoff_ms (default 300 000 ms).
func BackoffMs(attempt, maxMs int) int {
	if attempt <= 0 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 30 {
		shift = 30
	}
	delay := 10000 * (1 << uint(shift))
	return min(delay, maxMs)
}

// ScheduleRetry inserts a RetryEntry for issueID and marks it claimed.
// delayMs is the delay from now until the retry fires.
func ScheduleRetry(state State, issueID string, attempt int, identifier, errMsg string, now time.Time, delayMs int) State {
	dueAt := now.Add(time.Duration(delayMs) * time.Millisecond)
	var errPtr *string
	if errMsg != "" {
		s := errMsg
		errPtr = &s
	}
	state.RetryAttempts[issueID] = &RetryEntry{
		IssueID:    issueID,
		Identifier: identifier,
		Attempt:    attempt,
		DueAt:      dueAt,
		Error:      errPtr,
	}
	state.Claimed[issueID] = struct{}{}
	return state
}

// CancelRetry removes a pending retry entry and releases the claim.
func CancelRetry(state State, issueID string) State {
	delete(state.RetryAttempts, issueID)
	delete(state.Claimed, issueID)
	return state
}
