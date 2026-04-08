---
name: breaking-change-gate
description: Use when `change-impact-review` surfaces a breaking change, OR when you're about to remove/rename an exported Go symbol, remove/rename a Zod schema field, change an HTTP route's shape, rename a WORKFLOW.md config field, or alter the semantics of a field without changing its name. This skill is a HARD STOP — no code changes until the user explicitly confirms.
---

# Breaking Change Gate

itervox is a Go daemon + React dashboard implementing the OpenAI Symphony spec. Its public surface (`WORKFLOW.md`, HTTP API, SSE events, JSON shapes) is depended on by existing users. Breaking changes must be explicit and confirmed, never accidental. This skill fires downstream of `change-impact-review` — read that skill first to understand how breakage gets detected.

## Hard stop

When this skill fires, **do not edit code**. The first action is to surface the breakage and obtain explicit user confirmation. A breaking change made without confirmation is a bug even if the code compiles.

## Step 1 — Classify

Identify which category the breakage falls into:

- **A. API breakage** — exported Go symbol removed/renamed/signature-changed; HTTP route removed/renamed/body shape changed; SSE event removed/renamed/data shape changed.
- **B. Schema breakage** — Zod field or Go JSON field removed, renamed, or type-narrowed. Breaks the daemon↔dashboard serialization contract.
- **C. Config breakage** — `WORKFLOW.md` field removed, renamed, or semantics changed. Existing user config files break on next restart.
- **D. State machine breakage** — `orchestrator.State` field removed/renamed, `cfgMu` guard list changed without updating `CLAUDE.md`, or lock ordering changed.
- **E. Behavioral breakage** — API surface unchanged but observable behavior differs (e.g., tracker poll now includes terminal states).

## Step 2 — Try non-breaking escape hatches first

Before concluding a change MUST be breaking, check:

- **Go**: keep the old name as a type alias (`type OldName = NewName`) or a wrapper function that delegates, with a deprecation comment. Add new fields alongside old ones.
- **Zod**: mark the old field `.optional()` and add the new field, writing to both during the transition. Use `.transform()` to adapt old data to new shape.
- **HTTP**: add a new route, deprecate the old one, leave both live for one release.
- **Config**: accept both old and new `WORKFLOW.md` field names — check the new key first, fall back to the old, log a deprecation warning.
- **SSE**: add a new event type alongside the old one and switch the client to prefer the new. Old subscribers keep working.

Nine times out of ten one of these avoids the breakage entirely for the cost of temporary ugliness. That's an acceptable trade.

## Step 3 — Produce the breakage report

Present this exact format to the user in a single markdown block:

```markdown
## Breaking Change Proposal

**Summary**: <one sentence describing what breaks>

**Category**: <A/B/C/D/E>

**What exists today** (before):
<code block or description>

**What will exist after** (after):
<code block or description>

**Who breaks**:
- <every consumer identified in change-impact-review>
- <external consumers too: existing user WORKFLOW.md files, dashboard bookmarks, etc.>

**Migration path** (required):
- Option 1: <concrete steps users must take>
- Option 2 (if non-trivial): <alternative path>
- Automated migration possible? <yes/no — if yes, describe>

**Alternatives considered** (required — at least two):
- Alternative A: keep the old name as a deprecated alias for one release cycle. Tradeoff: <tradeoff>
- Alternative B: add the new name alongside the old one, no removal. Tradeoff: <tradeoff>
- Alternative C: <the proposed breaking change>. Tradeoff: <tradeoff>

**Recommended**: <one of A/B/C with rationale>

**Questions for you**:
1. <specific question the user must answer>
2. <specific question the user must answer>
```

## Step 4 — Wait for explicit confirmation

The user must respond with one of:

- "proceed with the breaking change" (or equivalent clear approval)
- "go with alternative X" (A or B from the report)
- "don't break this — find another way"

Ambiguous responses like "sure, sounds good" are **not sufficient**. Re-ask as a yes/no question if the response is unclear.

## Step 5 — Post-confirmation

Once confirmed:

1. Implement the change normally.
2. Add an entry under a **Breaking changes** heading in release notes / changelog.
3. Flag the breakage explicitly in the PR description so reviewers can't miss it.
4. Use a Conventional Commits `BREAKING CHANGE:` footer in the commit message so `git log` reflects it.

## Common mistakes to prevent

- Treating a rename as "just a refactor" when the old name was exported or part of a schema.
- Assuming "no one uses this" without running `change-impact-review` first.
- Conflating "I don't like the old name" with "the old name must be removed".
- Shipping a breaking change without a migration path.

## Do NOT use this skill for

- Internal refactors (unexported helpers, file splits).
- Adding new fields (additive, non-breaking).
- Adding new routes or new schemas.
- Bug fixes that restore documented behavior.
