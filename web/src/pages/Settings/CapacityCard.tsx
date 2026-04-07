import { useState } from 'react';
import { useItervoxStore } from '../../store/itervoxStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';

export function CapacityCard() {
  const maxConcurrentAgents = useItervoxStore((s) => s.snapshot?.maxConcurrentAgents ?? 0);
  const { bumpWorkers } = useSettingsActions();
  const [adjusting, setAdjusting] = useState(false);

  const handleBump = async (delta: number) => {
    if (adjusting) return;
    setAdjusting(true);
    await bumpWorkers(delta);
    setAdjusting(false);
  };

  return (
    <div className="border-theme-line bg-theme-panel rounded-lg border p-4">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-theme-text text-sm font-medium">Max concurrent agents</p>
          <p className="text-theme-muted mt-0.5 text-xs">
            Maximum number of agents that can run at the same time across all hosts.
          </p>
        </div>
        <div className="ml-4 flex flex-shrink-0 items-center gap-2">
          <button
            onClick={() => {
              void handleBump(-1);
            }}
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
            className="text-theme-text font-mono text-base font-semibold tabular-nums"
            style={{ minWidth: 24, textAlign: 'center' }}
          >
            {maxConcurrentAgents}
          </span>
          <button
            onClick={() => {
              void handleBump(1);
            }}
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
