import { useShallow } from 'zustand/react/shallow';
import { useSymphonyStore } from '../../../store/symphonyStore';

function StatTile({
  label,
  value,
  sub,
  valueColor,
}: {
  label: string;
  value: string;
  sub: string;
  valueColor?: string;
}) {
  return (
    <div
      className="flex flex-col items-center rounded-[var(--radius-md)] px-4 py-3 bg-white/[0.04]"
    >
      <span className="text-[10px] font-semibold uppercase tracking-[0.06em] text-theme-muted">
        {label}
      </span>
      <span
        className="mt-1 text-lg font-bold tabular-nums"
        style={{ color: valueColor ?? 'var(--text)' }}
      >
        {value}
      </span>
      <span className="text-[10px] text-theme-text-secondary">
        {sub}
      </span>
    </div>
  );
}

export function HeroStats() {
  const { running, paused, retrying, max } = useSymphonyStore(
    useShallow((s) => ({
      running: s.snapshot?.running.length ?? 0,
      paused: s.snapshot?.paused.length ?? 0,
      retrying: s.snapshot?.retrying.length ?? 0,
      max: s.snapshot?.maxConcurrentAgents ?? 0,
    })),
  );
  return (
    <div className="grid w-full flex-shrink-0 grid-cols-2 gap-2 sm:grid-cols-4 md:w-auto md:grid-cols-2">
      <StatTile label="Running" value={String(running)} sub="agents" valueColor={running > 0 ? 'var(--success)' : undefined} />
      <StatTile label="Paused" value={String(paused)} sub="issues" valueColor={paused > 0 ? 'var(--warning)' : undefined} />
      <StatTile label="Retrying" value={String(retrying)} sub="queued" valueColor={retrying > 0 ? 'var(--danger)' : undefined} />
      <StatTile
        label="Capacity"
        value={max > 0 ? `${String(running)}/${String(max)}` : '—'}
        sub={max > 0 ? `${String(Math.round((running / max) * 100))}% used` : 'No cap'}
        valueColor={max > 0 && running / max >= 0.9 ? 'var(--danger)' : max > 0 && running > 0 ? 'var(--success)' : undefined}
      />
    </div>
  );
}
