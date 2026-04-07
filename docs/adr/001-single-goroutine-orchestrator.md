# ADR 001 — Single-Goroutine Orchestrator State Machine

**Status:** Accepted
**Date:** 2025-01-01
**Authors:** Itervox maintainers

---

## Context

Itervox's core job is to:

1. Poll a tracker (Linear or GitHub) for candidate issues on a configurable interval.
2. Claim issues and dispatch agent subprocesses (Claude Code, Codex) for each one.
3. Track per-issue state transitions (dispatched → running → terminal/retry) across multiple concurrent agents.
4. Serve live state snapshots to the HTTP dashboard and terminal UI.

The challenge is that dispatch state is *highly* interdependent: deciding whether to dispatch an issue requires knowing how many agents are already running, which issues are paused, which are blocked by dependencies, and what the current tracker state says. Mutating this state from multiple goroutines simultaneously would require coarse locking or complex coordination, and errors (double-dispatch, lost state transitions) are difficult to detect in tests.

Two design families were considered:

| Approach | Description |
|---|---|
| **Multi-goroutine with shared state** | Worker goroutines mutate dispatch state directly, guarded by a shared mutex or per-field mutexes. |
| **Single-goroutine event loop** | One goroutine owns all dispatch state. Other goroutines communicate changes by sending events on a buffered channel. |

---

## Decision

**All dispatch state is owned by a single event-loop goroutine.**

The `Orchestrator.Run` method enters a `select` loop that is the *only* place dispatch state (running set, paused set, retry queue, issue claim map) is read or written. Worker goroutines (one per dispatched issue) never touch this state directly — they post `OrchestratorEvent` values on the `o.events` channel when something noteworthy happens (agent exited, PR detected, state changed, etc.).

```
                    ┌─────────────────────────────────────┐
                    │         Event Loop Goroutine         │
                    │  (single goroutine, owns all state)  │
                    │                                      │
  ticker ──────────►│  select {                            │
  refresh ─────────►│    case e := <-o.events:             │
  events ──────────►│      handleEvent(e)                  │
                    │    case <-ticker.C:                   │
                    │      poll() → dispatch()             │
                    │    case <-refresh:                    │
                    │      poll() → dispatch()             │
                    │  }                                   │
                    └──────────────┬──────────────────────┘
                                   │ spawns
                      ┌────────────▼────────────┐
                      │   Worker Goroutine(s)    │
                      │  (one per running issue) │
                      │  sends events back via   │
                      │  o.events channel        │
                      └─────────────────────────┘
```

A small number of fields that are mutated from *outside* goroutines (HTTP handlers, tests) use their own narrow mutexes:

| Field | Mutex | Reason |
|---|---|---|
| Runtime-mutable `cfg.Agent.*` and `cfg.Tracker.*` fields (see CLAUDE.md for the exact list), plus `sshHostDescs` | `cfgMu` (RW) | HTTP handlers: `SetMaxWorkers`, profile/state/SSH-host changes |
| `lastSnap` | `snapMu` (RW) | Written by event loop, read by HTTP snapshot endpoint |
| `completedRuns`, `historyFile`, `historyKey` | `historyMu` (RW) | Written by event loop, read by HTTP history endpoint |
| `workerCancels` | `workerCancelsMu` | Written by event-loop dispatch, read by `cancelRunningWorker` from any goroutine |
| `userCancelledIDs` | `userCancelledMu` | Written by HTTP `CancelIssue`, read by event loop |
| `userTerminatedIDs` | `userTerminatedMu` | Written by HTTP `TerminateIssue`, read by event loop |
| `issueProfiles` | `issueProfilesMu` (RW) | Written by HTTP `SetIssueProfile`, read by event-loop dispatch and `Snapshot()` |
| `issueBackends` | `issueBackendsMu` (RW) | Written by HTTP `SetIssueBackend`, read by event-loop dispatch and `Snapshot()` |
| `pausedFile` | `pausedMu` (RW) | Persistence path for `PausedIdentifiers` |
| `inputRequiredFile` | `inputRequiredMu` (RW) | Persistence path for `InputRequiredIssues` |

The invariant: **all dispatch logic that decides whether to claim or transition an issue runs in the event loop goroutine and requires no lock.**

---

## Rationale

**Why not a shared-state approach with mutexes?**

- Dispatch decisions are compound: "is this issue already running AND not paused AND concurrency limit not reached AND no active blockers?" Atomically evaluating compound predicates over multiple fields requires either a single coarse lock (serialising everything, same throughput as a single goroutine) or careful acquisition ordering (deadlock risk, hard to verify in code review).
- Tests become harder: you must reason about interleaving to cover all state combinations.

**Why not a channel-per-state-field (actor model)?**

- Actors work well when state is partitioned. Dispatch state here is *unified*: the decision to dispatch issue A depends on the current count of running agents, the paused set, the retry queue, etc. Splitting those across actors reintroduces coordination.
- Go does not have an actor framework; building one adds abstraction overhead for little benefit at this scale.

**Why a single goroutine?**

- Dispatch throughput is bounded by the tracker poll interval (seconds), not CPU speed. A single goroutine is fast enough.
- The code is easy to reason about: there are no data races within the event loop, and Go's race detector validates this in CI.
- Testing is straightforward: inject events via `o.events <- Event{...}`, assert resulting state changes without needing to synchronise goroutines.

---

## Consequences

### What becomes easier

- **Correctness**: No dispatch-related data races. The race detector (`go test -race`) cleanly validates the design.
- **Testability**: The event loop is deterministic; tests can drive it by sending events and checking state without sleeps or sync primitives.
- **Readability**: All state transitions are in one place (`handleEvent`, `dispatch`, `poll`). A new contributor can trace the full lifecycle of an issue in a single file.

### What becomes harder

- **Blocking in the loop is forbidden.** Any blocking call inside `handleEvent` or `dispatch` stalls *all* issues — polls are delayed, no events are processed. All I/O (tracker HTTP calls, workspace ops, log writes) must happen in worker goroutines or short goroutines that post results back via `o.events`.
- **Large state is on one goroutine's stack.** If the running set grows to hundreds of issues, the single goroutine's iteration over it (e.g. building a snapshot) adds latency to event processing. The current limit of 50 concurrent agents (`SetMaxWorkers` clamp) keeps this bounded.
- **Adding new cross-cutting state** requires adding a new mutex if that state is written from outside the event loop. Follow the pattern: name the mutex after the field it guards, document which goroutines read and write it.

---

## Related

- `internal/orchestrator/orchestrator.go` — `Orchestrator` struct definition and all mutex annotations
- `internal/orchestrator/event_loop.go` — `Run` select loop and `handleEvent` implementation
- `internal/orchestrator/state.go` — `State` value type and `NewState`
- `internal/orchestrator/worker.go` — worker goroutines that post `EventWorkerExited` back via `o.events`
- `internal/orchestrator/orchestrator_test.go` — event-driven tests exercising the loop
- `internal/orchestrator/retry.go` — `BackoffMs` formula (documented in godoc)
