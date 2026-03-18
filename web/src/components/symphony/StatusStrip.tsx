import { useState } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';

export default function StatusStrip() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const refreshSnapshot = useSymphonyStore((s) => s.refreshSnapshot);
  const [adjusting, setAdjusting] = useState(false);

  const running = snapshot?.running.length ?? 0;
  const paused = snapshot?.paused.length ?? 0;
  const retrying = snapshot?.retrying.length ?? 0;
  const maxAgents = snapshot?.maxConcurrentAgents ?? 0;

  const adjustWorkers = async (delta: number) => {
    if (adjusting) return;
    setAdjusting(true);
    try {
      const res = await fetch('/api/v1/settings/workers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ delta }),
      });
      if (res.ok) await refreshSnapshot();
    } catch {
      // endpoint may not be available yet
    } finally {
      setAdjusting(false);
    }
  };

  const pct = maxAgents > 0 ? Math.round((running / maxAgents) * 100) : 0;

  return (
    <div className="flex flex-wrap items-center gap-4 rounded-2xl border border-gray-200 bg-white px-4 py-3 text-sm shadow-sm dark:border-gray-800 dark:bg-white/[0.03]">
      {/* Live indicator */}
      <span className="flex items-center gap-2">
        <span className={`relative flex h-2.5 w-2.5`}>
          {running > 0 && (
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" />
          )}
          <span
            className={`relative inline-flex h-2.5 w-2.5 rounded-full ${running > 0 ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'}`}
          />
        </span>
        <span className="font-semibold text-gray-900 dark:text-white">{running}</span>
        <span className="text-gray-500">running</span>
      </span>

      {paused > 0 && (
        <span className="flex items-center gap-1.5 rounded-full bg-red-50 px-2 py-0.5 dark:bg-red-900/20">
          <span className="inline-block h-2 w-2 rounded-full bg-red-400" />
          <span className="font-medium text-red-700 dark:text-red-300">{paused} paused</span>
        </span>
      )}
      {retrying > 0 && (
        <span className="flex items-center gap-1.5 rounded-full bg-amber-50 px-2 py-0.5 dark:bg-amber-900/20">
          <span className="text-amber-500">↻</span>
          <span className="font-medium text-amber-700 dark:text-amber-400">
            {retrying} retrying
          </span>
        </span>
      )}

      {/* Capacity bar */}
      {maxAgents > 0 && (
        <span className="ml-2 flex items-center gap-2">
          <span className="text-xs text-gray-400">capacity</span>
          <span className="h-1.5 w-20 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
            <span
              className={`h-full rounded-full transition-all ${pct >= 90 ? 'bg-red-400' : pct >= 60 ? 'bg-amber-400' : 'bg-green-400'}`}
              style={{ width: `${String(pct)}%` }}
            />
          </span>
          <span className="font-mono text-xs font-medium text-gray-700 dark:text-gray-300">
            {running}/{maxAgents}
          </span>
        </span>
      )}

      {/* Agent mode badge */}
      {snapshot?.agentMode === 'subagents' && (
        <span className="ml-1 flex items-center gap-1.5 rounded-full bg-violet-50 px-2 py-0.5 dark:bg-violet-900/20">
          <span className="text-xs text-violet-500">⇄</span>
          <span className="text-xs font-medium text-violet-700 dark:text-violet-300">
            sub-agents
          </span>
        </span>
      )}
      {snapshot?.agentMode === 'teams' && (
        <span className="ml-1 flex items-center gap-1.5 rounded-full bg-blue-50 px-2 py-0.5 dark:bg-blue-900/20">
          <span className="text-xs text-blue-500">⊞</span>
          <span className="text-xs font-medium text-blue-700 dark:text-blue-300">agent teams</span>
        </span>
      )}

      <span className="ml-auto flex items-center gap-1">
        <button
          onClick={() => {
            void adjustWorkers(-1);
          }}
          disabled={adjusting || maxAgents <= 1}
          title="Decrease max workers"
          className="flex h-7 w-7 items-center justify-center rounded-lg border border-gray-200 text-base font-medium text-gray-500 transition-colors hover:border-gray-300 hover:bg-gray-50 disabled:opacity-40 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800"
        >
          −
        </button>
        <button
          onClick={() => {
            void adjustWorkers(1);
          }}
          disabled={adjusting}
          title="Increase max workers"
          className="flex h-7 w-7 items-center justify-center rounded-lg border border-gray-200 text-base font-medium text-gray-500 transition-colors hover:border-gray-300 hover:bg-gray-50 disabled:opacity-40 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800"
        >
          +
        </button>
      </span>
    </div>
  );
}
