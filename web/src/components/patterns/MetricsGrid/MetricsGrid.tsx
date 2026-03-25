import type { Status } from '../../ui/LiveIndicator/LiveIndicator';

export interface Metric {
  label: string;
  value: number | string;
  status: Status;
  subtitle?: string;
  badge?: string;
}

interface MetricsGridProps {
  metrics: Metric[];
  columns?: 2 | 3 | 4;
}

const GRID_CLASS: Record<2 | 3 | 4, string> = {
  2: 'grid-cols-2',
  3: 'grid-cols-3',
  4: 'grid-cols-4',
};

function badgeStyle(status: Status): React.CSSProperties {
  switch (status) {
    case 'live':
    case 'success':
      return { background: 'var(--success-soft)', color: 'var(--success)' };
    case 'warning':
      return { background: 'var(--warning-soft)', color: 'var(--warning)' };
    case 'error':
      return { background: 'var(--danger-soft)', color: 'var(--danger)' };
    default:
      return { background: 'rgba(255,255,255,0.06)', color: 'var(--text-secondary)' };
  }
}

export function MetricsGrid({ metrics, columns = 4 }: MetricsGridProps) {
  return (
    <div className={`grid gap-3 ${GRID_CLASS[columns]}`} data-columns={columns}>
      {metrics.map((m) => (
        <div
          key={m.label}
          data-status={m.status}
          className="rounded-[var(--radius-md)] transition-all hover:border-[var(--line-strong)] hover:shadow-md"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--line)',
            padding: '16px 18px',
          }}
        >
          {/* Header: label + badge */}
          <div className="mb-[10px] flex items-center justify-between gap-2">
            <span
              className="font-semibold uppercase"
              style={{ fontSize: 11, letterSpacing: '0.05em', color: 'var(--muted)' }}
            >
              {m.label}
            </span>
            {m.badge !== undefined && (
              <span
                className="rounded-[var(--radius-full)] font-semibold"
                style={{ ...badgeStyle(m.status), fontSize: 11, padding: '3px 8px' }}
              >
                {m.badge}
              </span>
            )}
          </div>
          {/* Value — gradient text matching .metric-value */}
          <div
            style={{
              fontSize: 32,
              fontWeight: 700,
              letterSpacing: '-0.02em',
              lineHeight: 1,
              background: 'var(--gradient-accent)',
              WebkitBackgroundClip: 'text',
              WebkitTextFillColor: 'transparent',
              backgroundClip: 'text',
            }}
          >
            {m.value}
          </div>
          {m.subtitle && (
            <p className="mt-2" style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.4 }}>
              {m.subtitle}
            </p>
          )}
        </div>
      ))}
    </div>
  );
}
