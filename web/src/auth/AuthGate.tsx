import { useCallback, useEffect, type ReactNode } from 'react';
import { useAuthStore } from './authStore';
import { getToken, useTokenStore } from './tokenStore';
import { TokenEntryScreen } from './TokenEntryScreen';
import { ServerDownScreen } from './ServerDownScreen';

/**
 * Top-level wrapper that gates the app on successful authentication.
 *
 * Mount flow:
 *   1. Capture `?token=` from the URL (if present) → tokenStore → strip from URL.
 *   2. Probe unauthenticated `/api/v1/health` to distinguish "server down"
 *      from "auth misconfigured".
 *   3. If health ok, probe `/api/v1/state` with any stored token:
 *      - 200 → status='authorized', render children.
 *      - 401 → status='needsToken', render TokenEntryScreen.
 *      - 200 but no token stored and server didn't require one → authorized.
 *   4. If health failed → status='serverDown', render ServerDownScreen with retry.
 *
 * Re-probes when authStore.status flips back to 'unknown' (e.g. after retry).
 */

function captureTokenFromUrl(): void {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  const token = url.searchParams.get('token');
  if (!token) return;
  useTokenStore.getState().setToken(token, false);
  url.searchParams.delete('token');
  const cleaned = url.pathname + (url.search ? url.search : '') + url.hash;
  window.history.replaceState(null, '', cleaned);
}

export function AuthGate({ children }: { children: ReactNode }) {
  const status = useAuthStore((s) => s.status);

  const probe = useCallback(async () => {
    // Health probe — unauthenticated.
    try {
      const health = await fetch('/api/v1/health');
      if (!health.ok) {
        useAuthStore.getState().setStatus('serverDown');
        return;
      }
    } catch {
      useAuthStore.getState().setStatus('serverDown');
      return;
    }

    // Authenticated state probe.
    const token = getToken();
    const headers: Record<string, string> = {};
    if (token) headers.Authorization = `Bearer ${token}`;
    try {
      const res = await fetch('/api/v1/state', { headers });
      if (res.status === 401) {
        useTokenStore.getState().clearToken();
        useAuthStore
          .getState()
          .markUnauthorized(
            token
              ? 'Stored token was rejected. Paste a fresh one.'
              : 'Server requires an API token.',
          );
        return;
      }
      if (!res.ok) {
        useAuthStore.getState().setStatus('serverDown');
        return;
      }
      useAuthStore.getState().setStatus('authorized');
    } catch {
      useAuthStore.getState().setStatus('serverDown');
    }
  }, []);

  useEffect(() => {
    captureTokenFromUrl();
  }, []);

  useEffect(() => {
    if (status === 'unknown') {
      void probe();
    }
  }, [status, probe]);

  if (status === 'authorized') return <>{children}</>;
  if (status === 'needsToken') return <TokenEntryScreen />;
  if (status === 'serverDown') {
    return (
      <ServerDownScreen
        onRetry={() => {
          void probe();
        }}
      />
    );
  }
  // 'unknown' — show nothing (or a tiny spinner) while probing.
  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-current border-t-transparent" />
    </div>
  );
}
