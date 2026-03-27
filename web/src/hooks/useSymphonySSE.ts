import { useEffect } from 'react';
import { useSymphonyStore } from '../store/symphonyStore';
import { StateSnapshotSchema } from '../types/schemas';
import type { StateSnapshot } from '../types/schemas';
import { SSE_RECONNECT_BASE_MS, SSE_RECONNECT_MAX_MS } from '../utils/timings';

/**
 * Connects to /api/v1/events (SSE) and keeps the Zustand snapshot up to date.
 * Falls back to polling /api/v1/state every 3s when SSE fails, so the
 * dashboard always shows data even if EventSource is unavailable.
 * Reconnects with exponential backoff (5s → 10s → 20s → 30s cap).
 * Mounts once at app level.
 *
 * Store actions are read via getState() inside the effect rather than via
 * selector subscriptions so that this effect never re-runs due to store
 * action reference changes (e.g. after middleware additions).
 */
export function useSymphonySSE() {
  useEffect(() => {
    // Read actions once at mount. Zustand actions are stable references, but
    // using getState() here decouples the effect from any future store refactors.
    const { setSnapshot, setSseConnected } = useSymphonyStore.getState();

    let es: EventSource | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let pollTimer: ReturnType<typeof setInterval> | null = null;
    let cancelled = false;
    let sseWorking = false;
    let reconnectAttempt = 0;

    // Fallback: poll /api/v1/state while SSE is not connected.
    async function poll() {
      if (sseWorking || cancelled) return;
      try {
        const res = await fetch('/api/v1/state');
        if (res.ok) {
          const snap = StateSnapshotSchema.parse(await res.json());
          setSnapshot(snap);
        }
      } catch {
        // network error — ignore
      }
    }

    function startPoll() {
      if (pollTimer) return;
      // Poll immediately and then every 3s while SSE is down.
      void poll();
      pollTimer = setInterval(() => {
        void poll();
      }, 3000);
    }

    function stopPoll() {
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    }

    function connect() {
      if (cancelled) return;
      es = new EventSource('/api/v1/events');

      es.onopen = () => {
        sseWorking = true;
        reconnectAttempt = 0; // reset backoff on successful connection
        setSseConnected(true);
        stopPoll();
      };

      es.onmessage = (e: MessageEvent<string>) => {
        try {
          const snap: StateSnapshot = StateSnapshotSchema.parse(JSON.parse(e.data));
          setSnapshot(snap);
          if (!sseWorking) {
            sseWorking = true;
            setSseConnected(true);
            stopPoll();
          }
        } catch (err) {
          // malformed JSON or schema validation failure — skip
          if (import.meta.env.DEV) {
            console.warn('[symphony] SSE message parse/validation failed', err);
          }
        }
      };

      es.onerror = () => {
        sseWorking = false;
        setSseConnected(false);
        es?.close();
        es = null;
        startPoll(); // fall back to polling while SSE is down
        // Guard: if a reconnect is already scheduled (e.g. a second error fires
        // before the timer fires), don't stack another timer on top.
        if (!cancelled && reconnectTimer === null) {
          const delay = Math.min(
            SSE_RECONNECT_BASE_MS * 2 ** reconnectAttempt,
            SSE_RECONNECT_MAX_MS,
          );
          reconnectAttempt++;
          reconnectTimer = setTimeout(connect, delay);
        }
      };
    }

    connect();
    // Also start polling immediately so the dashboard shows data
    // even during the SSE handshake window.
    startPoll();

    return () => {
      cancelled = true;
      sseWorking = false;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      stopPoll();
      es?.close();
      setSseConnected(false);
    };
  }, []); // empty deps: store actions are stable; getState() is always current
}
