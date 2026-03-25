import { MetricsGrid } from '../patterns/MetricsGrid/MetricsGrid';
import type { Metric } from '../patterns/MetricsGrid/MetricsGrid';
import { useSymphonyStore } from '../../store/symphonyStore';
import type { Status } from '../ui/LiveIndicator/LiveIndicator';

export function MetricsStrip() {
  const snapshot = useSymphonyStore((s) => s.snapshot);

  const running = snapshot?.counts.running ?? 0;
  const retrying = snapshot?.counts.retrying ?? 0;
  const paused = snapshot?.counts.paused ?? 0;
  const maxAgents = snapshot?.maxConcurrentAgents ?? 0;
  const capacityPct = maxAgents > 0 ? Math.round((running / maxAgents) * 100) : 0;

  const metrics: Metric[] = [
    {
      label: 'Running',
      value: running,
      status: (running > 0 ? 'live' : 'idle') satisfies Status,
      badge: running > 0 ? 'live' : String(running),
      subtitle: 'Active sessions',
    },
    {
      label: 'Backoff',
      value: retrying,
      status: (retrying > 0 ? 'warning' : 'idle') satisfies Status,
      badge: String(retrying),
      subtitle: 'In retry queue',
    },
    {
      label: 'Paused',
      value: paused,
      status: (paused > 0 ? 'error' : 'idle') satisfies Status,
      badge: String(paused),
      subtitle: 'Manually paused',
    },
    {
      label: 'Capacity',
      value: maxAgents > 0 ? `${String(running)}/${String(maxAgents)}` : '—',
      status: (
        maxAgents > 0 && running / maxAgents >= 0.9
          ? 'error'
          : maxAgents > 0 && running / maxAgents >= 0.6
            ? 'warning'
            : running > 0
              ? 'success'
              : 'idle'
      ) satisfies Status,
      badge: maxAgents > 0 ? `${String(capacityPct)}%` : '—',
      subtitle: 'Agents / max',
    },
  ];

  return <MetricsGrid metrics={metrics} columns={4} />;
}
