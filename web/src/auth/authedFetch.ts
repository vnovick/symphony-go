import { getToken, useTokenStore } from './tokenStore';
import { useAuthStore } from './authStore';
import { UnauthorizedError } from './UnauthorizedError';

/**
 * Drop-in replacement for `fetch()` that injects `Authorization: Bearer <token>`
 * when a token is present in the token store.
 *
 * On 401:
 *   - Clears the stored token (it's invalid).
 *   - Defers `markUnauthorized()` to a microtask so TanStack Query's
 *     `onError` rollback runs BEFORE the AuthGate swaps the screen.
 *     Otherwise optimistic updates would be left dirty on the next login.
 *   - Throws `UnauthorizedError` so callers can distinguish auth failures
 *     from other non-ok responses.
 *
 * Non-401 non-ok responses are returned unchanged — callers already check
 * `res.ok` and handle them.
 */
export async function authedFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
): Promise<Response> {
  const token = getToken();
  const headers = new Headers(init.headers);
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const res = await fetch(input, { ...init, headers });

  if (res.status === 401) {
    // Clear synchronously so subsequent requests in the same tick don't
    // retry with a known-bad token.
    useTokenStore.getState().clearToken();
    // Defer the screen swap so mutation rollbacks get a chance to run.
    queueMicrotask(() => {
      useAuthStore.getState().markUnauthorized();
    });
    throw new UnauthorizedError();
  }

  return res;
}
