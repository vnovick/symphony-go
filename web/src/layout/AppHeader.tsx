import { useEffect, useState, startTransition } from 'react';
import { useShallow } from 'zustand/react/shallow';
import { useItervoxStore } from '../store/itervoxStore';
import { MobileMenuButton } from '../components/ui/MobileMenuButton';

const AppHeader: React.FC<{ onMenuClick?: () => void }> = ({ onMenuClick }) => {
  const sseConnected = useItervoxStore((s) => s.sseConnected);
  const { running, paused, retrying, maxAgents, agentMode, hasSnapshot } = useItervoxStore(
    useShallow((s) => ({
      running: s.snapshot?.running.length ?? 0,
      paused: s.snapshot?.paused.length ?? 0,
      retrying: s.snapshot?.retrying.length ?? 0,
      maxAgents: s.snapshot?.maxConcurrentAgents ?? 0,
      agentMode: s.snapshot?.agentMode ?? '',
      hasSnapshot: s.snapshot !== null,
    })),
  );
  const [timedOut, setTimedOut] = useState(false);
  const orchestratorState = running > 0 ? 'running' : 'idle';
  const pct = maxAgents > 0 ? Math.round((running / maxAgents) * 100) : 0;

  // After 6 s without a snapshot, flip from "Connecting" to "Disconnected"
  useEffect(() => {
    if (hasSnapshot || sseConnected) {
      startTransition(() => {
        setTimedOut(false);
      });
      return;
    }
    const t = setTimeout(() => {
      setTimedOut(true);
    }, 6000);
    return () => {
      clearTimeout(t);
    };
  }, [hasSnapshot, sseConnected]);

  const liveLabel = sseConnected
    ? 'Live'
    : hasSnapshot
      ? 'Reconnecting\u2026'
      : timedOut
        ? 'Disconnected'
        : 'Connecting\u2026';

  return (
    <header className="bg-theme-bg-soft border-theme-line sticky top-0 z-30 flex items-center gap-3 border-b px-4 py-2 text-sm">
      {/* Mobile menu button */}
      {onMenuClick && <MobileMenuButton onClick={onMenuClick} />}

      {/* Live pulse */}
      <span className="flex items-center gap-2">
        <span className="relative flex h-2.5 w-2.5">
          {running > 0 && (
            <span className="bg-theme-success absolute inline-flex h-full w-full animate-ping rounded-full opacity-75" />
          )}
          <span
            className={`relative inline-flex h-2.5 w-2.5 rounded-full ${sseConnected ? 'bg-theme-success' : 'bg-theme-danger'}`}
          />
        </span>
        <span className="text-theme-text-secondary">{liveLabel}</span>
      </span>

      {/* Orchestrator state */}
      <span className="bg-theme-bg-elevated text-theme-text-secondary rounded px-2 py-0.5 font-mono text-xs">
        {orchestratorState}
      </span>

      {/* Running count */}
      {running > 0 && (
        <span className="text-theme-success flex items-center gap-1.5">
          <strong>{running}</strong>
          <span className="text-theme-text-secondary">running</span>
        </span>
      )}

      {paused > 0 && (
        <span className="bg-theme-danger-soft text-theme-danger rounded-full px-2 py-0.5 text-xs">
          {paused} paused
        </span>
      )}

      {retrying > 0 && (
        <span className="bg-theme-warning-soft text-theme-warning rounded-full px-2 py-0.5 text-xs">
          ↻ {retrying} retrying
        </span>
      )}

      {agentMode === 'subagents' && (
        <span
          className="rounded-full px-2 py-0.5 text-xs"
          style={{ background: 'var(--accent-soft)', color: 'var(--purple)' }}
        >
          sub-agents
        </span>
      )}

      {/* Capacity bar — hidden on mobile */}
      {maxAgents > 0 && (
        <span className="ml-2 hidden items-center gap-2 md:flex">
          <span className="text-theme-muted text-xs">capacity</span>
          <span className="bg-theme-bg-elevated h-1.5 w-20 overflow-hidden rounded-full">
            <span
              className="block h-full rounded-full transition-all"
              style={{
                width: `${String(pct)}%`,
                background:
                  pct >= 90 ? 'var(--danger)' : pct >= 60 ? 'var(--warning)' : 'var(--success)',
              }}
            />
          </span>
          <span className="text-theme-text-secondary font-mono text-xs">
            {running}/{maxAgents}
          </span>
        </span>
      )}
    </header>
  );
};

export default AppHeader;
