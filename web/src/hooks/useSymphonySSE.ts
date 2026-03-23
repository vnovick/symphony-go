import { useEffect } from 'react';
import { useSymphonyStore } from '../store/symphonyStore';
import { StateSnapshotSchema } from '../types/schemas';
import type { StateSnapshot } from '../types/symphony';

const SSE_RECONNECT_BASE_MS = 5_000;
const SSE_RECONNECT_MAX_MS = 30_000;

/**
 * Connects to /api/v1/events (SSE) and keeps the Zustand snapshot up to date.
 * Falls back to polling /api/v1/state every 3s when SSE fails, so the
 * dashboard always shows data even if EventSource is unavailable.
 * Reconnects with exponential backoff (5s → 10s → 20s → 30s cap).
 * Mounts once at app level.
 */
export function useSymphonySSE() {
  const setSnapshot = useSymphonyStore((s) => s.setSnapshot);
  const setSseConnected = useSymphonyStore((s) => s.setSseConnected);

  useEffect(() => {
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
          const snap = (await res.json()) as StateSnapshot;
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
        } catch {
          // malformed JSON — skip
        }
      };

      es.onerror = () => {
        sseWorking = false;
        setSseConnected(false);
        es?.close();
        es = null;
        startPoll(); // fall back to polling while SSE is down
        if (!cancelled) {
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
  }, [setSnapshot, setSseConnected]);
}
