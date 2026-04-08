import { fetchEventSource, type EventSourceMessage } from '@microsoft/fetch-event-source';
import { getToken, useTokenStore } from './tokenStore';
import { useAuthStore } from './authStore';

/**
 * Replaces native EventSource with a fetch-based SSE client that CAN send
 * `Authorization: Bearer <token>` headers (native EventSource cannot).
 *
 * Uses `@microsoft/fetch-event-source` under the hood. The library handles
 * reconnection when `onerror` returns a number (delay in ms); we throw from
 * `onerror` to signal a fatal error (auth failure) that should stop retries.
 */

// Sentinel: thrown from onopen/onerror to abort without triggering reconnect.
class FatalSSEError extends Error {
  constructor(msg: string) {
    super(msg);
    this.name = 'FatalSSEError';
  }
}

export interface AuthedEventStreamOptions {
  /** Called for each SSE message (default channel — no `event:` field). */
  onMessage: (msg: EventSourceMessage) => void;
  /** Called once the stream connects (server responded 200). */
  onOpen?: () => void;
  /** Called when the stream disconnects (transient — a reconnect will follow). */
  onDisconnect?: () => void;
}

const SSE_RECONNECT_BASE_MS = 1000;
const SSE_RECONNECT_MAX_MS = 15_000;

/**
 * Opens an authenticated SSE stream. Returns a closer function.
 *
 * On 401 at open time: clears the token, flips auth status, stops retrying.
 * On other failures: reconnects with exponential backoff (1s → 15s cap).
 */
export function openAuthedEventStream(url: string, opts: AuthedEventStreamOptions): () => void {
  const ctrl = new AbortController();
  let attempt = 0;

  void fetchEventSource(url, {
    signal: ctrl.signal,
    // Keep streaming when the tab is hidden (matches native EventSource).
    openWhenHidden: true,
    headers: ((): Record<string, string> => {
      const token = getToken();
      const h: Record<string, string> = {};
      if (token) h.Authorization = `Bearer ${token}`;
      return h;
    })(),
    onopen(res) {
      // fetchEventSource accepts either a sync or Promise-returning onopen.
      // We don't need to await anything here, so a sync function avoids the
      // @typescript-eslint/require-await lint warning.
      if (res.status === 401) {
        useTokenStore.getState().clearToken();
        queueMicrotask(() => {
          useAuthStore.getState().markUnauthorized();
        });
        throw new FatalSSEError('unauthorized');
      }
      if (!res.ok) {
        throw new Error(`sse open failed: ${String(res.status)}`);
      }
      attempt = 0;
      opts.onOpen?.();
      return Promise.resolve();
    },
    onmessage(msg) {
      opts.onMessage(msg);
    },
    onclose() {
      opts.onDisconnect?.();
      // Returning normally from onclose triggers a reconnect by the library.
    },
    onerror(err) {
      if (err instanceof FatalSSEError) {
        // Rethrow → library stops retrying.
        throw err;
      }
      opts.onDisconnect?.();
      // Exponential backoff with a cap. Returning a number tells the library
      // how long to wait before the next attempt.
      const delay = Math.min(SSE_RECONNECT_BASE_MS * 2 ** attempt, SSE_RECONNECT_MAX_MS);
      attempt += 1;
      return delay;
    },
  }).catch(() => {
    // Swallow: onerror/onopen already handled user-visible state.
  });

  return () => {
    ctrl.abort();
  };
}
