import { create } from 'zustand';

/**
 * Tracks the current authentication status for the AuthGate.
 *
 * State machine:
 *   unknown     — initial; AuthGate is probing /health
 *   serverDown  — /health failed (network error); retry UI
 *   needsToken  — /health ok but no token in store, OR token was rejected
 *   authorized  — token validated against /state; app can render
 */
export type AuthStatus = 'unknown' | 'serverDown' | 'needsToken' | 'authorized';

interface AuthStoreState {
  status: AuthStatus;
  /** Non-null when status is 'needsToken' after a 401 — tells the user why. */
  rejectedReason: string | null;
  setStatus: (status: AuthStatus) => void;
  markUnauthorized: (reason?: string) => void;
}

export const useAuthStore = create<AuthStoreState>((set) => ({
  status: 'unknown',
  rejectedReason: null,
  setStatus: (status) => {
    set({ status, rejectedReason: status === 'needsToken' ? null : null });
  },
  markUnauthorized: (reason = 'Token rejected by server (401).') => {
    set({ status: 'needsToken', rejectedReason: reason });
  },
}));
