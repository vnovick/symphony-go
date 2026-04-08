---
name: authed-transport
description: Use when editing any file in web/src/ that performs HTTP requests or opens SSE streams — any .ts/.tsx file outside web/src/auth/ that uses fetch, EventSource, or adds a new query/mutation under web/src/queries/. Enforces bearer-token transport rules so frontend network I/O stays compatible with ITERVOX_API_TOKEN auth and the AuthGate login flow.
---

# Authed transport rules (web/src/)

All frontend network I/O must flow through the auth module. The Go daemon gates its HTTP API on `ITERVOX_API_TOKEN`; raw `fetch`/`EventSource` calls bypass the bearer header and break login.

## Rule 1 — No raw `fetch()` outside `web/src/auth/`

Use `authedFetch` from `web/src/auth/authedFetch.ts`:

```ts
import { authedFetch } from '../auth/authedFetch';
import { UnauthorizedError } from '../auth/UnauthorizedError';

const res = await authedFetch('/api/v1/issues');
```

`authedFetch` injects `Authorization: Bearer <token>`, throws `UnauthorizedError` on 401, clears the stored token, and defers `markUnauthorized()` via `queueMicrotask` so TanStack Query rollbacks run BEFORE the AuthGate swaps the screen. Non-401 non-ok responses are returned unchanged — check `res.ok` as usual.

Enforce: `\bfetch\(` must not appear in `web/src/` outside `web/src/auth/`.

## Rule 2 — No `new EventSource(` anywhere in `web/src/`

Native `EventSource` cannot set headers, so it cannot carry the bearer token. Use `openAuthedEventStream` from `web/src/auth/authedEventStream.ts`:

```ts
import { openAuthedEventStream } from '../auth/authedEventStream';

const close = openAuthedEventStream('/api/v1/events', {
  onMessage: (msg) => { /* msg.data */ },
  onOpen: () => { /* connected */ },
  onDisconnect: () => { /* transient */ },
});
// later: close();
```

It wraps `@microsoft/fetch-event-source`, handles 401 the same way as `authedFetch`, and reconnects with exponential backoff (1s → 15s cap). The current consumers are `useItervoxSSE.ts`, `useLogStream.ts`, and `useIssueLogs` in `queries/logs.ts` — model new consumers on these.

## Rule 3 — Swallow `UnauthorizedError` in error handlers

The AuthGate (`web/src/auth/AuthGate.tsx`) handles the UI on 401. Never toast on top of the login screen:

```ts
} catch (err) {
  if (err instanceof UnauthorizedError) return; // AuthGate handles UI
  toastError('Action failed — please try again.');
}
```

`toastApiError` in `web/src/queries/issues.ts` already does this — call it (or copy the pattern) from every new mutation/error path.

## Rule 4 — New mutations in `queries/issues.ts` use the rollback handler

Add new mutations with the existing `makeRollbackHandler(queryClient)` pattern: optimistic update in `onMutate`, rollback in `onError`. Rollback on `UnauthorizedError` is fine — it mutates the React Query cache, which is independent of mount state, so the AuthGate screen swap won't lose the rollback.

## Rule 5 — Raw `fetch` exceptions (do not expand)

The ONLY files allowed to call raw `fetch` live in `web/src/auth/`:

- `AuthGate.tsx` — probes `/api/v1/health` (unauthenticated) and `/api/v1/state` for one-shot token validation.
- `TokenEntryScreen.tsx` — validates a user-pasted token before storing it.

These run BEFORE a token is in the store, so they cannot use `authedFetch`. Never add new entries to this exception list.

## Rule 6 — Trust the global TanStack Query retry guard

`web/src/main.tsx` sets `retry: (count, err) => !(err instanceof UnauthorizedError) && count < 2`, so 401s don't retry-spam during auth failure. Don't override `retry` on new queries unless you have a concrete reason.

## Verification

Before completing edits, run:

```bash
grep -rn "fetch(\|new EventSource(" web/src/ | grep -v "web/src/auth/"
```

Expected output: zero matches (other than `authedFetch.ts` itself, which defines the wrapper). Then `pnpm lint` for type safety.
