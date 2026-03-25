import { useEffect, useState } from 'react';
import { useSymphonyStore } from '../store/symphonyStore';

const AppHeader: React.FC = () => {
  const sseConnected = useSymphonyStore((s) => s.sseConnected);
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const [timedOut, setTimedOut] = useState(false);

  const running = snapshot?.running.length ?? 0;
  const paused = snapshot?.paused.length ?? 0;
  const retrying = snapshot?.retrying.length ?? 0;
  const maxAgents = snapshot?.maxConcurrentAgents ?? 0;
  const agentMode = snapshot?.agentMode ?? '';
  const orchestratorState = running > 0 ? 'running' : 'idle';
  const pct = maxAgents > 0 ? Math.round((running / maxAgents) * 100) : 0;

  // After 6 s without a snapshot, flip from "Connecting" to "Disconnected"
  useEffect(() => {
    if (snapshot ?? sseConnected) {
      setTimedOut(false);
      return;
    }
    const t = setTimeout(() => {
      setTimedOut(true);
    }, 6000);
    return () => {
      clearTimeout(t);
    };
  }, [snapshot, sseConnected]);

  const liveLabel = sseConnected
    ? 'Live'
    : snapshot
      ? 'Reconnecting\u2026'
      : timedOut
        ? 'Disconnected'
        : 'Connecting\u2026';

  return (
    <header
      className="sticky top-0 z-30 flex items-center gap-4 px-4 py-2 border-b text-sm"
      style={{
        background: 'var(--bg-soft)',
        borderColor: 'var(--line)',
      }}
    >
      {/* Live pulse */}
      <span className="flex items-center gap-2">
        <span className="relative flex h-2.5 w-2.5">
          {running > 0 && (
            <span
              className="absolute inline-flex h-full w-full animate-ping rounded-full opacity-75"
              style={{ background: 'var(--success)' }}
            />
          )}
          <span
            className="relative inline-flex h-2.5 w-2.5 rounded-full"
            style={{ background: sseConnected ? 'var(--success)' : 'var(--danger)' }}
          />
        </span>
        <span style={{ color: 'var(--text-secondary)' }}>{liveLabel}</span>
      </span>

      {/* Orchestrator state */}
      <span
        className="font-mono text-xs px-2 py-0.5 rounded"
        style={{ background: 'var(--bg-elevated)', color: 'var(--text-secondary)' }}
      >
        {orchestratorState}
      </span>

      {/* Running count */}
      {running > 0 && (
        <span className="flex items-center gap-1.5" style={{ color: 'var(--success)' }}>
          <strong>{running}</strong>
          <span style={{ color: 'var(--text-secondary)' }}>running</span>
        </span>
      )}

      {paused > 0 && (
        <span
          className="text-xs px-2 py-0.5 rounded-full"
          style={{ background: 'var(--danger-soft)', color: 'var(--danger)' }}
        >
          {paused} paused
        </span>
      )}

      {retrying > 0 && (
        <span
          className="text-xs px-2 py-0.5 rounded-full"
          style={{ background: 'var(--warning-soft)', color: 'var(--warning)' }}
        >
          ↻ {retrying} retrying
        </span>
      )}

      {agentMode === 'subagents' && (
        <span
          className="text-xs px-2 py-0.5 rounded-full"
          style={{ background: 'var(--accent-soft)', color: 'var(--purple)' }}
        >
          sub-agents
        </span>
      )}

      {/* Capacity bar */}
      {maxAgents > 0 && (
        <span className="flex items-center gap-2 ml-2">
          <span style={{ color: 'var(--muted)' }} className="text-xs">
            capacity
          </span>
          <span
            className="h-1.5 w-20 overflow-hidden rounded-full"
            style={{ background: 'var(--bg-elevated)' }}
          >
            <span
              className="h-full rounded-full transition-all block"
              style={{
                width: `${String(pct)}%`,
                background:
                  pct >= 90
                    ? 'var(--danger)'
                    : pct >= 60
                      ? 'var(--warning)'
                      : 'var(--success)',
              }}
            />
          </span>
          <span
            className="font-mono text-xs"
            style={{ color: 'var(--text-secondary)' }}
          >
            {running}/{maxAgents}
          </span>
        </span>
      )}

    </header>
  );
};

export default AppHeader;
