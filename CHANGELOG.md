# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.2] - 2026-03-19

### Fixed
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

### Changed
- Linear `WORKFLOW.md` template: `working_state: "In Progress"` enabled by default

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

[Unreleased]: https://github.com/vnovick/symphony-go/compare/v0.0.2...HEAD
[0.0.2]: https://github.com/vnovick/symphony-go/compare/v0.1.0...v0.0.2
[0.1.0]: https://github.com/vnovick/symphony-go/releases/tag/v0.1.0
