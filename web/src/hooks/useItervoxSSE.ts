import { useEffect } from 'react';
import { useItervoxStore } from '../store/itervoxStore';
import { StateSnapshotSchema } from '../types/schemas';
import type { StateSnapshot } from '../types/schemas';
import { authedFetch } from '../auth/authedFetch';
import { openAuthedEventStream } from '../auth/authedEventStream';
import { UnauthorizedError } from '../auth/UnauthorizedError';

/**
 * Connects to /api/v1/events (SSE) and keeps the Zustand snapshot up to date.
 * Uses @microsoft/fetch-event-source under the hood so the connection can
 * carry `Authorization: Bearer <token>` headers.
 *
 * Falls back to polling /api/v1/state every 3s while SSE is down. Mounts once.
 */
export function useItervoxSSE() {
  useEffect(() => {
    const { setSnapshot, setSseConnected } = useItervoxStore.getState();

    let pollTimer: ReturnType<typeof setInterval> | null = null;
    let sseWorking = false;
    let cancelled = false;

    async function poll() {
      if (sseWorking || cancelled) return;
      try {
        const res = await authedFetch('/api/v1/state');
        if (res.ok) {
          const snap = StateSnapshotSchema.parse(await res.json());
          setSnapshot(snap);
        }
      } catch (err) {
        if (err instanceof UnauthorizedError) return; // AuthGate will handle.
      }
    }

    function startPoll() {
      if (pollTimer) return;
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

    const close = openAuthedEventStream('/api/v1/events', {
      onOpen: () => {
        sseWorking = true;
        setSseConnected(true);
        stopPoll();
      },
      onMessage: (msg) => {
        if (!msg.data) return;
        try {
          const snap: StateSnapshot = StateSnapshotSchema.parse(JSON.parse(msg.data));
          setSnapshot(snap);
          if (!sseWorking) {
            sseWorking = true;
            setSseConnected(true);
            stopPoll();
          }
        } catch (err) {
          if (import.meta.env.DEV) {
            console.warn('[itervox] SSE message parse/validation failed', err);
          }
        }
      },
      onDisconnect: () => {
        sseWorking = false;
        setSseConnected(false);
        startPoll();
      },
    });

    // Start polling immediately so the dashboard shows data during SSE handshake.
    startPoll();

    return () => {
      cancelled = true;
      sseWorking = false;
      stopPoll();
      close();
      setSseConnected(false);
    };
  }, []);
}
