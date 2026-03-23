# Contributing to Symphony

Thank you for your interest in contributing. This document covers how to get the project running locally, how the codebase is structured, and the conventions used throughout.

---

## Development Setup

### Prerequisites

- Go 1.25.8
- `git`
- [Lefthook](https://github.com/evilmartians/lefthook) (`brew install lefthook` or `go install github.com/evilmartians/lefthook@latest`)
- [Claude Code CLI](https://claude.ai/code) (only needed for end-to-end manual testing)

### Clone and build

```bash
git clone https://github.com/vnovick/symphony-go
cd symphony-go
lefthook install        # wires pre-commit and pre-push hooks
go build ./...
go test -race ./...
```

All tests should pass with no external dependencies. Tests that hit real APIs are gated behind a build tag (see [Integration Tests](#integration-tests) below).

### Frontend setup (web dashboard)

The web dashboard is a Vite app that proxies API calls to a running `symphony` process. To work on the frontend you need two things running at the same time:

**Terminal 1 — build and run the Go binary from a project that has a `WORKFLOW.md`:**

```bash
# In symphony-go — build the binary
go build -o symphony ./cmd/symphony

# In your project repo (must have a WORKFLOW.md with server.port set)
/path/to/symphony-go/symphony   # picks up WORKFLOW.md in the current directory automatically
```

Make sure `server.port` is set in that project's `WORKFLOW.md`:

```yaml
server:
  port: 8090
```

**Terminal 2 — start the Vite dev server (in this repo):**

```bash
cd web
pnpm install --frozen-lockfile
pnpm dev     # HMR at http://localhost:5173, proxies /api/* to localhost:8090
```

`make dev` is a shortcut for the Vite step only. The Go binary must be running separately from a project directory.

### Make commands

| Command | Description |
|---|---|
| `make all` | `build` + `verify` — full build and check suite |
| `make build` | Build web dashboard then compile Go binary |
| `make verify` | `fmt` + `vet` + `lint-go` + `test` + `web-test` — mirrors CI |
| `make dev` | Start the Vite dev server with HMR at `localhost:5173` (run the Go binary separately — see [Frontend setup](#frontend-setup-web-dashboard)) |
| `make test` | `go test -race ./... -count=1` |
| `make coverage` | Run tests with coverage, output `coverage.html` |
| `make benchmark` | `go test -bench=. -benchmem ./...` |
| `make tui-golden` | Regenerate catwalk golden files after an intentional TUI render change |
| `make fmt` | `gofmt -l -w .` |
| `make vet` | `go vet ./...` |
| `make lint-go` | `golangci-lint run ./...` |
| `make web-build` | `pnpm install --frozen-lockfile && pnpm build` in `web/` |
| `make web-test` | `pnpm install --frozen-lockfile && pnpm test` in `web/` |
| `make clean` | Remove `symphony` binary and coverage files |

---

## Project Structure

```
symphony-go/
├── cmd/symphony/         # CLI entry point — wires all packages together
│   └── main.go
├── internal/
│   ├── workflow/         # WORKFLOW.md parser, content-hash file watcher, and `PatchIntField` for in-place field updates
│   ├── config/           # Typed config, defaults, $VAR resolution, validation
│   ├── domain/           # Shared types: Issue, BlockerRef
│   ├── tracker/          # Tracker interface + adapters
│   │   ├── tracker.go    # Tracker interface
│   │   ├── memory.go     # In-memory test adapter
│   │   ├── linear/       # Linear GraphQL adapter
│   │   └── github/       # GitHub REST adapter
│   ├── workspace/        # Per-issue workspace dirs, path safety, lifecycle hooks
│   ├── agent/            # claude subprocess runner, stream-json protocol
│   ├── orchestrator/     # Poll loop, state machine, retry queue, reconciliation
│   ├── prompt/           # Liquid template rendering
│   └── server/           # HTTP API + HTML dashboard (chi router)
├── testdata/workflows/   # WORKFLOW.md fixtures used by config/workflow tests
└── docs/                 # Design spec and implementation plan
```

### Protocol specification

The agent communication protocol is defined in the [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md). Refer to it when adding new agent backends or modifying the event stream format.

### Package dependency order

Packages only import packages below them in this list — no circular dependencies:

```
domain
  └── tracker (interface)
        └── tracker/linear, tracker/github, tracker/memory
workflow
  └── config
        └── workspace
              └── prompt
                    └── agent
                          └── orchestrator
                                └── server
                                      └── cmd/symphony
```

---

## Architecture

### Concurrency model

The orchestrator is a **single-goroutine state machine**. All state mutations go through one `select` loop — no mutexes are needed on the orchestrator's `State` struct.

Worker goroutines (one per running issue) send results back via a buffered `chan OrchestratorEvent`. The orchestrator processes these events in its main loop. This is the key invariant: **never mutate orchestrator state from a worker goroutine**.

```
                  ┌──────────────────────────────────┐
                  │         Orchestrator loop         │
                  │                                  │
  tick timer ───► │  select {                        │
  worker events ─►│    case <-tick.C:                │
  ctx.Done ──────►│    case ev := <-o.events:        │
                  │    case <-ctx.Done():             │
                  │  }                               │
                  └──────────────────────────────────┘
                            ▲              │
                 OrchestratorEvent        spawn goroutine
                            │              │
                  ┌─────────┴──────────────▼──────────┐
                  │        Worker goroutine            │
                  │  (one per running issue)           │
                  │  runs claude subprocess            │
                  │  sends EventWorkerExited on done   │
                  └───────────────────────────────────┘
```

### State is a value type

`orchestrator.State` is a struct, not a pointer. Functions in the orchestrator package take a `State` and return a new `State`. This makes the data flow explicit and makes tests straightforward — pass in state, assert on state out.

### Config reload

`workflow.Watch` polls `WORKFLOW.md` every 1 second using a content-hash stamp (mtime+size fast-path, SHA-256 fallback). When the file changes, `main.go` cancels the current run context, which gracefully shuts down the orchestrator and HTTP server, then restarts with fresh config. In-flight agent sessions are not interrupted — they exit naturally and their results are discarded.

---

## Testing

### Running tests

```bash
# All unit tests
go test ./...

# A single package
go test ./internal/orchestrator/...

# With race detector (recommended before submitting a PR)
go test -race ./...

# Verbose output
go test -v ./internal/orchestrator/...
```

### TUI tests (`internal/statusui`)

The status UI package uses two complementary testing strategies:

#### Pure-function and model-state tests

Unit tests in `model_test.go` (whitebox, same package) cover pure helper functions with no I/O — `wrapText`, `fmtDuration`, `fmtCount`, `truncate`, `extractPRLink`, `buildNavItems`, etc.

Interactive model-state tests in `model_teatest_test.go` use [`charmbracelet/x/exp/teatest`](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest) to drive the full `Update→View` pipeline: inject `tea.Msg` objects, then assert on the returned model struct:

```go
tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
tm.Send(tickMsg(time.Now()))
tm.Quit()
final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(Model)
assert.Len(t, final.sessions, 1)
```

#### Golden-file tests (catwalk)

Render-path tests in `model_catwalk_test.go` use [`knz/catwalk`](https://github.com/knz/catwalk) — a data-driven framework that records `View()` output into golden files under `internal/statusui/testdata/`. Each test file contains an input sequence (key events, resizes, custom messages) followed by the expected rendered output:

```
# Press 't' to activate the tool-call statistics view.
run
key t
----
-- view:
╔═[ SYM//PHONY ]═══...
║ AGENTS ▸ 0/5   ...
...
```

**When to regenerate golden files:** if you change the TUI layout or any render function, the catwalk tests will fail on `git push` (they run in the `pre-push` hook). Regenerate them with:

```bash
go test ./internal/statusui/... -args -rewrite
```

Then review the diff in `testdata/catwalk_*` — the diff is the exact render change — and commit alongside your code change.

**Custom updater commands** available in catwalk test files:

| Command | Effect |
|---|---|
| `tick` | Fires a `tickMsg`, syncing the snap function into the model |
| `picker-loaded` | Opens the project picker with two test projects (Alpha, Beta) |

#### Frontend tests

```bash
cd web
pnpm test              # run once
pnpm test:watch        # watch mode
pnpm test:coverage     # with lcov report
```

Tests use **Vitest** + **@testing-library/react**. All test files live next to the code they test under `__tests__/` directories.

### Test doubles

The project ships two test doubles that avoid real subprocesses and real API calls:

| Double | Package | Usage |
|---|---|---|
| `agent.FakeRunner` | `internal/agent` | Replays scripted `StreamEvent`s without spawning `claude` |
| `tracker.MemoryTracker` | `internal/tracker` | In-memory tracker with configurable issues and state |

Use these in orchestrator and integration-level tests. Never add a test that shells out to `claude` or hits a live API without a build tag.

### Integration tests

Tests that require live credentials are gated behind the `integration` build tag:

```bash
LINEAR_API_KEY=lin_api_... go test -tags integration ./internal/tracker/linear/...
GITHUB_TOKEN=ghp_...       go test -tags integration ./internal/tracker/github/...
```

These tests are skipped (not silently passed) in CI without credentials.

### Linting

**Go:**
```bash
golangci-lint run ./...
```

**Frontend:**
```bash
cd web
pnpm lint          # ESLint
pnpm format:check  # Prettier (check only)
pnpm format        # Prettier (write)
```

The CI pipeline enforces both. PRs with lint errors will not be merged.

### What to test

Each package has a corresponding `_test.go` file. When adding functionality, add tests in the same package. Coverage targets:

- **Happy path** — the normal flow works
- **Error path** — every `error` return has at least one test
- **Edge cases called out in the spec** — blockers, retry backoff cap, stall detection with `stall_timeout_ms=0`, etc.

For the orchestrator, prefer table-driven tests that build an initial `State`, call the function under test, and assert on the returned `State`.

---

## Code Conventions

### Error values

Return typed error strings for errors that callers may need to distinguish:

```go
return fmt.Errorf("linear_api_status: %d", resp.StatusCode)
```

The error string prefix (e.g. `linear_api_status`) is the stable error code. Avoid `errors.New` for errors that need context — wrap with `fmt.Errorf("...: %w", err)`.

### Logging

Use `log/slog` throughout. Always include structured fields rather than interpolating into the message:

```go
// good
slog.Warn("workspace hook failed", "hook", "before_run", "error", err, "issue_id", issue.ID)

// avoid
slog.Warn(fmt.Sprintf("before_run hook failed for %s: %v", issue.ID, err))
```

Never log API tokens or raw hook output beyond a safe truncation length.

### No globals

Avoid package-level mutable state. Configuration and dependencies flow through constructors. The orchestrator, server, and workspace manager all take their dependencies as constructor arguments.

### Comments

Add a doc comment to every exported type and function. Keep comments accurate — a wrong comment is worse than no comment.

---

## Making Changes

### Before you start

- Open an issue to discuss significant changes before writing code. This avoids wasted effort on directions that won't be merged.
- For bug fixes, a short description in the PR is sufficient.

### Branching

Work on a feature branch off `main`:

```bash
git checkout -b feat/my-feature
```

### Commit style

Use conventional commits:

```
feat(tracker): add GitHub Issues adapter
fix(orchestrator): cap backoff at max_retry_backoff_ms
docs: add WORKFLOW.md reference to README
test(workspace): cover symlink escape rejection
```

### Pull request checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] New behaviour is covered by tests
- [ ] No API tokens or secrets in the diff
- [ ] Exported symbols have doc comments
- [ ] PR description explains the **why**, not just the **what**

---

## Reporting Issues

Please include:

- Symphony version (`symphony --version` or the commit hash)
- Tracker kind (`linear` or `github`)
- A minimal `WORKFLOW.md` that reproduces the problem (redact API keys)
- The full error output or log lines

Open issues at: https://github.com/vnovick/symphony-go/issues

---

## Spec Conformance

Symphony aims for full conformance with the [Symphony spec](https://github.com/openai/symphony/blob/main/SPEC.md). The design document at `docs/superpowers/specs/2026-03-12-symphony-go-design.md` maps every spec section to the Go implementation. If you're adding a feature or fixing spec-divergent behaviour, note the relevant spec section in your PR.

The validation matrix in §17 of the spec maps directly to test cases in each package. New spec-required behaviours should have a corresponding test named after the behaviour they verify.
