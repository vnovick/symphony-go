import { useEffect } from 'react';
import { useItervoxStore } from '../store/itervoxStore';
import { openAuthedEventStream } from '../auth/authedEventStream';

/**
 * Streams log lines from /api/v1/logs into the Zustand store.
 * Accepts an optional identifier to filter logs server-side.
 * Uses the authed SSE helper so it sends the bearer token.
 */
export function useLogStream(identifier?: string) {
  useEffect(() => {
    const { appendLog } = useItervoxStore.getState();

    const url = identifier
      ? `/api/v1/logs?identifier=${encodeURIComponent(identifier)}`
      : '/api/v1/logs';

    const close = openAuthedEventStream(url, {
      onMessage: (msg) => {
        if (msg.event !== 'log') return;
        try {
          appendLog(msg.data);
        } catch (err) {
          if (import.meta.env.DEV) {
            console.warn('[itervox] useLogStream: appendLog threw', err);
          }
        }
      },
    });

    return () => {
      close();
    };
  }, [identifier]);
}
