# AGENTS.md — symphony-go

> This file provides context for AI coding agents (Codex, Claude Code, etc.) working on this repo.
> For human contributor docs see CONTRIBUTING.md.

## Project overview

Symphony Go is a Go 1.25.8 daemon that polls Linear/GitHub Issues, spawns Claude Code or
Codex subagents per issue, and serves a React web dashboard + Bubbletea TUI.
Config is a single `WORKFLOW.md` file (YAML front matter + Liquid template).

## Before making any change

1. **Read CLAUDE.md** — it contains architecture invariants, false-positive patterns for
   static analysis, and conventions that override defaults.
2. **Run tests** to establish a baseline: `go test -race ./...` and `cd web && pnpm test`.
3. **Check the gap doc** (`docs/gaps2_240326.md`) for known open items before adding new
   ones — it may already be tracked.

## Build commands

```bash
# Go
go build ./...
go test -race ./...
go vet ./...
golangci-lint run ./...

# Frontend
cd web
pnpm install --frozen-lockfile
pnpm test          # vitest
pnpm build         # production bundle
pnpm exec tsc --noEmit -p tsconfig.app.json   # type-check only

# Combined
make verify        # fmt + vet + lint + go tests + web tests
make build         # web build → go binary
```

## Repository layout

```
cmd/symphony/        CLI entry — wires all packages; main.go + main_test.go
internal/
  agent/             Claude/Codex subprocess runner (stream-json protocol)
  config/            Typed config, defaults, $VAR resolution, validation
  domain/            Shared types: Issue, BlockerRef, BufLogEntry
  logbuffer/         Ring buffer for per-issue log streaming
  orchestrator/      Single-goroutine state machine
    orchestrator.go  Struct, New, Load, event loop (~2100 lines — split planned)
    dispatch.go      Eligibility checks, slot calculation
    reconcile.go     Stall/state reconciliation helpers
    retry.go         Retry queue scheduling
    state.go         OrchestratorEvent types and RunEntry
  prdetector/        PR URL detection via `gh pr list`
  prompt/            Liquid template rendering
  server/            HTTP API (chi router) — REST + SSE
  statusui/          Bubbletea TUI model and golden-file tests
  tracker/           Tracker interface + Linear GraphQL + GitHub REST adapters
  workflow/          WORKFLOW.md parser and file watcher
  workspace/         Per-issue worktree lifecycle
web/                 React/Vite frontend (see web/src/README if present)
testdata/            WORKFLOW.md fixtures
docs/                Gap analysis docs
```

## Architecture constraints

### Orchestrator event loop — single goroutine

The orchestrator `Run()` loop is the ONLY place that mutates `State`. Workers
communicate via `o.events chan OrchestratorEvent`. Never write to state from a
worker goroutine — send an event instead.

### cfgMu scope

`cfgMu` protects only these `cfg` fields (mutable at runtime via HTTP):
- `cfg.Agent.AgentMode`, `cfg.Agent.MaxConcurrentAgents`, `cfg.Agent.Profiles`
- `cfg.Tracker.ActiveStates`, `cfg.Tracker.TerminalStates`, `cfg.Tracker.CompletionState`
- `cfg.Workspace.AutoClearWorkspace`

All other `cfg` fields are **read-only after startup** — no lock needed.

### Config value validation

`positiveIntField` in `config.go` rejects zero and negative values, replacing them
with defaults. Timeout fields (`TurnTimeoutMs`, `ReadTimeoutMs`, etc.) can never be
0 at runtime — do not flag `context.WithTimeout(ctx, 0)` as reachable.

### Package import order (no circular deps)

```
domain → tracker → (workflow, config) → workspace → prompt → agent
       → orchestrator → server → cmd/symphony
```

## Testing conventions

- Always run `go test -race` — the race detector catches real bugs here
- TUI tests use `charmbracelet/x/exp/teatest` (`model_teatest_test.go`) + catwalk
  golden files. Regenerate golden files with `make tui-golden` after intentional
  render changes.
- Integration tests (real API calls) are gated behind a build tag — not run by default.
- Frontend tests use Vitest + Testing Library.

## Common pitfalls

- **Toast API**: `addToast(message: string, variant?)` — first arg is a string.
  Passing an object silently renders `[object Object]`.
- **Settings mutations** must call `refreshSnapshot()`, NOT `patchSnapshot()`.
- **SSE hooks**: always use `useToastStore.getState()` / `useSymphonyStore.getState()`
  inside effects — never call hooks conditionally.
- **Map copy**: use `maps.Copy(dst, src)` not manual for-range loops.
- **Clamp pattern**: `max(1, min(n, 50))` not if-chains (Go 1.21+).

## Open architectural items (from docs/gaps2_240326.md)

Highest priority unresolved issues:
- GO-R11-1: `ReconcileTrackerStates` reads `TerminalStates` without `cfgMu` (race)
- GO-R11-3: `copyRunningMap` shallow copies `*RunEntry` — concurrent mutation risk
- GO-R11-4: Reviewer goroutines untracked — no WaitGroup, no slot limit
- FE-R11-10: `ReactQueryDevtools` renders unconditionally in production (+60 kB)
- WIRE-2/5: Linear project filter — 3 backend routes + TUI complete, zero web UI

Before adding new items to the doc, spawn a verification agent to confirm the
issue is real (read full call chain, check for upstream validation, verify file
exists). See the "Gap analysis — avoiding false positives" section of CLAUDE.md.
