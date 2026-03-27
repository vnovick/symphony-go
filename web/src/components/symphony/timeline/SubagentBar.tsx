import { memo } from 'react';
import type { SubagentSegment } from './types';
import { SUBAGENT_COLORS } from './types';

interface SubagentBarProps {
  segment: SubagentSegment;
  colorIdx: number;
  runBarLeft: number;
  runBarWidth: number;
  selected: boolean;
  onSelect: () => void;
}

export const SubagentBar = memo(function SubagentBar({
  segment,
  colorIdx,
  runBarLeft,
  runBarWidth,
  selected,
  onSelect,
}: SubagentBarProps) {
  const barLeft = runBarLeft + segment.startFrac * runBarWidth;
  const barWidth = Math.max((segment.endFrac - segment.startFrac) * runBarWidth, 0.005);
  const colors = SUBAGENT_COLORS[colorIdx % SUBAGENT_COLORS.length];

  const tokApprox = segment.logSlice.length > 0
    ? `${String(Math.round(segment.logSlice.length * 80 / 1000))}k tok`
    : `${String(segment.logSlice.length)} ev`;

  return (
    <div
      className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 transition-colors"
      style={{
        paddingLeft: 24,
        background: selected ? 'var(--purple-soft)' : 'transparent',
      }}
      onClick={onSelect}
    >
      <span className="text-[12px] flex-shrink-0" style={{ color: colors.text }}>↗</span>

      <span
        className="w-[80px] shrink-0 truncate font-mono text-[11px]"
        style={{ color: colors.text }}
        title={segment.name}
      >
        {segment.name.slice(0, 8)}
      </span>

      <div
        className="relative flex-1 overflow-hidden rounded bg-theme-bg-soft"
        style={{ height: 16 }}
      >
        <div
          className="absolute top-0 h-full rounded"
          style={{
            left: `${String(barLeft * 100)}%`,
            width: `${String(barWidth * 100)}%`,
            background: colors.bar,
          }}
        />
        {segment.logSlice
          .map((e, i) => ({ e, i }))
          .filter(({ e }) => e.event === 'action')
          .map(({ i }) => {
            const frac = barLeft + (i / Math.max(segment.logSlice.length, 1)) * barWidth;
            return (
              <div
                key={i}
                className="absolute top-0 z-10 h-full w-px"
                style={{ left: `${String(frac * 100)}%`, background: 'rgba(255,255,255,0.4)' }}
              />
            );
          })}
      </div>

      <span className="w-[56px] shrink-0 text-right text-[10px] text-theme-text-secondary">
        {tokApprox}
      </span>
    </div>
  );
});
