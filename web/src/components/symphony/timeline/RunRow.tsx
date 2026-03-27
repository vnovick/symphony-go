import { memo } from 'react';
import { fmtMs } from '../../../utils/format';
import type { NormalisedSession, SubagentSegment } from './types';
import { clamp01 } from './types';
import { SubagentBar } from './SubagentBar';

interface RunRowProps {
  session: NormalisedSession;
  subagents: SubagentSegment[];
  viewStart: number;
  viewEnd: number;
  expanded: boolean;
  selectedSubagentIdx: number | null;
  runNumber: number;
  onToggleExpand: () => void;
  onSelectSubagent: (idx: number | null) => void;
}

export const RunRow = memo(function RunRow({
  session,
  subagents,
  viewStart,
  viewEnd,
  expanded,
  selectedSubagentIdx,
  runNumber,
  onToggleExpand,
  onSelectSubagent,
}: RunRowProps) {
  const span = viewEnd - viewStart;
  const start = new Date(session.startedAt).getTime();
  const end = session.finishedAt
    ? new Date(session.finishedAt).getTime()
    : start + Math.max(session.elapsedMs, 1000);

  const barLeft = clamp01((start - viewStart) / span);
  const barRight = clamp01((end - viewStart) / span);
  const barWidth = Math.max(barRight - barLeft, 0.02);

  const isLive = session.status === 'live';
  const isSucceeded = session.status === 'succeeded';
  const isFailed = session.status === 'failed';

  const barBg = isLive
    ? 'linear-gradient(90deg, var(--accent), var(--teal))'
    : isSucceeded
      ? 'linear-gradient(90deg, var(--success), #16a34a)'
      : isFailed
        ? 'linear-gradient(90deg, var(--danger), #dc2626)'
        : 'linear-gradient(90deg, #52525b, #3f3f46)';

  const timeLabel = new Date(session.startedAt).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });

  return (
    <>
      <div
        className="flex cursor-pointer items-center gap-2 rounded transition-colors hover:bg-[var(--bg-soft)]"
        style={{ minHeight: 44, padding: '8px 0' }}
        onClick={onToggleExpand}
      >
        <span className="w-4 shrink-0 text-center text-[12px]" style={{ color: subagents.length > 0 ? 'var(--text-secondary)' : 'transparent' }}>
          {expanded ? '▼' : '▶'}
        </span>

        <span
          className="w-[120px] shrink-0 font-mono text-[11px] leading-tight"
          title={session.startedAt}
        >
          <span className="text-theme-accent-strong font-semibold">#{runNumber}</span>
          <span className="text-theme-muted"> · {timeLabel}</span>
        </span>

        <div
          className="relative flex-1 overflow-hidden rounded bg-theme-bg-soft"
          style={{ height: 24 }}
        >
          <div
            className="absolute top-0 flex h-full items-center rounded"
            style={{
              left: `${String(barLeft * 100)}%`,
              width: `${String(barWidth * 100)}%`,
              background: barBg,
            }}
            title={`${session.identifier} — ${fmtMs(session.elapsedMs)}`}
          />
          {subagents.map((sa, si) => (
            <div
              key={si}
              className="absolute top-0 z-10 h-full w-0.5"
              style={{ left: `${String((barLeft + sa.startFrac * barWidth) * 100)}%`, background: 'rgba(255,255,255,0.4)' }}
            />
          ))}
        </div>

        <span className="w-[60px] shrink-0 text-right font-mono text-[11px] text-theme-text-secondary">
          {fmtMs(session.elapsedMs)}
        </span>

        {!isLive && (
          <span
            className="shrink-0 rounded-full px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-[0.03em]"
            style={
              isSucceeded
                ? { background: 'var(--success-soft)', color: 'var(--success-strong)' }
                : isFailed
                  ? { background: 'var(--danger-soft)', color: 'var(--danger)' }
                  : { background: 'rgba(113,113,122,0.15)', color: 'var(--text-secondary)' }
            }
          >
            {isSucceeded ? 'done' : isFailed ? 'failed' : 'cancelled'}
          </span>
        )}
      </div>

      {expanded && (
        <div className="space-y-0.5 pb-1">
          <div
            className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 transition-colors"
            style={{
              paddingLeft: 24,
              background: selectedSubagentIdx === null ? 'var(--accent-soft)' : 'transparent',
            }}
            onClick={() => { onSelectSubagent(null); }}
          >
            <span className="text-[12px] flex-shrink-0 text-theme-muted">◈</span>
            <span
              className="w-[80px] shrink-0 truncate font-mono text-[11px]"
              style={{ color: selectedSubagentIdx === null ? 'var(--accent-strong)' : 'var(--text-secondary)' }}
            >
              Main
            </span>
            <div
              className="relative flex-1 overflow-hidden rounded bg-theme-bg-soft"
              style={{ height: 16 }}
            >
              <div
                className="absolute top-0 h-full rounded"
                style={{ left: `${String(barLeft * 100)}%`, width: `${String(barWidth * 100)}%`, background: barBg }}
              />
            </div>
          </div>

          {subagents.map((sa, si) => (
            <SubagentBar
              key={`${sa.name}-${String(si)}`}
              segment={sa}
              colorIdx={si}
              runBarLeft={barLeft}
              runBarWidth={barWidth}
              selected={selectedSubagentIdx === si}
              onSelect={() => { onSelectSubagent(selectedSubagentIdx === si ? null : si); }}
            />
          ))}
        </div>
      )}
    </>
  );
});
