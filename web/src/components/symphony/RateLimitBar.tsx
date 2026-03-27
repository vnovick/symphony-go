import { useSymphonyStore } from '../../store/symphonyStore';

function fmtReset(resetStr: string | null): string | null {
  if (!resetStr) return null;
  const diff = Math.round((new Date(resetStr).getTime() - Date.now()) / 1000);
  if (diff <= 0) return null;
  if (diff < 60) return `resets in ${String(diff)}s`;
  return `resets in ${String(Math.ceil(diff / 60))}m`;
}

function barColor(pct: number): string {
  if (pct > 0.5) return 'var(--success)';
  if (pct > 0.2) return 'var(--warning)';
  return 'var(--danger)';
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
  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <span className="text-xs text-theme-text-secondary">
          {label}
        </span>
        <span className="text-xs text-theme-muted">
          {remaining.toLocaleString()}/{limit.toLocaleString()}
          {resetLabel && <span className="ml-1">· {resetLabel}</span>}
        </span>
      </div>
      <div className="h-1.5 w-full rounded-full bg-theme-bg-elevated">
        <div
          className="h-1.5 rounded-full transition-all duration-500"
          style={{ width: `${String(Math.max(2, pct * 100))}%`, background: barColor(pct) }}
        />
      </div>
    </div>
  );
}

export default function RateLimitBar({ compact = false }: { compact?: boolean }) {
  const rateLimits = useSymphonyStore((s) => s.snapshot?.rateLimits);
  const trackerKind = useSymphonyStore((s) => s.snapshot?.trackerKind);
  if (!rateLimits) return null;

  const hasRequests = rateLimits.requestsLimit > 0;
  const hasComplexity = (rateLimits.complexityLimit ?? 0) > 0;
  if (!hasRequests && !hasComplexity) return null;

  const resetLabel = fmtReset(rateLimits.requestsReset ?? null);
  const trackerLabel =
    trackerKind === 'github' ? 'GitHub' : trackerKind === 'linear' ? 'Linear' : 'API';

  const content = (
    <>
      <p
        className="mb-2.5 text-[10px] font-semibold uppercase tracking-widest"
      >
        {trackerLabel} API Headroom
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
    </>
  );

  if (compact) return content;

  return (
    <div
      className="rounded-[var(--radius-md)] p-4 border border-theme-line bg-theme-panel"
    >
      {content}
    </div>
  );
}
