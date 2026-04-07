---

## Asking for human input — CRITICAL

If you genuinely cannot proceed without a human decision (ambiguous spec,
destructive action, missing credentials, conflicting requirements), you **must**
emit the literal token `<!-- itervox:needs-input -->` on its own line at the
end of your final message, followed by exactly one concise question. Itervox
detects this token, pauses the issue, and surfaces the question in the
dashboard and tracker so a human can reply. The token is an HTML comment so
it stays invisible in rendered Linear/GitHub markdown.

**This is a hard contract.** Without the sentinel token, Itervox cannot detect
that you are blocked — the issue will be marked as completed and moved to the
next state, which is incorrect. Asking a question in plain English without
the token is not enough.

### Example — correct usage

```
I've reviewed the auth module and there are two viable approaches:
1. Refresh tokens with rotation (more secure, breaking change)
2. Extend the JWT TTL (backwards compatible, lower security)

I need your decision before proceeding because this affects all existing
API clients.

<!-- itervox:needs-input -->
Which approach should I take: rotation (1) or extended TTL (2)?
```

### When to use
- Ambiguous requirements that have multiple valid interpretations
- Destructive actions that cannot be automatically reverted
- Missing credentials, API keys, or access tokens
- Conflicting instructions in the issue description
- Decisions that affect backwards compatibility

### When NOT to use
- Status updates or progress reports
- Rhetorical questions in your reasoning
- Questions that you can answer yourself by exploring the code
- Minor style or naming decisions (make a reasonable choice and move on)
