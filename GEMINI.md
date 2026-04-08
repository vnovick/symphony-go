# Gemini CLI instructions for itervox

This file is the entry point for Gemini CLI sessions. The actual rules live in `AGENTS.md` at the repo root and in `CLAUDE.md` — read both before making changes. The content applies to Gemini CLI the same way it applies to every other coding agent working on this codebase.

## Start here

1. Read `CLAUDE.md` at the repo root — architecture invariants, package dependency order, frontend architecture, hard rules.
2. Read `AGENTS.md` at the repo root — index of rule bundles and quick-reference commands.
3. Before editing a specific area, read the matching rule bundle under `.claude/skills/<name>/SKILL.md`:

| Trigger | Rule bundle |
|---|---|
| `internal/orchestrator/**/*.go`, concurrent code, mutable `cfg.*` fields | `.claude/skills/orchestrator-invariants/SKILL.md` |
| `web/src/**/*.{ts,tsx}` that uses HTTP or SSE (outside `web/src/auth/`) | `.claude/skills/authed-transport/SKILL.md` |
| `web/src/components/**` / `web/src/pages/**` — React component work | `.claude/skills/react-component-discipline/SKILL.md` |
| New Go file, file past ~400 lines, new helper, new package | `.claude/skills/go-package-hygiene/SKILL.md` |
| Adding/renaming fields in `internal/config/config.go` | `.claude/skills/config-field-checklist/SKILL.md` |
| Before editing any exported symbol / HTTP route / SSE / Zod schema | `.claude/skills/change-impact-review/SKILL.md` |
| When a change is breaking (hard stop — requires user confirmation) | `.claude/skills/breaking-change-gate/SKILL.md` |
| `go.mod`, `Makefile`, Go toolchain bumps, govulncheck findings | `.claude/skills/go-toolchain-sync/SKILL.md` |
| Before claiming complete, before committing, before PR | `.claude/skills/verify-before-done/SKILL.md` |

## Commands

- `/interview` (`.claude/commands/interview.md`) — 8-question structured interview at the start of a feature
- `/brainstorm` (`.claude/commands/brainstorm.md`) — 3-subagent design debate with tradeoffs table

The directory name `.claude/skills/` is historical — the content is plain markdown and tool-agnostic. Read these files directly; do not skip them.

## Verification gate

Before claiming any change is complete, run `make verify` from the repo root and confirm exit code 0. If you touched frontend code, also run `cd web && pnpm test:coverage` and confirm all four coverage axes are at or above 70%. If you bumped Go dependencies, also run `govulncheck -tags dev ./...`.

See `AGENTS.md` for the full command reference and hard rules.
