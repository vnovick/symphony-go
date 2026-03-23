# Changelog

All notable changes to Symphony Go are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [v0.0.3] — unreleased

### Added

#### Codex (OpenAI CLI) backend

| File | Change |
|------|--------|
| `internal/agent/codex.go` *(new)* | `CodexRunner.RunTurn` — spawns `codex` CLI, pipes stdout, delegates to `readLines` with `ParseCodexLine` |
| `internal/agent/codex_events.go` *(new)* | `ParseCodexLine` — parses Codex JSONL stream: `thread.started`, `item.started` (command_execution, collab_tool_call), `item.completed`, `turn.completed`, `turn.failed` |
| `internal/agent/multi.go` *(new)* | `MultiRunner` — selects Claude or Codex runner based on the active agent profile's `command` field |
| `internal/agent/events.go` | `StreamEvent.InProgress bool` — set `true` for `item.started` events to distinguish in-flight from completed tool calls |

#### Observability: action_started and action_detail log lines

| File | Change |
|------|--------|
| `internal/agent/claude.go` | `readLines`: emits `INFO <prefix>: action_started … tool=… description=…` when `ev.InProgress`; emits `INFO <prefix>: action_detail … tool=shell status=… exit_code=… output_size=…` for completed shell calls via new `logShellDetail()` |
| `internal/agent/claude.go` | `toolDescription("shell")`: appends ` (exit:N)` to description when exit code is non-zero |
| `internal/server/server.go` | `IssueLogEntry` gains `Detail string \`json:"detail,omitempty"\`` and `Time string \`json:"time,omitempty"\`` |
| `internal/server/handlers.go` | `parseLogLine`: new cases for `action_started` (→ `event:"action"` with `…` suffix) and `action_detail` (→ `event:"action"` with `Detail` JSON); both handle `claude:` and `codex:` prefixes |
| `internal/server/handlers.go` | `buildDetailJSON(status, exitCode, outputSize string) string` *(new)* — builds `{"status":…,"exit_code":…,"output_size":…}` omitting empty fields, using a typed struct for deterministic key order |
| `internal/statusui/model.go` | `colorLine`: `action_detail` case returns `""` (suppressed); `action_started` case renders gray `⧖ tool — desc…` |
| `web/src/types/symphony.ts` | `IssueLogEntry.time?: string`, `IssueLogEntry.detail?: string` |

#### Codex parity in TUI and web API

| File | Change |
|------|--------|
| `internal/statusui/model.go` | `colorLine`: `codex: text/action/subagent/todo/action_started` handled identically to `claude:` equivalents |
| `internal/statusui/model.go` | `buildToolStats`: `\|\| strings.HasPrefix(line, "INFO codex: action")` added; explicit early `action_detail` skip before generic `action` match |
| `internal/statusui/model.go` | `buildToolCalls`: same extensions as `buildToolStats` |
| `internal/server/handlers.go` | `parseLogLine`: `codex: text/subagent/action/todo` cases added mirroring `claude:` |
| `internal/server/handlers.go` | `skipLine`: `INFO codex: session started` and `INFO codex: turn done` added |

#### Named agent profiles

| File | Change |
|------|--------|
| `internal/config/config.go` | `AgentProfile{Command, Prompt, Backend}` struct; `Agent.Profiles map[string]AgentProfile` |
| `internal/orchestrator/orchestrator.go` | Profile lookup per issue; `MultiRunner` selected based on profile `Command`; profile prompt appended to rendered prompt |
| `internal/orchestrator/state.go` | `StateSnapshot.AvailableProfiles []string`, `ProfileDefs map[string]ProfileDef`, `AgentMode`, `ActiveStates`, `TerminalStates`, `CompletionState`, `BacklogStates` |
| `internal/server/handlers.go` | `/api/v1/settings` exposes `availableProfiles` and `profileDefs` |
| `internal/templates/workflow_github.md` | Profile section examples added |
| `internal/templates/workflow_linear.md` | Profile section examples added |
| `WORKFLOW.md` *(new)* | Root-level workflow template with profile definitions |
| `web/src/types/symphony.ts` | `ProfileDef` interface; `StateSnapshot.availableProfiles`, `profileDefs`, `agentMode`, `activeStates`, `terminalStates`, `completionState`, `backlogStates` |
| `web/src/pages/Settings/index.tsx` | Profile picker UI |
| `web/src/pages/Settings/profileCommands.ts` *(new)* | Per-profile agent command helpers |
| `web/src/hooks/useSettingsActions.ts` | Profile selection action |

#### Frontend: running sessions table

| File | Change |
|------|--------|
| `web/src/types/symphony.ts` | `RunningRow.backend string`, `HistoryRow.backend? string` |
| `web/src/components/symphony/RunningSessionsTable.tsx` | Backend column |
| `web/src/queries/issues.ts` | Backend field forwarded |

#### `symphony init --runner` flag

| File | Change |
|------|--------|
| `cmd/symphony/main.go` | `runInit`: new `--runner claude\|codex` flag (default: `claude`); `codex` emits `command: codex` + `backend: codex` in the generated WORKFLOW.md; runner is validated before file write |
| `cmd/symphony/main.go` | `generateWorkflow`: accepts `runner` parameter and emits the appropriate `agent:` block |
| `cmd/symphony/main.go` | `configuredBackend(command, explicit string)` *(new)* — resolves final backend string from agent command + explicit override |

#### Per-project log directory

| File | Change |
|------|--------|
| `cmd/symphony/main.go` | `--logs-dir` default changed from `./log` to `~/.simphony/logs/<tracker-kind>/<project-slug>`; new `defaultLogsDir(workflowPath string)` helper performs a lightweight early config read to derive the path; failures fall back to `~/.simphony/logs` |

#### Auto-clear workspace

| File | Change |
|------|--------|
| `internal/orchestrator/orchestrator.go` | `SetAutoClearWorkspaceCfg(enabled bool)` / `AutoClearWorkspaceCfg() bool` — toggle automatic workspace deletion after a task reaches completion state; safe to call from any goroutine (guards via `cfgMu`) |
| `internal/server/server.go` | `WorkspaceConfig.AutoClearWorkspace bool`; `setAutoClearWorkspace` callback + `SetAutoClearWorkspaceSetter` |
| `internal/server/handlers.go` | `POST /api/v1/settings/workspace/auto-clear` — persists the toggle back to WORKFLOW.md and notifies the orchestrator |
| `internal/workflow/loader.go` | `PatchWorkspaceBoolField(path, key string, enabled bool)` *(new)* — generic workspace-block bool patcher; backed by shared `patchBlockBoolField` with the existing `PatchAgentBoolField` |
| `web/src/pages/Settings/index.tsx` | Toggle switch "Auto-clear workspace on success" with description |
| `web/src/types/symphony.ts` (via `schemas.ts`) | `StateSnapshot.autoClearWorkspace?: boolean` |

#### Agent queue view

| File | Change |
|------|--------|
| `web/src/components/symphony/AgentQueueView.tsx` *(new)* | Drag-and-drop issue→agent-profile assignment board using `@dnd-kit/core`; columns per profile + "Unassigned"; dragging a card calls `onProfileChange` |
| `web/src/pages/Dashboard/index.tsx` | "◈ Agents" tab added to the board/list/agents toggle (visible when `availableProfiles.length > 0`); `AgentQueueView` rendered in agents tab |
| `web/src/pages/Dashboard/index.tsx` | Inline profile `<select>` in board and list views — per-issue assignment without opening the queue tab |

#### Git worktree mode (`workspace.worktree: true`)

| File | Change |
|------|--------|
| `internal/config/config.go` | `WorkspaceConfig.Worktree bool` — new field; defaults `false` (backward-compatible); loaded from `workspace.worktree` in WORKFLOW.md front-matter |
| `internal/workspace/worktree.go` *(new)* | `SlugifyIdentifier(id)` — lowercases, replaces non-alphanumeric chars with `-`, deduplicates; `ResolveWorktreeBranch(branchName, identifier)` — returns explicit branch > `symphony/<slug>` fallback, skips default branches (main/master/develop); `ensureWorktree` — `git worktree add -b <branch>`; retries without `-b` when branch already exists; `removeWorktree` — `git worktree remove --force` + `git worktree prune` + optional `git branch -D` |
| `internal/workspace/manager.go` | `EnsureWorkspace` / `RemoveWorkspace` gain a `branchName string` parameter; delegate to `ensureWorktree` / `removeWorktree` when `cfg.Workspace.Worktree = true`, otherwise fall back to original directory-per-issue behaviour |
| `internal/orchestrator/orchestrator.go` | `runWorker`: calls `workspace.ResolveWorktreeBranch` before `EnsureWorkspace`; `CheckoutBranch` step skipped when `worktreeMode = true` (branch already checked out by worktree); `auto_clear` goroutine passes resolved branch name to `RemoveWorkspace` |

#### Orchestrator & agent infrastructure

| File | Change |
|------|--------|
| `internal/orchestrator/orchestrator.go` | `cfgMu` RWMutex *(new)* — guards all config fields mutated at runtime from HTTP handler goroutines (`agentMode`, `maxConcurrentAgents`, `profiles`, tracker states, `autoClearWorkspace`); event loop reads these lock-free within a tick |
| `internal/orchestrator/orchestrator.go` | `SetHistoryKey(key string)` *(new)* — tags and filters history entries by `<kind>:<slug>`; entries with a different non-empty key are skipped on load, preventing cross-project history pollution |
| `internal/orchestrator/orchestrator.go` | `Snapshot()` merges live `issueProfiles` map (written concurrently by `SetIssueProfile`) into the snapshot overlay so board views see profile assignments without waiting for the next event-loop tick |
| `internal/agent/claude.go` | `ValidateClaudeCLI()` / `ValidateClaudeCLICommand(command string)` *(new)* — verify CLI availability on PATH with a 5-second timeout before spawning; `validateCLI(name, hint)` internal helper |

#### Tests

| File | Change |
|------|--------|
| `internal/agent/codex_test.go` *(new)* | `TestParseCodexLine_*` (all event types, errors, item.started variants); `TestCodexRunnerLogsActionStarted`; `TestCodexShellNonZeroExitInDescription`; `TestCodexShellDetailLoggedAtInfoLevel` |
| `internal/agent/validation_test.go` *(new)* | CLI path validation tests |
| `internal/agent/helpers_test.go` *(new)* | White-box tests for unexported helper functions in the agent package |
| `internal/agent/multi.go` *(new)* | `MultiRunner` unit tests |
| `internal/server/parse_test.go` *(new)* | 22 whitebox tests: `skipLine`, `parseLogLine` (text/action/subagent/action_started/action_detail, both backends), `buildDetailJSON`, `IssueLogEntry` JSON serialisation, time extraction, warn/error lines |
| `internal/statusui/model_test.go` *(new)* | 20+ whitebox tests: `colorLine` (all event types, both backends, lifecycle suppression), `buildToolStats`, `buildToolCalls`, `extractSubagents` |
| `internal/statusui/model_teatest_test.go` *(new)* | Teatest bubbletea Update→View pipeline tests |
| `internal/statusui/model_catwalk_test.go` *(new)* | Catwalk golden-file tests — drives full Update→View pipeline and diffs against `testdata/` snapshots |
| `internal/statusui/testdata/` *(new)* | Golden snapshots for catwalk tests (`catwalk_details`, `catwalk_picker`, `catwalk_tool_detail`, `catwalk_tools`) |
| `internal/orchestrator/subagents_internal_test.go` *(new)* | Subagent orchestration internal tests |
| `cmd/symphony/main_test.go` *(new)* | Main entry-point smoke tests |

---

### Fixed

| # | File | Bug | Fix |
|---|------|-----|-----|
| 1 | `internal/agent/claude.go` `logShellDetail` | Missing `tool=shell` kwarg — `action_detail` entries had empty `Tool` and messages like `" completed"` | Added `"tool", "shell"` to log args |
| 2 | `internal/server/handlers.go` `parseLogLine` | `"INFO claude: action_detail"` fell through to generic `action` case (prefix collision) | Added `\|\| strings.HasPrefix(line, "INFO claude: action_detail")` to the `action_detail` case |
| 3 | `internal/statusui/model.go` `buildToolStats`/`buildToolCalls` | `action_detail` lines (now with `tool=shell`) matched generic `"INFO codex: action"` prefix, double-counting shell calls | Added explicit early `action_detail` case that skips before the generic `action` match |
| 4 | `internal/agent/claude.go` `readLines` | `onProgress` callback fired for `InProgress` (item.started) events, causing spurious dashboard refresh churn | Added `&& !ev.InProgress` guard to the `onProgress` call |
| 5 | `internal/agent/runner.go` `ApplyEvent` | `EventAssistant` branch accumulated tokens/text for InProgress events (currently zero, but latent pollution risk) | Added `if ev.InProgress { break }` guard |
| 6 | `internal/orchestrator/orchestrator.go` `handleEvent` | Dashboard token counts reset to per-turn values at each turn boundary because `TurnResult` resets each call | Added `cumulativeInput`/`cumulativeOutput` vars in `runWorker`; both `onProgress` and end-of-turn update now send running totals |
| 7 | `internal/orchestrator/orchestrator.go` `handleEvent` | `EventDiscardComplete` used `select { default: }` — if the 64-slot events channel was full the event was silently dropped, permanently blocking issue dispatch | Replaced `default:` with `case <-ctx.Done():` + warn log |
| 8 | `internal/orchestrator/orchestrator.go` `sendExit` | `*o.runCtx.Load()` dereferenced without nil check — if called before `Run` stores the context, this panics | Nil-safe pattern: store channel in `orchDone`; nil receive channel blocks forever as safe fallback |
| 9 | `internal/agent/claude.go` `readLines` | Scanner goroutine blocked on `lineCh <-` indefinitely after outer function returned (SSH-hosted workers) | Added `done := make(chan struct{}); defer close(done)` with `select { case lineCh <- …: case <-done: }` in goroutine |
| 10 | `internal/orchestrator/orchestrator.go` `runAfterHook` | Called `workspace.RunHook` without `logFn` — `after_run` hook stdout/stderr was never forwarded to the per-issue log buffer | Added `identifier` parameter; passes `o.hookLogFn(identifier)` |
| 11 | `internal/server/handlers.go` `buildDetailJSON` | `map[string]any` produces non-deterministic JSON key order | Replaced with typed struct; Go marshals struct fields in declaration order |

- GitHub: `deriveState` now returns the configured state name (original casing) instead of the
  lowercased label — prevents duplicate Kanban columns (e.g. "In Progress" vs "in progress")
- GitHub: closed issues with no terminal label now fall back to `terminalStates[0]` instead of
  returning the literal string `"closed"`, which was not in `terminal_states`
- GitHub: `fetchPaginated` extraStates fallback uses configured state casing (`extra`) instead of
  lowercased label — fixes duplicate columns for backlog/completion states
- Paused→discard race: pressing D on a paused issue no longer immediately re-dispatches it.
  `DiscardingIdentifiers` blocks dispatch until the async `UpdateIssueState` goroutine completes
- Web UI: `parseLogLine` now handles `ERROR`-level log entries (previously silently dropped)
- TUI: `logBuf` entries added for `before_run hook failed` and `prompt render failed` paths so
  the TUI shows actual error reasons instead of a blank log
- `gofmt` violations in `state.go` and `client_test.go`
- README: license badge replaced with static badge (was failing due to GitHub license detection)

### OSS readiness

| Item | Details |
|------|---------|
| `SECURITY.md` | Responsible disclosure process, scope (API token exposure, workspace path traversal, HTTP API, prompt injection), and link to Symphony SPEC |
| `CONTRIBUTING.md` | Added Protocol Specification section linking to the [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) |
| `.env.example` | All required env vars documented with format hints (`LINEAR_API_KEY`, `GITHUB_TOKEN`, `SSH_KEY_PATH`) |
| `ErrorBoundary` component | Class component wrapping the app root — render crashes show a recovery UI instead of a blank screen |
| SSE exponential backoff | Reconnect delay increases `5s → 10s → 20s → 30s` (cap) instead of a flat 5 s retry |
| Go coverage gate | CI fails if total Go coverage drops below **50%** (`ci-go.yml`) |
| Frontend coverage thresholds | `vitest.config.ts` enforces ≥ 15% statement / ≥ 12% function coverage |
| Zod runtime API validation | `src/types/schemas.ts` validates all 4 API boundaries; `symphony.ts` re-exports inferred types for backward compatibility |
| Typed tracker errors | `tracker.APIStatusError`, `tracker.NotFoundError` (supports `errors.Is(err, tracker.ErrNotFound)`), `tracker.GraphQLError` in `internal/tracker/errors.go` |
| `BackoffMs` godoc | Full retry progression table (`10s → 20s → … → 300s cap`) with formula and rationale |
| `AgentConfig` timeout docs | Each of `turn_timeout_ms`, `read_timeout_ms`, `stall_timeout_ms` now has a doc comment explaining scope, default, and how it differs from the others |
| ADR 001 | `docs/adr/001-single-goroutine-orchestrator.md` — explains the event-loop model, invariants, and trade-offs vs channels/actors |
| Compatibility matrix | `docs/compatibility.md` — Go runtime, Claude Code / Codex CLI versions, Linear API, GitHub REST API (pinned `2022-11-28`), Node.js, OS support |
| Lint clean | Fixed `react-hooks/set-state-in-effect`, `react-hooks/refs`, `no-confusing-void-expression`, and `restrict-template-expressions` in `RunningSessionsTable` and `ErrorBoundary` |

### Changed

- Linear `WORKFLOW.md` template: `working_state: "In Progress"` enabled by default
- README: build-from-source instructions corrected to `pnpm 9+` (was inconsistently `pnpm 10+`)

---

## [0.1.0] - 2026-03-18

### Added

- Initial public release of Symphony Go
- Kanban web dashboard for real-time issue monitoring
- Terminal UI (TUI) with split-panel issue list and log viewer
- Linear and GitHub tracker integration
- Claude agent runner with SSH worker host support
- Agent profiles for per-issue command overrides
- Agent teams mode for multi-agent collaboration
- Timeline view for historical agent run review
- `symphony --version` flag
- `symphony init` command for WORKFLOW.md scaffolding
- `symphony clear` command for workspace cleanup
- CONTRIBUTING.md and CODE_OF_CONDUCT.md

### Security

- Removed `StrictHostKeyChecking=no` from SSH agent worker invocations;
  host key verification now uses `~/.ssh/known_hosts`
- Added HTTP server `ReadTimeout` (5s) and `IdleTimeout` (120s) to prevent
  connection exhaustion from slow or idle clients

---

## [v0.0.2] — 2025-03-xx

- Fix GitHub issues sync label duplication and refresh behaviour.
- Fix GitHub issues users loading bug.

## [v0.0.1] — initial release

- Linear + GitHub tracker integration, Claude Code agent runner, bubbletea TUI, REST API, web dashboard.

[Unreleased]: https://github.com/vnovick/symphony-go/compare/v0.0.2...HEAD
[0.0.2]: https://github.com/vnovick/symphony-go/compare/v0.1.0...v0.0.2
[0.1.0]: https://github.com/vnovick/symphony-go/releases/tag/v0.1.0
