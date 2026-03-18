import { useSymphonyStore } from '../../store/symphonyStore';
import TokenSparkline from './TokenSparkline';

interface MetricCardProps {
  label: string;
  value: string | number;
  subtitle?: string;
  color?: string;
  children?: React.ReactNode;
}

function MetricCard({
  label,
  value,
  subtitle,
  color = 'text-gray-900 dark:text-white',
  children,
}: MetricCardProps) {
  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-5 dark:border-gray-800 dark:bg-white/[0.03]">
      <p className="mb-1 text-sm text-gray-500 dark:text-gray-400">{label}</p>
      <p className={`text-3xl font-bold ${color}`}>{value}</p>
      {subtitle && <p className="mt-1 text-xs text-gray-400 dark:text-gray-500">{subtitle}</p>}
      {children}
    </div>
  );
}

export default function MetricsCards() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const running = snapshot?.counts.running ?? 0;
  const retrying = snapshot?.counts.retrying ?? 0;
  const paused = snapshot?.counts.paused ?? 0;
  const maxAgents = snapshot?.maxConcurrentAgents ?? 0;
  const inputTokens = snapshot?.running.reduce((acc, r) => acc + r.inputTokens, 0) ?? 0;
  const outputTokens = snapshot?.running.reduce((acc, r) => acc + r.outputTokens, 0) ?? 0;
  const avgTurns =
    running > 0
      ? Math.round((snapshot?.running.reduce((acc, r) => acc + r.turnCount, 0) ?? 0) / running)
      : 0;

  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-6">
      <MetricCard
        label="Running Sessions"
        value={running}
        color={running > 0 ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}
      />
      <MetricCard
        label="Backoff Queue"
        value={retrying}
        color={
          retrying > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-900 dark:text-white'
        }
      />
      <MetricCard
        label="Paused"
        value={paused}
        color={paused > 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-900 dark:text-white'}
      />
      <MetricCard
        label="↑ Input Tokens"
        value={inputTokens > 0 ? inputTokens.toLocaleString() : '—'}
        subtitle={running > 0 ? `${String(avgTurns)} avg turns` : undefined}
      />
      <MetricCard
        label="↓ Output Tokens"
        value={outputTokens > 0 ? outputTokens.toLocaleString() : '—'}
      />
      <MetricCard
        label="Capacity"
        value={snapshot ? `${String(running)}/${String(maxAgents)}` : '—'}
        subtitle={maxAgents > 0 ? `${String(maxAgents - running)} slots free` : undefined}
        color={
          running >= maxAgents && maxAgents > 0
            ? 'text-red-600 dark:text-red-400'
            : 'text-gray-900 dark:text-white'
        }
      >
        <TokenSparkline height={48} />
      </MetricCard>
    </div>
  );
}
