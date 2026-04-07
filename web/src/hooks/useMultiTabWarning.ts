import { useEffect } from 'react';
import { useToastStore } from '../store/toastStore';

const CHANNEL_NAME = 'itervox-tab-sync';
const PING = 'ping';
const PONG = 'pong';

/**
 * Detects when multiple browser tabs/windows have Itervox open and shows a
 * warning toast. Uses BroadcastChannel to coordinate — no server changes needed.
 *
 * Flow: on mount, sends a PING. Any existing tab replies with PONG.
 * If we receive a PONG, show a warning. If we receive a PING, reply with PONG
 * (the other tab will show the warning).
 */
export function useMultiTabWarning() {
  useEffect(() => {
    if (typeof BroadcastChannel === 'undefined') return; // SSR / unsupported browser

    const channel = new BroadcastChannel(CHANNEL_NAME);
    let warned = false;

    channel.onmessage = (event: MessageEvent) => {
      if (event.data === PING) {
        // Another tab just opened — reply so it knows we exist.
        channel.postMessage(PONG);
      } else if (event.data === PONG && !warned) {
        // Another tab replied to our ping — we're the new tab, show warning.
        warned = true;
        useToastStore
          .getState()
          .addToast(
            'Itervox is open in another tab. Using multiple tabs may cause SSE connection issues.',
            'error',
          );
      }
    };

    // Announce ourselves — any existing tab will reply with PONG.
    channel.postMessage(PING);

    return () => {
      channel.close();
    };
  }, []);
}
