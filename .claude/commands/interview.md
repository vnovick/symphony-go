---
name: interview
description: Interview the developer with 8 structured questions that surface design intent, blast radius, and verification criteria BEFORE any code is written. Use at the start of a new feature, refactor, or non-trivial change when the request is vague or the scope is unclear.
---

# /interview

You are about to interview the developer. The goal is to extract the design intent, scope, and verification criteria *before* any code is written, so that implementation is grounded in explicit answers rather than guesses.

Ask the 8 questions below **one at a time**, in order. After each answer, acknowledge briefly (one sentence) and move on. Do not start answering or coding until all 8 questions have been asked and the developer has confirmed the summary at the end.

## The 8 questions

1. **What user-visible behavior changes?** State it as a sentence that starts with "After this change, a user will be able to…" or "After this change, the dashboard will…". If you can't express it that way, the scope isn't concrete enough yet.

2. **Which existing files will you touch, and which new files will you create?** List them. File-level planning before code.

3. **What's the smallest version that ships?** Name the minimum viable implementation. Everything beyond that is scope creep to defer to a follow-up.

4. **What in `cfg`, `orchestrator.State`, SSE event shape, or Zod schemas does this touch?** This is the cross-boundary surface that itervox cares about most (see `.claude/skills/change-impact-review/SKILL.md`). If the answer is "nothing", the change is purely internal.

5. **What could regress? Name two existing features that share code paths with what you're changing.** Forces blast-radius thinking. If the developer can't name two, the change is risky OR the developer doesn't yet understand the code area well enough to proceed.

6. **What's the first test you'd write to prove it works?** Forces falsifiability. Tests-first thinking without necessarily mandating TDD.

7. **Is there a `WORKFLOW.md` or tracker-state implication?** itervox-specific. A new config field? A new state transition? A change to how Linear / GitHub issues flow through the board?

8. **What would make you abandon this approach mid-implementation?** Extracts hidden assumptions. If the developer says "nothing", they haven't thought about failure modes yet — follow up with "what assumption are you making that, if wrong, would kill this approach?"

## After the 8 questions

Present a compact summary in this format:

```markdown
## Interview Summary

**Goal**: <from Q1>
**Scope (files)**: <from Q2>
**MVP**: <from Q3>
**Cross-boundary surface**: <from Q4>
**Regression risk**: <from Q5>
**First test**: <from Q6>
**Config/tracker implication**: <from Q7>
**Abandonment signal**: <from Q8>
```

Ask: "Does this match your intent? Any corrections before I start?"

Wait for confirmation (or corrections applied into an updated summary). Only then proceed to implementation planning or coding.

## When to use this command

- New feature with unclear scope
- Refactor touching multiple packages
- User asks "can you add X" where X is one sentence
- Any change where the developer's first message is under 2 sentences

## When NOT to use it

- Bug fix with a clear failing test
- Typo or documentation correction
- Trivial one-line changes
- When the developer has already written a design doc or detailed plan — reading that is faster than interviewing
