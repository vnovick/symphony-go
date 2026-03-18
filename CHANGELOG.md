# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/vnovick/symphony-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vnovick/symphony-go/releases/tag/v0.1.0
