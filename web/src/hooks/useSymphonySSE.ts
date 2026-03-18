import { useEffect } from 'react';
import { useSymphonyStore } from '../store/symphonyStore';
import type { StateSnapshot } from '../types/symphony';

/**
 * Connects to /api/v1/events (SSE) and keeps the Zustand snapshot up to date.
 * Falls back to polling /api/v1/state every 3s when SSE fails, so the
 * dashboard always shows data even if EventSource is unavailable.
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
        setSseConnected(true);
        stopPoll();
      };

      es.onmessage = (e: MessageEvent<string>) => {
        try {
          const snap: StateSnapshot = JSON.parse(e.data) as StateSnapshot;
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
          reconnectTimer = setTimeout(connect, 5000);
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
