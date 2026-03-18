import { useEffect, useMemo, useRef, useState } from 'react';

import PageMeta from '../../components/common/PageMeta';
import { useSymphonyStore } from '../../store/symphonyStore';
import type { HistoryRow, IssueLogEntry, RunningRow } from '../../types/symphony';
import { useIssueLogs } from '../../queries/logs';
import { entryStyle } from '../../utils/logFormatting';
import { useStableValue } from '../../hooks/useStableValue';

// ─── Helpers ──────────────────────────────────────────────────────────────────

const clamp01 = (x: number) => Math.max(0, Math.min(1, x));

function fmtMs(ms: number): string {
  const s = Math.floor(ms / 1000);
  return `${String(Math.floor(s / 60))}m ${String(s % 60).padStart(2, '0')}s`;
}

// ─── Data model ───────────────────────────────────────────────────────────────

interface NormalisedSession {
  identifier: string;
  title?: string;
  startedAt: string;
  finishedAt?: string;
  elapsedMs: number;
  turnCount: number;
  tokens: number;
  status: 'live' | 'succeeded' | 'failed' | 'cancelled';
}

function fromRunning(r: RunningRow): NormalisedSession {
  return {
    identifier: r.identifier,
    startedAt: r.startedAt,
    elapsedMs: r.elapsedMs,
    turnCount: r.turnCount,
    tokens: r.tokens,
    status: 'live',
  };
}

function fromHistory(h: HistoryRow): NormalisedSession {
  return {
    identifier: h.identifier,
    title: h.title,
    startedAt: h.startedAt,
    finishedAt: h.finishedAt,
    elapsedMs: h.elapsedMs,
    turnCount: h.turnCount,
    tokens: h.tokens,
    status: h.status,
  };
}

// All runs for a single issue identifier, grouped for the sidebar + timeline.
interface IssueGroup {
  identifier: string;
  runs: NormalisedSession[]; // sorted by startedAt asc
  latestStatus: NormalisedSession['status'];
  latestStartedAt: string;
}

// A subagent "segment" derived from log entries — positional, no real timestamps.
interface SubagentSegment {
  name: string;
  startFrac: number; // fraction of total log entries
  endFrac: number;
  logSlice: IssueLogEntry[];
}

function extractSubagents(logs: IssueLogEntry[]): SubagentSegment[] {
  if (logs.length === 0) return [];
  const total = logs.length;
  const markers = logs.map((e, i) => ({ e, i })).filter(({ e }) => e.event === 'subagent');
  return markers.map(({ e, i }, si) => {
    const nextIdx = markers[si + 1]?.i ?? total;
    return {
      name: e.message.slice(0, 80),
      startFrac: i / total,
      endFrac: nextIdx / total,
      logSlice: logs.slice(i, nextIdx),
    };
  });
}

// ─── Log panel ────────────────────────────────────────────────────────────────

interface AgentLogPanelProps {
  identifier: string;
  /** When provided, render this slice instead of polling live logs. */
  logSlice?: IssueLogEntry[];
}

function AgentLogPanel({ identifier, logSlice }: AgentLogPanelProps) {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const isLive = !!(
    snapshot?.running.some((r) => r.identifier === identifier) ||
    snapshot?.retrying.some((r) => r.identifier === identifier)
  );
  const { data: liveEntries = [] } = useIssueLogs(identifier, isLive);
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const followRef = useRef(true);
  const [isFollowing, setIsFollowing] = useState(true);

  const entries = logSlice ?? liveEntries;

  const onScroll = () => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const atBottom = scrollHeight - scrollTop - clientHeight < 40;
    followRef.current = atBottom;
    setIsFollowing(atBottom);
  };

  useEffect(() => {
    if (followRef.current) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [entries]);

  const scrollToBottom = () => {
    followRef.current = true;
    setIsFollowing(true);
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {!isFollowing && (
        <div className="flex flex-shrink-0 justify-end border-b border-gray-700 bg-gray-900/80 px-3 py-1">
          <button
            onClick={scrollToBottom}
            className="text-[10px] font-medium text-blue-400 transition-colors hover:text-blue-300"
          >
            ▼ Jump to live
          </button>
        </div>
      )}
      <div
        ref={containerRef}
        onScroll={onScroll}
        className="flex-1 overflow-y-auto bg-gray-950 p-3 font-mono text-xs leading-[1.6]"
      >
        {entries.length === 0 ? (
          <p className="text-gray-600 italic">No logs yet for {identifier}.</p>
        ) : (
          entries.map((entry, i) => {
            const style = entryStyle(entry.event, entry.level);
            return (
              <div key={i} className={`mb-0.5 flex gap-1.5 ${style.textClass}`}>
                {entry.time && (
                  <span className="w-[4.5rem] shrink-0 text-right text-gray-600">{entry.time}</span>
                )}
                <span className="shrink-0">{style.prefixChar}</span>
                <span className="break-all whitespace-pre-wrap">{entry.message}</span>
              </div>
            );
          })
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

// ─── Subagent bar ─────────────────────────────────────────────────────────────

interface SubagentBarProps {
  segment: SubagentSegment;
  runBarLeft: number; // fraction — where the parent run bar starts on the track
  runBarWidth: number; // fraction — width of the parent run bar on the track
  selected: boolean;
  onSelect: () => void;
}

function SubagentBar({ segment, runBarLeft, runBarWidth, selected, onSelect }: SubagentBarProps) {
  // Position the subagent bar within the parent run's extent on the track.
  const barLeft = runBarLeft + segment.startFrac * runBarWidth;
  const barWidth = Math.max((segment.endFrac - segment.startFrac) * runBarWidth, 0.005);

  const actionTicks = segment.logSlice
    .map((e, i) => ({ e, i }))
    .filter(({ e }) => e.event === 'action');

  return (
    <div
      className={`ml-8 flex cursor-pointer items-center gap-3 rounded-md px-1 py-0.5 transition-colors ${
        selected
          ? 'bg-purple-50 dark:bg-purple-950/60'
          : 'hover:bg-gray-50 dark:hover:bg-gray-800/40'
      }`}
      style={{ minHeight: 34 }}
      onClick={onSelect}
    >
      {/* Label */}
      <div
        className={`w-20 shrink-0 truncate text-right font-mono text-[10px] ${
          selected ? 'text-purple-600 dark:text-purple-400' : 'text-gray-400 dark:text-gray-500'
        }`}
        title={segment.name}
      >
        🤖 {segment.name}
      </div>

      {/* Track — shares same width as run track above */}
      <div className="relative h-4 flex-1 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/60">
        <div
          className="absolute top-0 flex h-full items-center rounded"
          style={{
            left: `${String(barLeft * 100)}%`,
            width: `${String(barWidth * 100)}%`,
            background: selected
              ? 'linear-gradient(90deg, #7c3aed 0%, #6d28d9 100%)'
              : 'linear-gradient(90deg, #a78bfa 0%, #8b5cf6 100%)',
          }}
          title={segment.name}
        >
          {barWidth > 0.06 && (
            <span className="truncate pl-1 text-[9px] text-white/90">{segment.name}</span>
          )}
        </div>
        {/* Tool-call ticks */}
        {actionTicks.map(({ i }) => {
          const frac = barLeft + (i / Math.max(segment.logSlice.length, 1)) * barWidth;
          return (
            <div
              key={i}
              className="absolute top-0 z-10 h-full w-px bg-white/40"
              style={{ left: `${String(frac * 100)}%` }}
            />
          );
        })}
      </div>

      <div className="w-14 shrink-0 text-right text-[10px] text-gray-400 dark:text-gray-500">
        {segment.logSlice.length} ev
      </div>
    </div>
  );
}

// ─── Run row ──────────────────────────────────────────────────────────────────

interface RunRowProps {
  session: NormalisedSession;
  subagents: SubagentSegment[];
  viewStart: number;
  viewEnd: number;
  expanded: boolean;
  selectedSubagentIdx: number | null;
  onToggleExpand: () => void;
  onSelectSubagent: (idx: number | null) => void;
}

function RunRow({
  session,
  subagents,
  viewStart,
  viewEnd,
  expanded,
  selectedSubagentIdx,
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
  const barWidth = Math.max(barRight - barLeft, 0.005);

  const isLive = session.status === 'live';
  const isSucceeded = session.status === 'succeeded';
  const isFailed = session.status === 'failed';

  const barBg = isLive
    ? 'linear-gradient(90deg, #3b82f6 0%, #6366f1 100%)'
    : isSucceeded
      ? 'linear-gradient(90deg, #22c55e 0%, #16a34a 100%)'
      : isFailed
        ? 'linear-gradient(90deg, #ef4444 0%, #dc2626 100%)'
        : 'linear-gradient(90deg, #9ca3af 0%, #6b7280 100%)';

  const timeLabel = new Date(session.startedAt).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });

  return (
    <>
      <div
        className="flex cursor-pointer items-center gap-3 rounded-lg px-1 py-0.5 transition-colors hover:bg-gray-50 dark:hover:bg-gray-800/40"
        style={{ minHeight: 44 }}
        onClick={onToggleExpand}
      >
        {/* Chevron + time label */}
        <div className="flex w-24 shrink-0 items-center justify-end gap-1.5">
          <span className="text-[10px] text-gray-400 select-none dark:text-gray-500">
            {subagents.length > 0 ? (expanded ? '▼' : '▶') : ' '}
          </span>
          <span
            className="truncate font-mono text-[10px] text-gray-500 dark:text-gray-400"
            title={session.startedAt}
          >
            {timeLabel}
          </span>
        </div>

        {/* Track */}
        <div className="relative h-6 flex-1 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/60">
          <div
            className="absolute top-0 flex h-full items-center rounded"
            style={{
              left: `${String(barLeft * 100)}%`,
              width: `${String(barWidth * 100)}%`,
              background: barBg,
            }}
            title={`${session.identifier} — ${fmtMs(session.elapsedMs)}`}
          >
            {barWidth > 0.04 && (
              <span className="truncate pl-1.5 text-[10px] text-white/90">
                {!isLive && (isSucceeded ? '✓ ' : isFailed ? '✗ ' : '⏸ ')}
                turn {session.turnCount} · {session.tokens.toLocaleString()} tok
              </span>
            )}
          </div>
          {/* Subagent boundary ticks */}
          {subagents.map((sa, si) => (
            <div
              key={si}
              className="absolute top-0 z-10 h-full w-0.5 bg-white/40"
              style={{ left: `${String((barLeft + sa.startFrac * barWidth) * 100)}%` }}
              title={sa.name}
            />
          ))}
        </div>

        <div className="w-14 shrink-0 text-right text-xs text-gray-500 dark:text-gray-400">
          {fmtMs(session.elapsedMs)}
        </div>
      </div>

      {/* Subagent drill-down */}
      {expanded && (
        <div className="space-y-0.5 pb-1">
          {subagents.length === 0 ? (
            <p className="ml-8 px-1 py-1 text-[11px] text-gray-400 italic dark:text-gray-500">
              No subagent events recorded
            </p>
          ) : (
            subagents.map((sa, si) => (
              <SubagentBar
                key={si}
                segment={sa}
                runBarLeft={barLeft}
                runBarWidth={barWidth}
                selected={selectedSubagentIdx === si}
                onSelect={() => {
                  onSelectSubagent(selectedSubagentIdx === si ? null : si);
                }}
              />
            ))
          )}
        </div>
      )}
    </>
  );
}

// ─── Time axis ────────────────────────────────────────────────────────────────

function TimeAxis({ viewStart, viewEnd }: { viewStart: number; viewEnd: number }) {
  const span = viewEnd - viewStart;
  const rawStep = span / 6;
  const steps = [30_000, 60_000, 5 * 60_000, 10 * 60_000, 30 * 60_000, 60 * 60_000];
  const step = steps.find((s) => s >= rawStep) ?? steps[steps.length - 1];

  const ticks: number[] = [];
  const first = Math.ceil(viewStart / step) * step;
  for (let t = first; t <= viewEnd; t += step) ticks.push(t);

  return (
    <div className="relative mr-[4.5rem] ml-[7rem] h-5 border-b border-gray-200 dark:border-gray-700">
      {ticks.map((t) => {
        const pct = ((t - viewStart) / span) * 100;
        if (pct < 0 || pct > 100) return null;
        const label = new Date(t).toLocaleTimeString([], {
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false,
        });
        return (
          <div
            key={t}
            className="absolute top-0 flex flex-col items-center"
            style={{ left: `${String(pct)}%`, transform: 'translateX(-50%)' }}
          >
            <span className="text-[9px] whitespace-nowrap text-gray-400 dark:text-gray-500">
              {label}
            </span>
            <div className="mt-0.5 h-1.5 w-px bg-gray-300 dark:bg-gray-600" />
          </div>
        );
      })}
      <NowMarker viewStart={viewStart} viewEnd={viewEnd} />
    </div>
  );
}

function NowMarker({ viewStart, viewEnd }: { viewStart: number; viewEnd: number }) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const id = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => {
      clearInterval(id);
    };
  }, []);
  const pct = ((now - viewStart) / (viewEnd - viewStart)) * 100;
  if (pct < 0 || pct > 100) return null;
  return (
    <div className="pointer-events-none absolute top-0 h-full" style={{ left: `${String(pct)}%` }}>
      <div className="h-full w-px bg-red-400/60" />
    </div>
  );
}

// ─── Stable fallbacks ────────────────────────────────────────────────────────
const EMPTY_RUNNING: RunningRow[] = [];
const EMPTY_HISTORY: HistoryRow[] = [];

// ─── Main component ───────────────────────────────────────────────────────────

export default function Timeline() {
  const rawRunning = useSymphonyStore((s) => s.snapshot?.running ?? EMPTY_RUNNING);
  const rawHistory = useSymphonyStore((s) => s.snapshot?.history ?? EMPTY_HISTORY);
  const liveRunning = useStableValue(rawRunning, 5000);

  const liveSessions = useMemo(() => liveRunning.map(fromRunning), [liveRunning]);
  const historySessions = useMemo(() => rawHistory.map(fromHistory), [rawHistory]);

  // Merge history + live; live overrides same-identifier history entries.
  // Sort by startedAt asc for stable ordering across status changes.
  const allSessions = useMemo<NormalisedSession[]>(() => {
    const liveIds = new Set(liveRunning.map((r) => r.identifier));
    const historyOnly = historySessions.filter((h) => !liveIds.has(h.identifier));
    return [...historyOnly, ...liveSessions].sort(
      (a, b) => new Date(a.startedAt).getTime() - new Date(b.startedAt).getTime(),
    );
  }, [historySessions, liveSessions, liveRunning]);

  // Group into per-issue buckets for the sidebar.
  const issueGroups = useMemo<IssueGroup[]>(() => {
    const map = new Map<string, NormalisedSession[]>();
    for (const s of allSessions) {
      const existing = map.get(s.identifier) ?? [];
      map.set(s.identifier, [...existing, s]);
    }
    return Array.from(map.entries())
      .map(([identifier, runs]) => {
        const hasLive = runs.some((r) => r.status === 'live');
        const hasFailed = runs.some((r) => r.status === 'failed');
        const latestStatus: NormalisedSession['status'] = hasLive
          ? 'live'
          : hasFailed
            ? 'failed'
            : runs.every((r) => r.status === 'succeeded')
              ? 'succeeded'
              : 'cancelled';
        const latestStartedAt = runs[runs.length - 1].startedAt;
        return { identifier, runs, latestStatus, latestStartedAt };
      })
      .sort((a, b) => {
        // Live issues float to top, then sort by most recent run descending.
        if (a.latestStatus === 'live' && b.latestStatus !== 'live') return -1;
        if (b.latestStatus === 'live' && a.latestStatus !== 'live') return 1;
        return new Date(b.latestStartedAt).getTime() - new Date(a.latestStartedAt).getTime();
      });
  }, [allSessions]);

  // ── Selection state ──────────────────────────────────────────────────────────
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // Which run (by startedAt) is expanded to show subagents.
  const [expandedRunAt, setExpandedRunAt] = useState<string | null>(null);
  // Which subagent index is selected within the expanded run.
  const [selectedSubagentIdx, setSelectedSubagentIdx] = useState<number | null>(null);

  // Auto-select first issue when data arrives.
  useEffect(() => {
    if (issueGroups.length === 0) return;
    if (selectedId === null || !issueGroups.find((g) => g.identifier === selectedId)) {
      setSelectedId(issueGroups[0].identifier);
    }
  }, [issueGroups, selectedId]);

  // Reset drill-down state when selected issue changes.
  useEffect(() => {
    setExpandedRunAt(null);
    setSelectedSubagentIdx(null);
  }, [selectedId]);

  const selectedGroup = useMemo(
    () => issueGroups.find((g) => g.identifier === selectedId) ?? null,
    [issueGroups, selectedId],
  );

  // ── Log fetching — only for the selected identifier ──────────────────────────
  const isSelectedLive = selectedGroup?.runs.some((r) => r.status === 'live') ?? false;
  const { data: logsForSelected = [] } = useIssueLogs(selectedId ?? '', isSelectedLive);

  // ── Viewport — scoped to selected issue's runs ───────────────────────────────
  const viewportRef = useRef<{ start: number; end: number } | null>(null);
  const prevSelectedId = useRef<string | null>(null);

  // Reset viewport when issue selection changes so each issue gets a fresh fit.
  if (selectedId !== prevSelectedId.current) {
    viewportRef.current = null;
    prevSelectedId.current = selectedId;
  }

  const { viewStart, viewEnd } = useMemo(() => {
    // eslint-disable-next-line react-hooks/purity
    const now = Date.now();
    const runs = selectedGroup?.runs ?? [];
    const times = runs.map((r) => new Date(r.startedAt).getTime());
    const earliest = times.length > 0 ? Math.min(...times) : now - 10 * 60_000;
    const wantStart = earliest - 2 * 60_000;

    const hasLive = runs.some((r) => r.status === 'live');
    const wantEnd = hasLive
      ? now + 10 * 60_000
      : runs.length > 0
        ? Math.max(
            ...runs.map((r) =>
              r.finishedAt
                ? new Date(r.finishedAt).getTime()
                : new Date(r.startedAt).getTime() + r.elapsedMs,
            ),
          ) +
          2 * 60_000
        : now + 10 * 60_000;

    if (!viewportRef.current) {
      viewportRef.current = { start: wantStart, end: wantEnd };
    } else {
      if (wantStart < viewportRef.current.start) viewportRef.current.start = wantStart;
      if (wantEnd > viewportRef.current.end) viewportRef.current.end = wantEnd;
    }
    return { viewStart: viewportRef.current.start, viewEnd: viewportRef.current.end };
  }, [selectedGroup]);

  // ── Subagent segments — derived from logs ────────────────────────────────────
  const subagents = useMemo(() => extractSubagents(logsForSelected), [logsForSelected]);

  // Log panel shows subagent slice when one is selected, else full logs.
  const logPanelSlice =
    selectedSubagentIdx !== null ? subagents[selectedSubagentIdx]?.logSlice : undefined;

  const selectedSession = selectedGroup?.runs.find((r) => r.startedAt === expandedRunAt) ?? null;

  const anyLive = allSessions.some((s) => s.status === 'live');

  // ── Render ───────────────────────────────────────────────────────────────────
  return (
    <>
      <PageMeta title="Symphony | Timeline" description="Agent timeline" />

      <div className="flex" style={{ height: 'calc(100vh - 64px)' }}>
        {/* ── Left sidebar: issue list ─────────────────────────────────────── */}
        <div className="flex w-44 flex-shrink-0 flex-col border-r border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900">
          <div className="flex items-center justify-between border-b border-gray-200 px-3 py-3 dark:border-gray-700">
            <p className="text-[11px] font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500">
              Issues
            </p>
            {issueGroups.length > 0 && (
              <span className="text-[10px] text-gray-400 dark:text-gray-500">
                {issueGroups.length}
              </span>
            )}
          </div>
          <div className="flex-1 overflow-y-auto">
            {issueGroups.length === 0 ? (
              <p className="px-3 py-4 text-xs text-gray-400 italic dark:text-gray-500">
                No sessions yet
              </p>
            ) : (
              issueGroups.map((group) => {
                const isSelected = selectedId === group.identifier;
                const dotClass =
                  group.latestStatus === 'live'
                    ? 'bg-green-400 animate-pulse'
                    : group.latestStatus === 'succeeded'
                      ? 'bg-green-500'
                      : group.latestStatus === 'failed'
                        ? 'bg-red-500'
                        : 'bg-gray-400';
                return (
                  <button
                    key={group.identifier}
                    onClick={() => {
                      setSelectedId(group.identifier);
                    }}
                    className={`flex w-full items-center gap-2 border-b border-gray-100 px-3 py-2.5 text-left transition-colors dark:border-gray-800 ${
                      isSelected
                        ? 'bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300'
                        : 'text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800/50'
                    }`}
                  >
                    <span className={`h-2 w-2 flex-shrink-0 rounded-full ${dotClass}`} />
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-mono text-xs">{group.identifier}</div>
                      {group.runs.length > 1 && (
                        <div className="text-[10px] text-gray-400 dark:text-gray-500">
                          {group.runs.length} runs
                        </div>
                      )}
                    </div>
                  </button>
                );
              })
            )}
          </div>
        </div>

        {/* ── Right: timeline + logs ───────────────────────────────────────── */}
        <div className="flex flex-1 flex-col overflow-hidden">
          {/* Header */}
          <div className="flex flex-shrink-0 items-center justify-between border-b border-gray-200 px-4 py-3 md:px-6 dark:border-gray-800">
            <div>
              <h1 className="text-xl font-bold text-gray-900 dark:text-white">
                {selectedId ? `Timeline: ${selectedId}` : 'Timeline'}
              </h1>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {anyLive
                  ? 'Running · click a run bar to expand subagents'
                  : 'Completed sessions · click a run to drill down'}
              </p>
            </div>
            {selectedGroup && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-100 px-2.5 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                {selectedGroup.runs.some((r) => r.status === 'live') && (
                  <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-blue-500" />
                )}
                {selectedGroup.runs.length} run{selectedGroup.runs.length !== 1 ? 's' : ''}
              </span>
            )}
          </div>

          {/* Timeline bars — scrollable, max half the viewport */}
          <div
            className="flex-shrink-0 overflow-y-auto border-b border-gray-200 px-4 py-3 md:px-6 dark:border-gray-800"
            style={{ maxHeight: '50%' }}
          >
            {!selectedGroup ? (
              <div className="flex h-12 items-center justify-center text-sm text-gray-400 dark:text-gray-500">
                Select an issue from the sidebar to view its timeline
              </div>
            ) : (
              <>
                <TimeAxis viewStart={viewStart} viewEnd={viewEnd} />
                <div className="mt-1 space-y-0.5">
                  {selectedGroup.runs.map((run) => {
                    const isExpanded = expandedRunAt === run.startedAt;
                    return (
                      <RunRow
                        key={`${run.identifier}-${run.startedAt}`}
                        session={run}
                        subagents={isExpanded ? subagents : []}
                        viewStart={viewStart}
                        viewEnd={viewEnd}
                        expanded={isExpanded}
                        selectedSubagentIdx={isExpanded ? selectedSubagentIdx : null}
                        onToggleExpand={() => {
                          setExpandedRunAt((prev) =>
                            prev === run.startedAt ? null : run.startedAt,
                          );
                          setSelectedSubagentIdx(null);
                        }}
                        onSelectSubagent={setSelectedSubagentIdx}
                      />
                    );
                  })}
                </div>
              </>
            )}
          </div>

          {/* Log stream — fills remaining space */}
          <div className="flex flex-1 flex-col overflow-hidden">
            {selectedId ? (
              <>
                <div className="flex flex-shrink-0 items-center gap-2 border-b border-gray-200 bg-gray-50 px-4 py-1.5 dark:border-gray-700 dark:bg-gray-900/50">
                  <p className="text-[11px] font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500">
                    {selectedSubagentIdx !== null && subagents[selectedSubagentIdx]
                      ? `Subagent — ${subagents[selectedSubagentIdx].name}`
                      : `Logs — ${selectedId}`}
                  </p>
                  {selectedSession && selectedSession.status !== 'live' && (
                    <span
                      className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${
                        selectedSession.status === 'succeeded'
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                          : selectedSession.status === 'failed'
                            ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                            : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                      }`}
                    >
                      {selectedSession.status}
                    </span>
                  )}
                  {selectedSession?.status === 'live' && (
                    <span className="flex items-center gap-1 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                      <span className="h-1 w-1 animate-pulse rounded-full bg-blue-500" />
                      live
                    </span>
                  )}
                  {selectedSubagentIdx !== null && (
                    <button
                      onClick={() => {
                        setSelectedSubagentIdx(null);
                      }}
                      className="ml-auto text-[10px] text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                    >
                      ✕ show all logs
                    </button>
                  )}
                </div>
                <div className="flex-1 overflow-hidden">
                  <AgentLogPanel identifier={selectedId} logSlice={logPanelSlice} />
                </div>
              </>
            ) : (
              <div className="flex flex-1 items-center justify-center text-sm text-gray-400 dark:text-gray-500">
                Select an issue from the sidebar to view logs
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
