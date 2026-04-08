import { create } from 'zustand';

/**
 * Single source of truth for the API bearer token.
 *
 * Storage policy:
 * - Default: sessionStorage (clears on tab close — safer on shared machines)
 * - Persistent opt-in: localStorage (survives reloads/reboots)
 * - Multi-tab sync: `storage` event listener propagates clears/sets across tabs
 *
 * The token is injected into every request by `authedFetch` and every SSE
 * connection by `authedEventStream`, as `Authorization: Bearer <token>`.
 */

const SESSION_KEY = 'itervox.apiToken';
const LOCAL_KEY = 'itervox.apiToken.persistent';

function readInitial(): string | null {
  try {
    // Prefer persistent — if the user ticked "Remember", that wins.
    const persistent = localStorage.getItem(LOCAL_KEY);
    if (persistent) return persistent;
    return sessionStorage.getItem(SESSION_KEY);
  } catch {
    // Private-mode browsers, sandboxed iframes, etc. — gracefully degrade.
    return null;
  }
}

interface TokenStoreState {
  token: string | null;
  /** Set the token, optionally persisting it across sessions. */
  setToken: (token: string, persist?: boolean) => void;
  /** Clear the token from both storages. */
  clearToken: () => void;
}

export const useTokenStore = create<TokenStoreState>((set) => ({
  token: readInitial(),
  setToken: (token, persist = false) => {
    try {
      if (persist) {
        localStorage.setItem(LOCAL_KEY, token);
        sessionStorage.removeItem(SESSION_KEY);
      } else {
        sessionStorage.setItem(SESSION_KEY, token);
        localStorage.removeItem(LOCAL_KEY);
      }
    } catch {
      // Storage unavailable — still set in memory so the current tab works.
    }
    set({ token });
  },
  clearToken: () => {
    try {
      sessionStorage.removeItem(SESSION_KEY);
      localStorage.removeItem(LOCAL_KEY);
    } catch {
      // ignore
    }
    set({ token: null });
  },
}));

/**
 * Synchronous read for non-React call sites (authedFetch, authedEventStream).
 * Avoids a Zustand subscription just to get the current value.
 */
export function getToken(): string | null {
  return useTokenStore.getState().token;
}

/**
 * Cross-tab sync: when another tab changes the token, mirror the change here.
 * Registered once at module load. SSR-safe (checks for window).
 */
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (e) => {
    if (e.key !== SESSION_KEY && e.key !== LOCAL_KEY) return;
    const next = readInitial();
    if (next !== useTokenStore.getState().token) {
      useTokenStore.setState({ token: next });
    }
  });
}
