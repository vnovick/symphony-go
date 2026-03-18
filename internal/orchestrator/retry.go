package orchestrator

import "time"

// BackoffMs computes the exponential backoff delay for failure-driven retries.
// Formula: min(10000 * 2^(attempt-1), maxMs)
func BackoffMs(attempt, maxMs int) int {
	if attempt <= 0 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 30 {
		shift = 30
	}
	delay := 10000 * (1 << uint(shift))
	if delay > maxMs || delay < 0 {
		return maxMs
	}
	return delay
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
