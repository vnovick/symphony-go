import { useAuthStore } from './authStore';

/**
 * Shown when /api/v1/health fails with a network error — distinguishes
 * "daemon not running" from "auth misconfigured".
 */
export function ServerDownScreen({ onRetry }: { onRetry: () => void }) {
  const setStatus = useAuthStore((s) => s.setStatus);
  return (
    <div className="flex min-h-screen items-center justify-center p-6">
      <div className="bg-theme-bg-soft border-theme-line max-w-md rounded-[var(--radius-lg)] border p-6 text-center shadow-xl">
        <h1 className="mb-2 text-xl font-semibold">Can't reach the daemon</h1>
        <p className="text-theme-muted mb-4 text-sm">
          <code className="font-mono text-xs">/api/v1/health</code> is unreachable. Check that{' '}
          <code className="font-mono text-xs">itervox</code> is running and the host/port match your
          dashboard URL.
        </p>
        <button
          type="button"
          onClick={() => {
            setStatus('unknown');
            onRetry();
          }}
          className="rounded-[var(--radius-md)] bg-[color:var(--color-accent)] px-4 py-2 text-sm font-medium text-white"
        >
          Retry
        </button>
      </div>
    </div>
  );
}
