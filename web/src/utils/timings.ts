// Named timing constants used across the app.
// Centralised here so tuning one value updates all affected components.

/** Base delay (ms) before the first SSE reconnect attempt. Doubles on each retry up to SSE_RECONNECT_MAX_MS. */
export const SSE_RECONNECT_BASE_MS = 5_000;

/** Maximum SSE reconnect backoff delay (ms). */
export const SSE_RECONNECT_MAX_MS = 30_000;

/** Delay (ms) before the running-sessions table clears stale rows after agents go idle. */
export const LOG_STABLE_DELAY_MS = 5_000;

/** Duration (ms) the "Saved successfully" banner stays visible after a settings save. */
export const SAVE_OK_BANNER_MS = 3_000;

/** Duration (ms) before a toast auto-dismisses. */
export const TOAST_DISMISS_MS = 4_000;
