import { useEffect } from 'react';
import { useSymphonyStore } from '../store/symphonyStore';

/**
 * Streams log lines from /api/v1/logs into the Zustand store.
 * Accepts an optional identifier to filter logs server-side.
 */
export function useLogStream(identifier?: string) {
  useEffect(() => {
    // Read appendLog via getState() so this effect never re-runs due to store
    // action reference changes (same pattern as useSymphonySSE).
    const { appendLog } = useSymphonyStore.getState();

    let es: EventSource | undefined;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;

    function connect() {
      if (cancelled) return;
      const url = identifier
        ? `/api/v1/logs?identifier=${encodeURIComponent(identifier)}`
        : '/api/v1/logs';
      es = new EventSource(url);

      es.addEventListener('log', (e: MessageEvent<string>) => {
        try {
          appendLog(e.data);
        } catch (err) {
          if (import.meta.env.DEV) {
            console.warn('[symphony] useLogStream: appendLog threw', err);
          }
        }
      });

      es.onerror = () => {
        es?.close();
        if (!cancelled) {
          reconnectTimer = setTimeout(connect, 3000);
        }
      };
    }

    connect();

    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (es) es.close();
    };
  }, [identifier]); // appendLog omitted — stable via getState(), no reconnect needed on action change
}
