# ADR 002 — Per-Run Log Isolation via AppSessionID and Session Stamping

**Status:** Accepted
**Date:** 2026-03-25
**Authors:** Symphony Go maintainers

---

## Context

Symphony stores a single flat log file per issue (e.g. `~/.symphony/logs/linear/my-project/<id>.jsonl`).
When an issue is retried or re-dispatched, the new agent run's log lines are appended to the same file.

The Timeline page groups runs by issue and lets users expand any historical run to see its subagent
activity. Before this ADR, `extractSubagents` was called with the *entire* issue log across all runs.
This meant that expanding run #2 of an issue would show subagents from run #1 (and all earlier runs)
mixed in — making the per-run view misleading and noisy.

A secondary problem: the Timeline header had no way to indicate which runs belonged to the *current*
daemon invocation versus a previous one (e.g. after a restart).

---

## Decision

### 1. AppSessionID — daemon-invocation grouping key

`main.go` generates a 16-byte `crypto/rand` hex token (`AppSessionID`) at startup. It is:
- Stored on `Orchestrator` via `SetAppSessionID(id)`.
- Stamped on every `CompletedRun` written to history (`CompletedRun.AppSessionID`).
- Surfaced in `StateSnapshot.CurrentAppSessionID` so the frontend can display a session badge.

This lets the UI distinguish runs produced in the current daemon session from runs produced by earlier
invocations, without requiring any persistent storage beyond what is already in the history ring buffer.

### 2. Session-stamped log entries

Claude Code emits a `session_id` key-value in its slog output for each run. Previously, `formatBufLine`
silently discarded this key. The fix adds:

```go
case "session_id":
    e.SessionID = val
```

The ID flows through the log pipeline: `BufLogEntry.SessionID` → `IssueLogEntry.SessionID` (JSON:
`sessionId`) → frontend `IssueLogEntry.sessionId`.

For on-disk sublogs read from `{sessionId}.jsonl` files, the session ID is derivable from the filename
and is stamped on all entries at read time.

### 3. Filtered `extractSubagents`

`extractSubagents(logs, filterSessionId?)` now accepts an optional session ID. When provided, it
pre-filters the log to entries where `!entry.sessionId || entry.sessionId === filterSessionId` before
scanning for subagent boundaries. The Timeline expands each run with its own `sessionId` as the filter.

---

## Alternatives considered

### Store per-run log files separately

Instead of a single flat file, write `{issueId}/{runIndex}.jsonl` (or `{sessionId}.jsonl` directly).

- **Pro**: isolation is physical; no filtering needed.
- **Con**: requires migrating existing log directories; symlinks or index files needed for "latest run"
  streaming; on-disk structure becomes more complex to explain to users.
- **Rejected**: the single-file approach is simpler and already deployed. Filtering in-memory at read
  time is fast enough for typical log volumes (< 10k lines per issue).

### Tag log entries with a run index instead of session ID

Use a monotonically incrementing `run_index` integer instead of the Claude Code session UUID.

- **Pro**: compact, human-readable.
- **Con**: requires the orchestrator to track and inject the run index into every log line — more
  coupling between the orchestrator and the log format. The `session_id` is already emitted by the
  Claude Code subprocess, so we get it for free.
- **Rejected**: using the existing `session_id` minimises new code.

---

## Consequences

### What becomes easier

- **Correct per-run subagent drill-down**: expanding any historical run in the Timeline shows exactly
  that run's subagents, with no bleed from earlier or later runs.
- **Session continuity indicator**: the daemon session badge (`Session abc12345`) lets users know
  whether a run was produced by the current process or a previous one.
- **Zero migration cost**: `AppSessionID` and `session_id` are optional fields (`omitempty`). Existing
  history entries and log files continue to work; they simply have no session stamp and are included
  in all filtered views (`!entry.sessionId` passes the filter).

### What becomes harder

- **Log format is now semi-structured**: the `session_id` field on log entries is meaningful for
  filtering but not surfaced in the UI directly. Changing the Claude Code slog key name would silently
  break per-run isolation without a test failure.
- **`AppSessionID` is in-memory only**: it is not persisted to the WORKFLOW.md history file, so after
  a restart the `AppSessionID` in history entries for prior runs will not match the new daemon session.
  This is intentional — it is a runtime grouping key, not a durable identifier.

---

## Related

- `internal/orchestrator/state.go` — `CompletedRun.AppSessionID`
- `internal/orchestrator/logging.go` — `formatBufLine` `case "session_id"`
- `internal/domain/types.go` — `BufLogEntry.SessionID`, `IssueLogEntry.SessionID`
- `internal/server/server.go` — `HistoryRow.AppSessionID`, `StateSnapshot.CurrentAppSessionID`
- `cmd/symphony/main.go` — `newAppSessionID()`, `buildSnapFunc`
- `web/src/pages/Timeline/index.tsx` — `extractSubagents` with `filterSessionId`
- ADR 001 — Single-Goroutine Orchestrator State Machine
