import { useSymphonyStore } from '../../store/symphonyStore';
import RateLimitBar from './RateLimitBar';

export function HealthRail() {
  const snapshot = useSymphonyStore((s) => s.snapshot);

  if (!snapshot) return null;

  const running = snapshot.counts.running;
  const retrying = snapshot.counts.retrying;
  const paused = snapshot.counts.paused;
  const maxAgents = snapshot.maxConcurrentAgents ?? 0;
  const saturationPct = maxAgents > 0 ? Math.min(100, (running / maxAgents) * 100) : 0;

  return (
    <div
      data-testid="health-rail"
      className="hidden xl:flex flex-col gap-4 w-52 flex-shrink-0"
    >
      {/* API headroom */}
      <div
        className="rounded-[var(--radius-md)] p-3"
        style={{ border: '1px solid var(--line)', background: 'var(--panel)' }}
      >
        <RateLimitBar compact />
      </div>

      {/* Worker saturation */}
      <div
        className="rounded-[var(--radius-md)] p-3 space-y-2"
        style={{ border: '1px solid var(--line)', background: 'var(--panel)' }}
      >
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wide" style={{ color: 'var(--muted)' }}>
            Capacity
          </span>
          <span className="font-mono text-xs font-semibold" style={{ color: 'var(--text)' }}>
            {running} / {maxAgents > 0 ? maxAgents : '∞'}
          </span>
        </div>
        <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--bg-soft)' }}>
          <div
            className="h-full rounded-full transition-all duration-500"
            style={{
              width: `${saturationPct}%`,
              background: saturationPct >= 90 ? 'var(--danger)' : saturationPct >= 70 ? 'var(--warning)' : 'var(--success)',
            }}
          />
        </div>
      </div>

      {/* Retry pressure */}
      {retrying > 0 && (
        <div
          className="rounded-[var(--radius-md)] p-3 flex items-center justify-between"
          style={{ border: '1px solid var(--warning-soft)', background: 'var(--warning-soft)' }}
        >
          <span className="text-xs font-medium" style={{ color: 'var(--warning)' }}>
            Retry pressure
          </span>
          <span className="font-mono text-sm font-bold" style={{ color: 'var(--warning)' }}>
            {retrying}
          </span>
        </div>
      )}

      {/* Blocked badge */}
      {paused > 0 && (
        <div
          className="rounded-[var(--radius-md)] p-3 flex items-center justify-between"
          style={{ border: '1px solid var(--danger-soft)', background: 'var(--danger-soft)' }}
        >
          <span className="text-xs font-medium" style={{ color: 'var(--danger)' }}>
            {paused} blocked
          </span>
          <span className="text-xs" style={{ color: 'var(--danger)' }}>⏸</span>
        </div>
      )}
    </div>
  );
}
