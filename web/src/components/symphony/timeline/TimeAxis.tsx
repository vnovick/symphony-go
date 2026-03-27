import { useEffect, useState } from 'react';

function NowMarker({ viewStart, viewEnd }: { viewStart: number; viewEnd: number }) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const id = setInterval(() => { setNow(Date.now()); }, 1000);
    return () => { clearInterval(id); };
  }, []);
  const pct = ((now - viewStart) / (viewEnd - viewStart)) * 100;
  if (pct < 0 || pct > 100) return null;
  return (
    <div
      className="pointer-events-none absolute top-0 bottom-0 w-px"
      style={{ left: `${String(pct)}%`, background: 'var(--danger)' }}
    />
  );
}

export function TimeAxis({ viewStart, viewEnd }: { viewStart: number; viewEnd: number }) {
  const span = viewEnd - viewStart;
  const rawStep = span / 6;
  const steps = [30_000, 60_000, 5 * 60_000, 10 * 60_000, 30 * 60_000, 60 * 60_000];
  const step = steps.find((s) => s >= rawStep) ?? steps[steps.length - 1];

  const ticks: number[] = [];
  const first = Math.ceil(viewStart / step) * step;
  for (let t = first; t <= viewEnd; t += step) ticks.push(t);

  return (
    <div
      className="relative h-6 border-b border-theme-line"
      style={{ marginLeft: 140, marginRight: 80 }}
    >
      {ticks.map((t) => {
        const pct = ((t - viewStart) / span) * 100;
        if (pct < 0 || pct > 100) return null;
        const label = new Date(t).toLocaleTimeString([], {
          hour: '2-digit',
          minute: '2-digit',
          hour12: false,
        });
        return (
          <span
            key={t}
            className="absolute font-mono text-[10px]"
            style={{ left: `${String(pct)}%`, transform: 'translateX(-50%)', color: 'var(--muted)' }}
          >
            {label}
          </span>
        );
      })}
      <NowMarker viewStart={viewStart} viewEnd={viewEnd} />
    </div>
  );
}
