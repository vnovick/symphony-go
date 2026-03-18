# Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                        symphony                          │
│                                                          │
│  ┌──────────┐   poll    ┌──────────────┐                │
│  │  Tracker │◄─────────│ Orchestrator │                 │
│  │ (Linear/ │           │  event loop  │                 │
│  │  GitHub) │           │  (1 goroutine│                 │
│  └──────────┘           │   owns state)│                 │
│                          └──────┬───────┘                │
│                                 │ dispatch               │
│                          ┌──────▼──────┐                 │
│                          │   Workers   │ (N goroutines)  │
│                          │  agent.Runner│                │
│                          │  (claude CLI)│                │
│                          └─────────────┘                 │
│                                                          │
│  ┌──────────┐  HTTP/SSE  ┌──────────────┐               │
│  │  Web UI  │◄──────────│  HTTP server │                │
│  │ (React)  │           │  /api/v1/...  │               │
│  └──────────┘            └──────────────┘               │
│                                                          │
│  ┌──────────┐                                            │
│  │  TUI     │  (bubbletea, reads Orchestrator snapshot)  │
│  └──────────┘                                            │
└─────────────────────────────────────────────────────────┘
```

## Key design principles

### Single-goroutine event loop

`Orchestrator.Run()` owns all dispatch state (which issues are running, paused, retrying) in a single goroutine. Every mutation to that state flows through a buffered `events` channel. This eliminates the need for locks on the core state machine and makes state transitions easy to reason about.

Worker goroutines (one per running issue) send `EventWorkerExited` back through the channel when done. HTTP handler goroutines send `EventResumeIssue`, `EventTerminatePaused`, etc.

### Config field guards

A small subset of `config.Config` fields are mutable at runtime (via the web dashboard):
- `Agent.AgentMode`, `Agent.Profiles`
- `Tracker.ActiveStates`, `TerminalStates`, `CompletionState`

These are protected by `Orchestrator.cfgMu` (`sync.RWMutex`). All other config fields are read-only after startup.

### Workspace isolation

Each issue gets a dedicated workspace directory under `~/.simphony/workspaces/<identifier>/`. The `workspace.Manager` ensures the directory exists before the agent is invoked and enforces that the agent cannot escape to parent directories (`workspace.Safety`).

### Tracker abstraction

Both Linear and GitHub implement the `tracker.Tracker` interface. The orchestrator is tracker-agnostic — it works with `domain.Issue` values regardless of origin.

## Request flow (web dashboard → agent dispatch)

```
Browser POST /api/v1/issues/:id/resume
  → server.Handler (HTTP goroutine)
  → orch.ResumeIssue()
  → o.events <- EventResumeIssue      (non-blocking send)
  → Orchestrator event loop receives
  → removes from PausedIdentifiers
  → next poll tick dispatches the issue
  → go runWorker(workerCtx, issue, ...)
  → agent.Runner.RunTurn() (spawns claude subprocess)
  → streams JSON events back
  → EventWorkerExited sent to o.events
  → state updated, snapshot stored
  → OnStateChange() → SSE broadcast to browser
```

## Packages

| Package | Responsibility |
|---|---|
| `cmd/symphony` | Entry point, wires all components |
| `internal/orchestrator` | Core event loop, dispatch, state machine |
| `internal/agent` | Runner interface, claude subprocess, stream parsing |
| `internal/tracker/linear` | Linear GraphQL client |
| `internal/tracker/github` | GitHub REST client |
| `internal/config` | WORKFLOW.md loading, validation, env var resolution |
| `internal/server` | HTTP API handlers, SSE broadcast |
| `internal/statusui` | Terminal UI (bubbletea) |
| `internal/workspace` | Workspace directory management, path safety |
| `internal/logbuffer` | Per-issue ring buffer for live log streaming |
| `internal/prompt` | Liquid template rendering for agent prompts |
| `internal/workflow` | WORKFLOW.md patch helpers (in-place YAML edits) |
| `web/` | React + Vite dashboard (served as embedded FS) |
