# Changelog

All notable changes to Itervox are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [0.1.4] — unreleased

### Changed

- Input-required tracker comments are human-facing again: Itervox now persists pending resume metadata locally instead of embedding session, host, backend, or command details in tracker comments.
- Snapshot and dashboard state now distinguish `pending_input_resume` from `input_required`, so “reply received, waiting to resume” is surfaced as a separate live state instead of being inferred as plain waiting-for-input.
- Reviewer settings are now validated consistently: `agent.auto_review` requires `reviewer_profile`, and `workspace.auto_clear` cannot be enabled together with automatic review.
- The TUI now surfaces input-related issues directly, including both `input_required` and `pending_input_resume`, while keeping replies in the tracker or web dashboard.

### Fixed

- Input-required resume now continues the existing Claude or Codex session with the actual user reply, instead of re-entering a fresh-dispatch path.
- Input-required resume now reuses the existing workspace and skips setup steps that could reset repo state, including PR detection, branch checkout, and `before_run`.
- Input-required resume now persists the exact tracker question comment ID and author identity locally, so tracker replies and dashboard replies can both resume the same saved session, backend, command, branch, profile, and SSH host after restart.
- Input-required resume now reruns fresh-dispatch setup only when the original workspace is gone and had to be recreated, instead of resuming into an uninitialized checkout.
- Pending input replies now survive early resumed-worker failures until the resumed run actually makes progress, instead of being discarded on any worker exit.
- Input-required persistence now writes atomically, reducing the chance of losing waiting/pending resume state on interruption.
- Successful turns that end with a real blocking question or confirmation request now enter `input_required` via a deterministic fallback detector, even when the agent omitted the explicit `<!-- itervox:needs-input -->` marker.
- Codex sessions that request user input now enter `input_required` correctly instead of falling through the single-turn success path.
- Resume command resolution now preserves Codex backends when the saved entry has no explicit command, including backend-only profile setups.
- Claude resume invocations now append `-p <reply>` when a resumed turn needs to send fresh user input.
- Agent command resolution now recognizes zsh alias output in addition to direct paths and bash-style aliases.
- The SSE/query invalidation bridge now reacts when an issue transitions from `input_required` to `pending_input_resume`, avoiding stale issue detail and board views after a reply is accepted.
- The dashboard header and logs view now reflect pending resumes explicitly instead of showing `idle`, and the logs sidebar preserves visible live issues even before the first log line exists.
- The logs view restores branch/profile/host context for selected issues.
- The reviewer settings card now preserves pending edits when a save fails, and workspace-reset actions refresh snapshot/log state on success.
- Opening a browser from the TUI now isolates the child process and ignores `SIGTTOU`/`SIGTTIN`, preventing terminal freezes after `open`/`xdg-open`.

### Tests

- Added end-to-end orchestrator coverage for input-required resume, including workspace continuity, saved session reuse, and `before_run` suppression.
- Added restart/recovery coverage for locally persisted input-required sessions, including tracker replies detected after restart and exact saved session/backend/host reuse.
- Added coverage for pending input resumes that survive retryable worker failures until progress is observed.
- Added Codex-specific resume regression coverage to assert saved session ID reuse, exact user-reply forwarding, and preserved command selection.
- Added coverage for plain-English blocking questions so successful turns that ask the user to choose or confirm are queued for `input_required`.
- Added frontend regression coverage for pending-resume snapshot rows, snapshot invalidation fingerprints, app-header state, and logs-page rendering.
- Tightened manual pause/resume tests to assert the resumed prompt, and added direct argument coverage for Claude and Codex resume flows.

### Documentation

- Documented the updated human-input contract in the README, generated `WORKFLOW.md` guidance, and site docs: the explicit `<!-- itervox:needs-input -->` marker remains preferred, with the deterministic fallback acting as backup behavior for plain-English blocking questions and its English-oriented limitation now called out explicitly.
- Documented the `auto_review` / `workspace.auto_clear` guardrail, the requirement that `auto_review` be paired with `reviewer_profile`, and the current lifecycle-hook semantics more precisely in the README, generated workflow template comments, and site docs.

## [0.1.3] — 2026-04-08

### Added

- Codex backend support, including JSONL stream parsing, backend-aware runner selection, and Codex log/event parity across the web UI and TUI.
- Named agent profiles with per-issue assignment, profile-aware settings payloads, backend visibility in session tables, and an agent queue view.
- Per-run log isolation using daemon app-session IDs and stamped agent session IDs, so Timeline expansions show only logs from the selected run.
- Git worktree mode, auto-clear workspace support, project-scoped default log directories, `.env` loading, and `itervox init --runner`.
- Fast single-issue fetch support, typed tracker/workflow errors, Claude CLI validation helpers, and broader automated coverage across agent, server, TUI, and orchestrator paths.

### Changed

- `server.New` now takes a validated `server.Config` instead of relying on positional arguments and late setter injection.
- Settings snapshots now expose profile definitions, backend metadata, tracker state configuration, and auto-clear workspace state.
- Workspace management can resolve issue-specific worktree branches and skips manual branch checkout when worktree mode is enabled.
- The Linear workflow template now defaults `working_state` to `"In Progress"`.

### Fixed

- Action logging is now consistent across Claude and Codex: `action_started` and `action_detail` parse correctly, shell metadata is preserved, and duplicate tool counts are avoided.
- Dashboard token accounting now remains cumulative across turns instead of resetting at turn boundaries.
- Orchestrator lifecycle bugs were fixed around paused-to-discard races, event-channel exit handling, scanner goroutine shutdown, nil-safe exit handling, and after-run hook log forwarding.
- GitHub tracker state casing and fallback behavior no longer create duplicate columns or invalid terminal-state labels.
- Web and TUI log views now surface worker errors more reliably, including `ERROR` lines and hook/prompt failures.
- SSH worker execution now allocates a PTY so remote agent processes receive `SIGHUP` and do not become orphaned.

### Documentation

- Expanded project docs with `SECURITY.md`, `CONTRIBUTING.md`, `.env.example`, compatibility notes, ADRs, and dashboard design documentation.

## [0.1.0] — 2026-03-18

### Added

- Initial public release of Itervox.
- Kanban web dashboard for real-time issue monitoring.
- Terminal UI (TUI) with split-panel issue list and log viewer.
- Linear and GitHub tracker integration.
- Claude agent runner with SSH worker host support.
- Agent profiles for per-issue command overrides.
- Agent teams mode for multi-agent collaboration.
- Timeline view for historical agent run review.
- `itervox --version` flag.
- `itervox init` command for `WORKFLOW.md` scaffolding.
- `itervox clear` command for workspace cleanup.
- `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md`.

### Security

- Removed `StrictHostKeyChecking=no` from SSH agent worker invocations; host key verification now uses `~/.ssh/known_hosts`.
- Added HTTP server `ReadTimeout` (5s) and `IdleTimeout` (120s) to prevent connection exhaustion from slow or idle clients.

## [0.0.2] — 2025-03-xx

### Fixed

- GitHub issues sync label duplication and refresh behavior.
- GitHub issues users loading bug.

## [0.0.1] — initial release

### Added

- Linear and GitHub tracker integration.
- Claude Code agent runner.
- Bubbletea TUI.
- REST API.
- Web dashboard.

[Unreleased]: https://github.com/vnovick/itervox-go/compare/v0.1.3...HEAD
[0.1.4]: https://github.com/vnovick/itervox-go/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/vnovick/itervox-go/compare/v0.1.0...v0.1.3
[0.1.0]: https://github.com/vnovick/itervox-go/compare/v0.0.2...v0.1.0
[0.0.2]: https://github.com/vnovick/itervox-go/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/vnovick/itervox-go/releases/tag/v0.0.1
