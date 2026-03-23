import { useSymphonyStore } from '../../store/symphonyStore';

function fmtReset(resetStr: string | null): string | null {
  if (!resetStr) return null;
  const diff = Math.round((new Date(resetStr).getTime() - Date.now()) / 1000);
  if (diff <= 0) return null;
  if (diff < 60) return `resets in ${String(diff)}s`;
  return `resets in ${String(Math.ceil(diff / 60))}m`;
}

function LimitRow({
  label,
  remaining,
  limit,
  resetLabel,
}: {
  label: string;
  remaining: number;
  limit: number;
  resetLabel?: string | null;
}) {
  const pct = limit > 0 ? remaining / limit : 1;
  const barColor = pct > 0.5 ? 'bg-green-500' : pct > 0.2 ? 'bg-amber-500' : 'bg-red-500';
  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <span className="text-xs text-gray-600 dark:text-gray-400">{label}</span>
        <span className="text-xs text-gray-500 dark:text-gray-500">
          {remaining.toLocaleString()}/{limit.toLocaleString()}
          {resetLabel && <span className="ml-1">· {resetLabel}</span>}
        </span>
      </div>
      <div className="h-1.5 w-full rounded-full bg-gray-200 dark:bg-gray-700">
        <div
          className={`${barColor} h-1.5 rounded-full transition-all duration-500`}
          style={{ width: `${String(Math.max(2, pct * 100))}%` }}
        />
      </div>
    </div>
  );
}

export default function RateLimitBar() {
  const rateLimits = useSymphonyStore((s) => s.snapshot?.rateLimits);
  if (!rateLimits) return null;

  const hasRequests = rateLimits.requestsLimit > 0;
  const hasComplexity = (rateLimits.complexityLimit ?? 0) > 0;
  if (!hasRequests && !hasComplexity) return null;

  const resetLabel = fmtReset(rateLimits.requestsReset ?? null);

  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-white/[0.03]">
      <p className="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">
        Linear API Rate Limits
      </p>
      <div className="space-y-3">
        {hasRequests && (
          <LimitRow
            label="Requests / hr"
            remaining={rateLimits.requestsRemaining}
            limit={rateLimits.requestsLimit}
            resetLabel={resetLabel}
          />
        )}
        {hasComplexity && (
          <LimitRow
            label="Complexity / hr"
            remaining={rateLimits.complexityRemaining ?? 0}
            limit={rateLimits.complexityLimit ?? 0}
          />
        )}
      </div>
    </div>
  );
}
