import { useState } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';

export function CapacityCard() {
  const maxConcurrentAgents = useSymphonyStore((s) => s.snapshot?.maxConcurrentAgents ?? 0);
  const { bumpWorkers } = useSettingsActions();
  const [adjusting, setAdjusting] = useState(false);

  const handleBump = async (delta: number) => {
    if (adjusting) return;
    setAdjusting(true);
    await bumpWorkers(delta);
    setAdjusting(false);
  };

  return (
    <div
      className="rounded-lg border p-4 border-theme-line bg-theme-panel"
    >
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-theme-text">
            Max concurrent agents
          </p>
          <p className="mt-0.5 text-xs text-theme-muted">
            Maximum number of agents that can run at the same time across all hosts.
          </p>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0 ml-4">
          <button
            onClick={() => { void handleBump(-1); }}
            disabled={adjusting || maxConcurrentAgents <= 1}
            aria-label="Decrease max concurrent agents"
            style={{
              width: 28,
              height: 28,
              borderRadius: 6,
              fontSize: 16,
              lineHeight: 1,
              cursor: adjusting || maxConcurrentAgents <= 1 ? 'not-allowed' : 'pointer',
              background: 'var(--bg-soft)',
              color: 'var(--text)',
              border: '1px solid var(--line)',
              opacity: adjusting || maxConcurrentAgents <= 1 ? 0.4 : 1,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            −
          </button>
          <span
            className="font-mono text-base font-semibold tabular-nums text-theme-text"
            style={{ minWidth: 24, textAlign: 'center' }}
          >
            {maxConcurrentAgents}
          </span>
          <button
            onClick={() => { void handleBump(1); }}
            disabled={adjusting}
            aria-label="Increase max concurrent agents"
            style={{
              width: 28,
              height: 28,
              borderRadius: 6,
              fontSize: 16,
              lineHeight: 1,
              cursor: adjusting ? 'not-allowed' : 'pointer',
              background: 'var(--bg-soft)',
              color: 'var(--text)',
              border: '1px solid var(--line)',
              opacity: adjusting ? 0.4 : 1,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            +
          </button>
        </div>
      </div>
    </div>
  );
}
