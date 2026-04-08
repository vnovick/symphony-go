---
name: brainstorm
description: Explore design alternatives for a feature or refactor by spawning 3 subagents with forced orthogonal positions (Minimalist, Architect, Pragmatist). Each argues its position, then produces a single tradeoffs table and decision document. Use when multiple reasonable approaches exist and the "right" one isn't obvious.
---

# /brainstorm

You are about to facilitate a 3-way design debate. The goal is to surface tradeoffs that would otherwise stay implicit, by forcing three subagents into distinct positions and making them argue. Mush is the enemy — unanimous agreement after 10 seconds means you didn't force enough disagreement.

## Input

The developer's feature request or design question is the input. If it's ambiguous, ask ONE clarifying question first, then proceed.

## Mechanics

Spawn **three subagents in parallel**, one message with three tool calls. Each gets a different forced position. Do not let them converge — the forced orthogonality is the feature.

### Subagent 1 — MINIMALIST

**Forced position**: ship the smallest diff that delivers user-visible value. Defer abstractions until the second use case appears. Accept duplication. Optimize for reversibility — the option that's easiest to undo in 6 weeks is the winning option. "YAGNI" is the dominant heuristic. Willing to push back against the user's scope.

Prompt the subagent with:
- The developer's request (verbatim)
- The relevant repo context (file paths, existing patterns)
- Instruction: "Argue the smallest-possible-change approach. Propose the implementation. Call out what you are deliberately NOT doing and why. Under 400 words."

### Subagent 2 — ARCHITECT

**Forced position**: optimize for 6-month maintainability and long-term correctness. Extract shared infrastructure now, while the context is loaded. Accept upfront cost in exchange for reduced future churn. Willing to block on refactors that unblock multiple future features. "Pay it once, not many times."

Prompt the subagent with:
- The developer's request (verbatim)
- The relevant repo context
- Instruction: "Argue the highest-quality-long-term approach. Propose the implementation. Call out the debt you are removing and the invariants you are strengthening. Under 400 words."

### Subagent 3 — PRAGMATIST / USER ADVOCATE

**Forced position**: optimize for what the user actually notices. Challenge scope that doesn't move user-visible metrics. Ask "would the user care if this didn't ship?" Willing to kill features that exist for code-hygiene reasons only. Prioritizes shipping speed and falsifiable value.

Prompt the subagent with:
- The developer's request (verbatim)
- The relevant repo context
- Instruction: "Argue the maximum-user-value approach. Propose the implementation. Call out the parts of the request the user probably doesn't need. Under 400 words."

## Synthesizing the three responses

After all three return, you produce the output. The output is a SINGLE markdown block with exactly this structure:

```markdown
# Brainstorm: <one-line summary of the request>

## The three positions

### Minimalist
<100-word distillation of the Minimalist proposal>

### Architect
<100-word distillation of the Architect proposal>

### Pragmatist
<100-word distillation of the Pragmatist proposal>

## Tradeoffs

| Option | Files touched | Effort (S/M/L) | Reversibility | Blast radius | User-visible value | Maintenance cost | Recommended by |
|---|---|---|---|---|---|---|---|
| Minimalist | <count / short list> | S/M/L | easy/medium/hard | internal / package / cross-package / API | none / modest / high | low / medium / high | self |
| Architect | ... | ... | ... | ... | ... | ... | self |
| Pragmatist | ... | ... | ... | ... | ... | ... | self |

## Where the agents disagree

- **<issue 1>**: Minimalist says X, Architect says Y, Pragmatist says Z. Resolved by: <your judgment>.
- **<issue 2>**: ...
- **<issue 3>**: ...

## Recommendation

**<Option name>** — <one sentence rationale grounded in which tradeoff dominates for this specific request.>

## Dissent (one line each)

- **Minimalist**: <what they'd object to in the recommendation, if anything>
- **Architect**: <same>
- **Pragmatist**: <same>

## Open questions for the developer

1. <a concrete question the agents couldn't resolve without more input>
2. <another concrete question>
3. <another concrete question>
```

## Rules for running /brainstorm

1. **Subagents run in parallel, not sequentially.** A single message with three `Agent` tool calls. Do not dispatch one, wait for the reply, then dispatch the next — that defeats the parallelism and wastes wall-clock time.

2. **Do not pre-bias the subagents.** Each gets the same request verbatim plus its forced position. Do not tell Subagent 1 what Subagent 2 will say.

3. **Cap each subagent at 400 words.** Long proposals dilute the signal. If a subagent returns 800 words, summarize to 100 in your synthesis anyway.

4. **The tradeoffs table is mandatory.** No free-form-only output. The table forces concrete answers for each column.

5. **The "Where the agents disagree" section is mandatory.** If you write "all three agents agreed", you failed — the forced positions should have produced at least 2-3 real disagreements. Go back and push harder on the positions that collapsed.

6. **The recommendation must be singular.** Pick one option. "It depends" is a non-answer. The open questions section is where you surface the things that could change the recommendation.

## When to use /brainstorm

- Feature with 2+ reasonable implementation paths
- Refactor where "how big" is the main question
- User asks "what do you think about X" and you genuinely have multiple defensible answers
- After `/interview` if the answers revealed ambiguity about approach

## When NOT to use it

- The answer is obvious (well-defined bug with one fix)
- The user has already picked an approach and wants implementation
- Trivial changes where the tradeoffs would be the same for all three positions
- When speed matters more than exploration — this command is deliberately expensive (3 parallel agents)
