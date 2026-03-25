import { useEffect, useMemo, useRef, useState } from 'react';

import PageMeta from '../../components/common/PageMeta';
import { useSymphonyStore } from '../../store/symphonyStore';
import type { HistoryRow, IssueLogEntry, RunningRow } from '../../types/symphony';
import { useIssueLogs, useSubagentLogs } from '../../queries/logs';
import { toTermLine } from '../../utils/logFormatting';
import { fmtMs } from '../../utils/format';
import { useStableValue } from '../../hooks/useStableValue';
import { useClearIssueLogs, useClearIssueSubLogs } from '../../queries/issues';
import { Modal } from '../../components/ui/modal';

// ─── Helpers ──────────────────────────────────────────────────────────────────

const clamp01 = (x: number) => Math.max(0, Math.min(1, x));

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
  sessionId?: string;
}

function fromRunning(r: RunningRow): NormalisedSession {
  return {
    identifier: r.identifier,
    startedAt: r.startedAt,
    elapsedMs: r.elapsedMs,
    turnCount: r.turnCount,
    tokens: r.tokens,
    status: 'live',
    sessionId: r.sessionId,
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
    sessionId: h.sessionId,
  };
}

interface IssueGroup {
  identifier: string;
  runs: NormalisedSession[];
  latestStatus: NormalisedSession['status'];
  latestStartedAt: string;
}

interface SubagentSegment {
  name: string;
  startFrac: number;
  endFrac: number;
  logSlice: IssueLogEntry[];
}

function extractSubagents(logs: IssueLogEntry[], filterSessionId?: string): SubagentSegment[] {
  const filtered = filterSessionId
    ? logs.filter(e => e.sessionId === filterSessionId)
    : logs;
  if (filtered.length === 0) return [];
  const total = filtered.length;
  const markers = filtered.map((e, i) => ({ e, i })).filter(({ e }) => e.event === 'subagent');
  return markers.map(({ e, i }, si) => {
    const nextIdx = markers[si + 1]?.i ?? total;
    return {
      name: e.message.slice(0, 80),
      startFrac: i / total,
      endFrac: nextIdx / total,
      logSlice: filtered.slice(i, nextIdx),
    };
  });
}

// ─── Subagent colours (alternating accent → teal → purple) ───────────────────

const SUBAGENT_COLORS = [
  { text: 'var(--accent-strong)', bar: 'linear-gradient(90deg, #a855f7, #6366f1)' },
  { text: 'var(--teal)',          bar: 'linear-gradient(90deg, #14b8a6, #22c55e)' },
  { text: 'var(--accent-strong)', bar: 'linear-gradient(90deg, var(--accent), var(--teal))' },
];

// ─── Log panel ────────────────────────────────────────────────────────────────

interface AgentLogPanelProps {
  identifier: string;
  logSlice?: IssueLogEntry[];
}

function AgentLogPanel({ identifier, logSlice }: AgentLogPanelProps) {
  const isLive = useSymphonyStore(
    (s) =>
      !!(
        s.snapshot?.running.some((r) => r.identifier === identifier) ||
        s.snapshot?.retrying.some((r) => r.identifier === identifier)
      ),
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
    followRef.current = scrollHeight - scrollTop - clientHeight < 40;
    setIsFollowing(followRef.current);
  };

  useEffect(() => {
    if (followRef.current) bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [entries]);

  const scrollToBottom = () => {
    followRef.current = true;
    setIsFollowing(true);
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {!isFollowing && (
        <div
          className="flex flex-shrink-0 justify-end border-b px-3 py-1"
          style={{ borderColor: 'var(--line)', background: 'var(--panel)' }}
        >
          <button
            onClick={scrollToBottom}
            className="text-[10px] font-medium transition-colors hover:opacity-80"
            style={{ color: 'var(--accent)' }}
          >
            ▼ Jump to live
          </button>
        </div>
      )}
      <div
        ref={containerRef}
        onScroll={onScroll}
        className="flex-1 overflow-y-auto p-3 font-mono text-[12px] leading-[1.6]"
        style={{ background: 'var(--panel-dark)' }}
      >
        {entries.length === 0 ? (
          <p className="italic" style={{ color: 'var(--muted)' }}>
            No logs yet for {identifier}.
          </p>
        ) : (
          entries.map((entry) => {
            const line = toTermLine(entry);
            return (
              <div
                key={`${entry.time ?? ''}-${entry.event ?? ''}-${entry.message.slice(0, 24)}`}
                className="mb-0.5 flex gap-2"
              >
                {line.time && (
                  <span className="w-[50px] shrink-0" style={{ color: 'var(--muted)' }}>
                    {line.time}
                  </span>
                )}
                <span className="shrink-0" style={{ color: line.prefixColor }}>{line.prefix}</span>
                <span className="break-all whitespace-pre-wrap" style={{ color: line.textColor, wordBreak: 'break-word' }}>
                  {line.text}
                </span>
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
  colorIdx: number;
  runBarLeft: number;
  runBarWidth: number;
  selected: boolean;
  onSelect: () => void;
}

function SubagentBar({ segment, colorIdx, runBarLeft, runBarWidth, selected, onSelect }: SubagentBarProps) {
  const barLeft = runBarLeft + segment.startFrac * runBarWidth;
  const barWidth = Math.max((segment.endFrac - segment.startFrac) * runBarWidth, 0.005);
  const colors = SUBAGENT_COLORS[colorIdx % SUBAGENT_COLORS.length];

  const tokApprox = segment.logSlice.length > 0
    ? `${Math.round(segment.logSlice.length * 80 / 1000)}k tok`
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
      {/* ↗ icon */}
      <span className="text-[12px] flex-shrink-0" style={{ color: colors.text }}>↗</span>

      {/* Subagent name — monospace truncated */}
      <span
        className="w-[80px] shrink-0 truncate font-mono text-[11px]"
        style={{ color: colors.text }}
        title={segment.name}
      >
        {segment.name.slice(0, 8)}
      </span>

      {/* Track */}
      <div
        className="relative flex-1 overflow-hidden rounded"
        style={{ height: 16, background: 'var(--bg-soft)' }}
      >
        <div
          className="absolute top-0 h-full rounded"
          style={{
            left: `${String(barLeft * 100)}%`,
            width: `${String(barWidth * 100)}%`,
            background: colors.bar,
          }}
        />
        {/* Tool-call ticks */}
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

      {/* Token approx */}
      <span className="w-[56px] shrink-0 text-right text-[10px]" style={{ color: 'var(--text-secondary)' }}>
        {tokApprox}
      </span>
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
  runNumber: number;
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
      {/* Run row */}
      <div
        className="flex cursor-pointer items-center gap-2 rounded transition-colors hover:bg-[var(--bg-soft)]"
        style={{ minHeight: 44, padding: '8px 0' }}
        onClick={onToggleExpand}
      >
        {/* Chevron */}
        <span className="w-4 shrink-0 text-center text-[12px]" style={{ color: subagents.length > 0 ? 'var(--text-secondary)' : 'transparent' }}>
          {expanded ? '▼' : '▶'}
        </span>

        {/* Run number + time — 120px mono */}
        <span
          className="w-[120px] shrink-0 font-mono text-[11px] leading-tight"
          title={session.startedAt}
        >
          <span style={{ color: 'var(--accent-strong)', fontWeight: 600 }}>#{runNumber}</span>
          <span style={{ color: 'var(--muted)' }}> · {timeLabel}</span>
        </span>

        {/* Track */}
        <div
          className="relative flex-1 overflow-hidden rounded"
          style={{ height: 24, background: 'var(--bg-soft)' }}
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
          {/* Subagent boundary ticks */}
          {subagents.map((sa, si) => (
            <div
              key={si}
              className="absolute top-0 z-10 h-full w-0.5"
              style={{ left: `${String((barLeft + sa.startFrac * barWidth) * 100)}%`, background: 'rgba(255,255,255,0.4)' }}
            />
          ))}
        </div>

        {/* Duration */}
        <span className="w-[60px] shrink-0 text-right font-mono text-[11px]" style={{ color: 'var(--text-secondary)' }}>
          {fmtMs(session.elapsedMs)}
        </span>

        {/* Status badge — only for completed runs */}
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

      {/* Subagent drill-down */}
      {expanded && (
        <div className="space-y-0.5 pb-1">
          {/* Main agent row — always present, shows full run logs */}
          <div
            className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 transition-colors"
            style={{
              paddingLeft: 24,
              background: selectedSubagentIdx === null ? 'var(--accent-soft)' : 'transparent',
            }}
            onClick={() => { onSelectSubagent(null); }}
          >
            <span className="text-[12px] flex-shrink-0" style={{ color: 'var(--muted)' }}>◈</span>
            <span
              className="w-[80px] shrink-0 truncate font-mono text-[11px]"
              style={{ color: selectedSubagentIdx === null ? 'var(--accent-strong)' : 'var(--text-secondary)' }}
            >
              Main
            </span>
            <div
              className="relative flex-1 overflow-hidden rounded"
              style={{ height: 16, background: 'var(--bg-soft)' }}
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
    <div
      className="relative h-6 border-b"
      style={{ marginLeft: 140, marginRight: 80, borderColor: 'var(--line)' }}
    >
      {ticks.map((t) => {
        const pct = ((t - viewStart) / span) * 100;
        if (pct < 0 || pct > 100) return null;
        const label = new Date(t).toLocaleTimeString([], {
          hour: '2-digit',
          minute: '2-digit',
          hour12: false,
        });
        return (
          <span
            key={t}
            className="absolute font-mono text-[10px]"
            style={{ left: `${String(pct)}%`, transform: 'translateX(-50%)', color: 'var(--muted)' }}
          >
            {label}
          </span>
        );
      })}
      <NowMarker viewStart={viewStart} viewEnd={viewEnd} />
    </div>
  );
}

function NowMarker({ viewStart, viewEnd }: { viewStart: number; viewEnd: number }) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const id = setInterval(() => { setNow(Date.now()); }, 1000);
    return () => { clearInterval(id); };
  }, []);
  const pct = ((now - viewStart) / (viewEnd - viewStart)) * 100;
  if (pct < 0 || pct > 100) return null;
  return (
    <div
      className="pointer-events-none absolute top-0 bottom-0 w-px"
      style={{ left: `${String(pct)}%`, background: 'var(--danger)' }}
    />
  );
}

// ─── Stable fallbacks ─────────────────────────────────────────────────────────
const EMPTY_RUNNING: RunningRow[] = [];
const EMPTY_HISTORY: HistoryRow[] = [];

// ─── RunRowWithSubagents ──────────────────────────────────────────────────────
// Wrapper that computes per-run subagents via useMemo (hooks require a component
// boundary — they cannot be called inside a .map() callback).

interface RunRowWithSubagentsProps {
  run: NormalisedSession;
  logs: IssueLogEntry[];
  viewStart: number;
  viewEnd: number;
  expanded: boolean;
  selectedSubagentIdx: number | null;
  runNumber: number;
  onToggleExpand: () => void;
  onSelectSubagent: (idx: number | null) => void;
}

function RunRowWithSubagents({
  run,
  logs,
  viewStart,
  viewEnd,
  expanded,
  selectedSubagentIdx,
  runNumber,
  onToggleExpand,
  onSelectSubagent,
}: RunRowWithSubagentsProps) {
  const runSubagents = useMemo(
    () => extractSubagents(logs, run.sessionId),
    [logs, run.sessionId],
  );
  return (
    <RunRow
      session={run}
      subagents={runSubagents}
      viewStart={viewStart}
      viewEnd={viewEnd}
      expanded={expanded}
      selectedSubagentIdx={expanded ? selectedSubagentIdx : null}
      runNumber={runNumber}
      onToggleExpand={onToggleExpand}
      onSelectSubagent={onSelectSubagent}
    />
  );
}

// ─── IssueRunsView ────────────────────────────────────────────────────────────

interface IssueRunsViewProps {
  group: IssueGroup;
  logs: IssueLogEntry[];
  viewStart: number;
  viewEnd: number;
  expandedRunAt: string | null;
  selectedSubagentIdx: number | null;
  onToggleExpand: (runStartedAt: string) => void;
  onSelectSubagent: (idx: number | null) => void;
}

function IssueRunsView({
  group,
  logs,
  viewStart,
  viewEnd,
  expandedRunAt,
  selectedSubagentIdx,
  onToggleExpand,
  onSelectSubagent,
}: IssueRunsViewProps) {
  return (
    <div className="mt-2 space-y-0.5">
      {group.runs.map((run, idx) => (
        <RunRowWithSubagents
          key={`${run.identifier}-${run.startedAt}`}
          run={run}
          logs={logs}
          viewStart={viewStart}
          viewEnd={viewEnd}
          expanded={expandedRunAt === run.startedAt}
          selectedSubagentIdx={selectedSubagentIdx}
          runNumber={idx + 1}
          onToggleExpand={() => { onToggleExpand(run.startedAt); }}
          onSelectSubagent={onSelectSubagent}
        />
      ))}
    </div>
  );
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function Timeline() {
  const rawRunning = useSymphonyStore((s) => s.snapshot?.running ?? EMPTY_RUNNING);
  const rawHistory = useSymphonyStore((s) => s.snapshot?.history ?? EMPTY_HISTORY);
  const currentAppSessionId = useSymphonyStore((s) => s.snapshot?.currentAppSessionId);
  const liveRunning = useStableValue(rawRunning, 5000);

  const liveSessions = useMemo(() => liveRunning.map(fromRunning), [liveRunning]);
  const historySessions = useMemo(() => rawHistory.map(fromHistory), [rawHistory]);

  const allSessions = useMemo<NormalisedSession[]>(() => {
    // Include ALL historical runs plus current live runs. Historical and live
    // entries for the same issue are distinct records (different startedAt) and
    // should both appear — hiding history for live issues caused previous
    // cancelled runs to vanish after a discard+re-dispatch cycle.
    return [...historySessions, ...liveSessions].sort(
      (a, b) => new Date(a.startedAt).getTime() - new Date(b.startedAt).getTime(),
    );
  }, [historySessions, liveSessions]);

  const issueGroups = useMemo<IssueGroup[]>(() => {
    const map = new Map<string, NormalisedSession[]>();
    for (const s of allSessions) {
      map.set(s.identifier, [...(map.get(s.identifier) ?? []), s]);
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
        if (a.latestStatus === 'live' && b.latestStatus !== 'live') return -1;
        if (b.latestStatus === 'live' && a.latestStatus !== 'live') return 1;
        return new Date(b.latestStartedAt).getTime() - new Date(a.latestStartedAt).getTime();
      });
  }, [allSessions]);

  // ── Clear log mutations ───────────────────────────────────────────────────
  const clearIssueLogs = useClearIssueLogs();
  const clearIssueSubLogs = useClearIssueSubLogs();
  const [confirmClearId, setConfirmClearId] = useState<string | null>(null);

  // ── Selection ─────────────────────────────────────────────────────────────
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [expandedRunAt, setExpandedRunAt] = useState<string | null>(null);
  const [selectedSubagentIdx, setSelectedSubagentIdx] = useState<number | null>(null);
  // Track which selectedId the current expandedRunAt was computed for, so we
  // can reset synchronously (during render) when the user switches issues.
  // This avoids the stale-state flash that effects (async, post-render) cause.
  const [expandedForId, setExpandedForId] = useState<string | null>(null);

  // Auto-select first issue when current selection becomes invalid.
  useEffect(() => {
    if (issueGroups.length === 0) return;
    if (!selectedId || !issueGroups.find((g) => g.identifier === selectedId)) {
      setSelectedId(issueGroups[0].identifier);
    }
  }, [issueGroups, selectedId]);

  // Synchronous reset: when selectedId changes, immediately auto-expand
  // the latest run and clear subagent selection — no async effect gap.
  if (selectedId !== expandedForId) {
    setExpandedForId(selectedId);
    setSelectedSubagentIdx(null);
    const group = issueGroups.find((g) => g.identifier === selectedId);
    if (group && group.runs.length > 0) {
      const latest = [...group.runs].sort(
        (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
      )[0];
      setExpandedRunAt(latest.startedAt);
    } else {
      setExpandedRunAt(null);
    }
  }

  const selectedGroup = useMemo(
    () => issueGroups.find((g) => g.identifier === selectedId) ?? null,
    [issueGroups, selectedId],
  );

  const isSelectedLive = selectedGroup?.runs.some((r) => r.status === 'live') ?? false;
  // In-memory log buffer: real-time for live sessions
  const { data: liveLogsForSelected = [] } = useIssueLogs(selectedId ?? '', isSelectedLive);
  // Full session logs from CLAUDE_CODE_LOG_DIR (covers all subagents)
  const { data: sublogsForSelected = [] } = useSubagentLogs(selectedId ?? '', isSelectedLive);
  // ── Viewport ──────────────────────────────────────────────────────────────
  const [viewport, setViewport] = useState<{ start: number; end: number } | null>(null);
  const [viewportForId, setViewportForId] = useState<string | null>(selectedId);

  if (selectedId !== viewportForId) {
    setViewportForId(selectedId);
    setViewport(null);
  }

  const [liveTick, setLiveTick] = useState(0);

  const { wantStart, wantEnd } = useMemo(() => {
    const now = Date.now();
    const runs = selectedGroup?.runs ?? [];
    const times = runs.map((r) => new Date(r.startedAt).getTime());
    const earliest = times.length > 0 ? Math.min(...times) : now - 10 * 60_000;
    const ws = earliest - 2 * 60_000;
    const hasLive = runs.some((r) => r.status === 'live');
    const we = hasLive
      ? now + 10 * 60_000
      : runs.length > 0
        ? Math.max(
            ...runs.map((r) =>
              r.finishedAt
                ? new Date(r.finishedAt).getTime()
                : new Date(r.startedAt).getTime() + r.elapsedMs,
            ),
          ) + 2 * 60_000
        : now + 10 * 60_000;
    return { wantStart: ws, wantEnd: we };
  // liveTick forces recalculation every 10s when sessions are live
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedGroup, liveTick]);

  useEffect(() => {
    setViewport((prev) =>
      prev
        ? { start: Math.min(prev.start, wantStart), end: Math.max(prev.end, wantEnd) }
        : { start: wantStart, end: wantEnd },
    );
  }, [wantStart, wantEnd]);

  // ── Viewport zoom: when a run is expanded, zoom in to just that run ────────
  const zoomedViewport = useMemo(() => {
    if (!expandedRunAt || !selectedGroup) return null;
    const run = selectedGroup.runs.find((r) => r.startedAt === expandedRunAt);
    if (!run) return null;
    const runStart = new Date(run.startedAt).getTime();
    const runEnd = run.finishedAt
      ? new Date(run.finishedAt).getTime()
      : runStart + Math.max(run.elapsedMs, 1000);
    const span = runEnd - runStart;
    const pad = Math.max(span * 0.12, 15_000);
    return { start: runStart - pad, end: runEnd + pad };
  }, [expandedRunAt, selectedGroup]);

  const viewStart = (zoomedViewport ?? viewport ?? { start: wantStart, end: wantEnd }).start;
  const viewEnd = (zoomedViewport ?? viewport ?? { start: wantStart, end: wantEnd }).end;

  // Subagent bar extraction uses the in-memory log buffer because it contains
  // 'subagent' event markers emitted by the orchestrator. Sublogs (CLAUDE_CODE_LOG_DIR)
  // contain full session text/action entries but no explicit subagent boundary events.
  // Filtered by the expanded run's sessionId so each run shows only its own subagents.
  const expandedRun = selectedGroup?.runs.find((r) => r.startedAt === expandedRunAt) ?? null;
  const expandedSessionId = expandedRun?.sessionId;
  const subagentsForExpanded = useMemo(
    () => extractSubagents(liveLogsForSelected, expandedSessionId),
    [liveLogsForSelected, expandedSessionId],
  );

  // Stable primitives for the expanded run — used as useMemo deps.
  const expandedRunFinishedAt = expandedRun?.finishedAt;

  // Per-run filter using BOTH sessionId AND timestamp.
  //
  // Entries WITH a sessionId: include only if it matches the run's sessionId.
  // Entries WITHOUT a sessionId (orchestrator messages: "hook:", "worker:", "$"):
  //   include only if their timestamp falls within the run's time window.
  //   This prevents logs from earlier runs leaking through even when they have no sessionId.
  const filterByRun = (logs: IssueLogEntry[], run: NormalisedSession | null): IssueLogEntry[] => {
    if (!run) return logs;
    const sid = run.sessionId;
    // When the run has a sessionId, use it exclusively — it's the most reliable filter.
    // Only fall back to timestamp when sessionId is absent (pre-session orchestrator messages).
    if (sid) {
      return logs.filter((e) => {
        // Entries with a sessionId: must match exactly.
        if (e.sessionId) return e.sessionId === sid;
        // Entries without sessionId (orchestrator messages): filter by timestamp.
        if (!e.time) return false; // can't place, exclude rather than leak
        const t = new Date(e.time).getTime();
        if (isNaN(t)) return false;
        const startMs = new Date(run.startedAt).getTime() - 5_000;
        const endMs = run.finishedAt
          ? new Date(run.finishedAt).getTime() + 5_000
          : Date.now() + 60_000;
        return t >= startMs && t <= endMs;
      });
    }
    // No sessionId on run (very early or old format): timestamp-only filter.
    const startMs = new Date(run.startedAt).getTime() - 5_000;
    const endMs = run.finishedAt
      ? new Date(run.finishedAt).getTime() + 5_000
      : Date.now() + 60_000;
    return logs.filter((e) => {
      if (!e.time) return false;
      const t = new Date(e.time).getTime();
      if (isNaN(t)) return false;
      return t >= startMs && t <= endMs;
    });
  };

  // Filter sublogs to the expanded run.
  // Returns empty when no run is expanded — prevents leaking entries from all runs.
  // Falls back to all sublogs when no entries carry sessionId (old-format logs).
  const sublogsForExpanded = useMemo(() => {
    if (!expandedRun) return [];
    if (sublogsForSelected.length === 0) return [];
    const hasSessionIds = sublogsForSelected.some((e) => e.sessionId);
    if (!hasSessionIds) return sublogsForSelected;
    return filterByRun(sublogsForSelected, expandedRun);
    // filterByRun captures expandedRun by closure; stable primitive deps listed below
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sublogsForSelected, expandedSessionId, expandedRunAt, expandedRunFinishedAt]);

  // Show subagent slice when one is selected; otherwise prefer sublogs (full session),
  // then fall back to the in-memory buffer filtered to this run.
  // When no run is expanded, logPanelSlice is irrelevant (placeholder shown instead).
  //
  // Guard: only use sublogs when expandedSessionId is known. Before the first
  // EventWorkerUpdate arrives, sessionId is undefined, so filterByRun drops every
  // entry that carries a sessionId (all agent output). The result is a non-empty
  // but near-empty array of orchestrator-only messages, and the SSE fallback never
  // fires. When sessionId is not yet set, fall through to the SSE buffer instead.
  // Guard: if selectedSubagentIdx is out of bounds (stale from a previous run),
  // treat it as null so we fall through to the full run log view.
  const validSubagentIdx =
    selectedSubagentIdx !== null &&
    selectedSubagentIdx >= 0 &&
    selectedSubagentIdx < subagentsForExpanded.length
      ? selectedSubagentIdx
      : null;
  const logPanelSlice = !expandedRun
    ? []
    : validSubagentIdx !== null
      ? subagentsForExpanded[validSubagentIdx].logSlice
      : sublogsForExpanded.length > 0 && expandedSessionId
        ? sublogsForExpanded
        : filterByRun(liveLogsForSelected, expandedRun);
  const selectedSession = selectedGroup?.runs.find((r) => r.startedAt === expandedRunAt) ?? null;
  const anyLive = allSessions.some((s) => s.status === 'live');

  useEffect(() => {
    if (!anyLive) return;
    const id = setInterval(() => { setLiveTick((n) => n + 1); }, 10_000);
    return () => { clearInterval(id); };
  }, [anyLive]);

  // ── Status dot style ──────────────────────────────────────────────────────
  function dotStyle(status: NormalisedSession['status']): React.CSSProperties {
    switch (status) {
      case 'live':
        return {
          background: 'var(--success)',
          boxShadow: '0 0 0 4px rgba(34,197,94,0.2)',
        };
      case 'succeeded':
        return { background: 'var(--accent)' };
      case 'failed':
        return { background: 'var(--danger)' };
      default:
        return { background: 'var(--muted)' };
    }
  }

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <>
      <PageMeta title="Symphony | Timeline" description="Agent timeline" />

      <div className="flex" style={{ height: 'calc(100vh - 100px)', minHeight: 500 }}>
        {/* ── Sidebar: issue chips ─────────────────────────────────────────── */}
        <aside
          className="flex flex-shrink-0 flex-col border-r"
          style={{ width: 180, borderColor: 'var(--line)', background: 'var(--panel)' }}
        >
          {/* Sidebar header */}
          <div
            className="border-b px-3 py-3"
            style={{ borderColor: 'var(--line)' }}
          >
            <p className="text-[10px] font-bold tracking-[0.08em] uppercase" style={{ color: 'var(--muted)' }}>
              Issues
            </p>
          </div>

          {/* Issue chips */}
          <div className="flex-1 overflow-y-auto p-2" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {issueGroups.length === 0 ? (
              <p className="px-2 py-4 text-xs italic" style={{ color: 'var(--muted)' }}>
                No sessions yet
              </p>
            ) : (
              issueGroups.map((group) => {
                const isSelected = selectedId === group.identifier;
                const isLive = group.latestStatus === 'live';
                const statusText = isLive
                  ? `${String(group.runs.length)} run${group.runs.length !== 1 ? 's' : ''} · live`
                  : group.latestStatus === 'failed'
                    ? 'failed + blocked'
                    : group.latestStatus === 'succeeded'
                      ? 'completed'
                      : `${String(group.runs.length)} run${group.runs.length !== 1 ? 's' : ''}`;
                return (
                  <div key={group.identifier} className="group relative">
                    <button
                      onClick={() => { setSelectedId(group.identifier); }}
                      className="w-full text-left transition-all"
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 10,
                        padding: '11px 12px',
                        borderRadius: 'var(--radius-md)',
                        border: `1px solid ${isSelected ? 'var(--accent)' : 'var(--line)'}`,
                        background: isSelected ? 'var(--accent-soft)' : 'var(--bg-elevated)',
                        cursor: 'pointer',
                        font: 'inherit',
                      }}
                    >
                      {/* Status dot */}
                      <span
                        className={`flex-shrink-0 rounded-full ${isLive ? 'animate-pulse' : ''}`}
                        style={{ width: 8, height: 8, ...dotStyle(group.latestStatus) }}
                      />
                      <div className="min-w-0 pr-4">
                        <div
                          className="font-mono font-semibold truncate"
                          style={{ fontSize: 14, color: 'var(--text)' }}
                        >
                          {group.identifier}
                        </div>
                        <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 2 }}>
                          {statusText}
                        </div>
                      </div>
                    </button>
                    {/* Trash icon — visible on hover, only for non-live issues */}
                    {!isLive && (
                      <button
                        onClick={(e) => { e.stopPropagation(); setConfirmClearId(group.identifier); }}
                        title="Clear all session logs"
                        className="absolute right-2 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 transition-opacity"
                        style={{
                          background: 'transparent',
                          border: 'none',
                          cursor: 'pointer',
                          padding: 4,
                          color: 'var(--muted)',
                          lineHeight: 1,
                        }}
                      >
                        <svg width="13" height="13" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                          <path d="M2 4h12M5 4V2h6v2M6 7v5M10 7v5M3 4l1 10h8l1-10H3z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                        </svg>
                      </button>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </aside>

        {/* ── Main: track area + log area ──────────────────────────────────── */}
        <div className="flex flex-1 flex-col overflow-hidden">
          {/* Header */}
          <div
            className="flex flex-shrink-0 items-center justify-between border-b px-4 py-3"
            style={{ borderColor: 'var(--line)' }}
          >
            <div>
              <h2 className="text-base font-semibold" style={{ color: 'var(--text)' }}>
                {selectedId ? `Timeline: ${selectedId}` : 'Timeline'}
              </h2>
              <p className="mt-1 text-[12px]" style={{ color: 'var(--text-secondary)' }}>
                {anyLive
                  ? 'Running · click a run bar to expand subagents'
                  : 'Completed sessions · click a run to drill down'}
              </p>
              {currentAppSessionId && (
                <p className="mt-0.5 font-mono text-[10px]" style={{ color: 'var(--muted)' }}>
                  Session {currentAppSessionId.slice(0, 8)}
                </p>
              )}
            </div>
            {selectedGroup && (
              <span
                className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[12px] font-semibold"
                style={{ background: 'var(--accent-soft)', color: 'var(--accent-strong)' }}
              >
                {selectedGroup.runs.some((r) => r.status === 'live') && (
                  <span
                    className="inline-block animate-pulse rounded-full"
                    style={{ width: 6, height: 6, background: 'var(--success)' }}
                  />
                )}
                {selectedGroup.runs.length} run{selectedGroup.runs.length !== 1 ? 's' : ''}
              </span>
            )}
          </div>

          {/* Track area */}
          <div
            className="flex-1 overflow-y-auto border-b"
            style={{ padding: 16, borderColor: 'var(--line)' }}
          >
            {!selectedGroup ? (
              <div className="flex h-12 items-center justify-center text-sm" style={{ color: 'var(--muted)' }}>
                Select an issue from the sidebar
              </div>
            ) : (
              <>
                <TimeAxis viewStart={viewStart} viewEnd={viewEnd} />
                <IssueRunsView
                  group={selectedGroup}
                  logs={liveLogsForSelected}
                  viewStart={viewStart}
                  viewEnd={viewEnd}
                  expandedRunAt={expandedRunAt}
                  selectedSubagentIdx={selectedSubagentIdx}
                  onToggleExpand={(runStartedAt) => {
                    setExpandedRunAt((prev) => {
                      // Always clear subagent selection when switching or toggling runs
                      setSelectedSubagentIdx(null);
                      return prev === runStartedAt ? null : runStartedAt;
                    });
                  }}
                  onSelectSubagent={setSelectedSubagentIdx}
                />
              </>
            )}
          </div>

          {/* Log area */}
          <div className="flex flex-1 flex-col overflow-hidden">
            {selectedId ? (
              <>
                {/* Log header */}
                <div
                  className="flex flex-shrink-0 items-center justify-between border-b px-3 py-2"
                  style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)' }}
                >
                  <div className="flex items-center gap-2">
                    <span
                      className="text-[11px] font-bold uppercase tracking-[0.08em]"
                      style={{ color: 'var(--muted)' }}
                    >
                      Logs — {selectedId}
                    </span>
                    {!isSelectedLive && sublogsForSelected.length > 0 && (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-medium"
                        style={{ background: 'var(--bg-soft)', color: 'var(--text-secondary)' }}
                        title="Full session logs from CLAUDE_CODE_LOG_DIR (includes all subagents)"
                      >
                        session logs · {sublogsForSelected.length} events
                      </span>
                    )}
                    {validSubagentIdx !== null && subagentsForExpanded[validSubagentIdx] && (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-medium"
                        style={{ background: 'var(--accent-soft)', color: 'var(--accent-strong)' }}
                      >
                        {subagentsForExpanded[validSubagentIdx].name.slice(0, 8)}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {validSubagentIdx !== null && (
                      <button
                        onClick={() => { setSelectedSubagentIdx(null); }}
                        className="text-[10px] hover:opacity-70 transition-opacity"
                        style={{
                          color: 'var(--muted)',
                          background: 'transparent',
                          border: '1px solid var(--line)',
                          padding: '4px 8px',
                          borderRadius: 4,
                          cursor: 'pointer',
                        }}
                      >
                        show all logs
                      </button>
                    )}
                    {selectedSession?.status === 'live' ? (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-medium"
                        style={{ background: 'var(--success-soft)', color: 'var(--success)' }}
                      >
                        live
                      </span>
                    ) : selectedSession && (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-medium"
                        style={
                          selectedSession.status === 'succeeded'
                            ? { background: 'var(--success-soft)', color: 'var(--success)' }
                            : { background: 'var(--danger-soft)', color: 'var(--danger)' }
                        }
                      >
                        {selectedSession.status}
                      </span>
                    )}
                  </div>
                </div>

                {/* Log content */}
                <div className="flex-1 overflow-hidden">
                  {expandedRunAt == null ? (
                    <div
                      className="flex h-full items-center justify-center font-mono text-[12px] italic"
                      style={{ color: 'var(--muted)', background: 'var(--panel-dark)' }}
                    >
                      Select a run to show logs per run
                    </div>
                  ) : (
                    <AgentLogPanel key={expandedRunAt ?? ''} identifier={selectedId} logSlice={logPanelSlice} />
                  )}
                </div>
              </>
            ) : (
              <div className="flex flex-1 items-center justify-center text-sm" style={{ color: 'var(--muted)' }}>
                Select an issue from the sidebar to view logs
              </div>
            )}
          </div>
        </div>
      </div>

      {/* ── Confirm clear session logs modal ───────────────────────────── */}
      <Modal
        isOpen={!!confirmClearId}
        onClose={() => { setConfirmClearId(null); }}
        showCloseButton={false}
        className="max-w-sm p-6"
      >
        <p className="text-sm font-semibold" style={{ color: 'var(--text)' }}>
          Clear session logs for <span className="font-mono">{confirmClearId}</span>?
        </p>
        <p className="mt-1 text-xs" style={{ color: 'var(--muted)' }}>
          All JSONL session files for this issue will be permanently deleted.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <button
            onClick={() => { setConfirmClearId(null); }}
            style={{
              padding: '6px 14px',
              borderRadius: 4,
              fontSize: 12,
              cursor: 'pointer',
              background: 'transparent',
              color: 'var(--text-secondary)',
              border: '1px solid var(--line)',
            }}
          >
            Cancel
          </button>
          <button
            disabled={clearIssueLogs.isPending || clearIssueSubLogs.isPending}
            onClick={() => {
              if (!confirmClearId) return;
              const id = confirmClearId;
              clearIssueLogs.mutate(id);
              clearIssueSubLogs.mutate(id, { onSettled: () => { setConfirmClearId(null); } });
            }}
            style={{
              padding: '6px 14px',
              borderRadius: 4,
              fontSize: 12,
              fontWeight: 600,
              cursor: clearIssueLogs.isPending || clearIssueSubLogs.isPending ? 'wait' : 'pointer',
              background: 'var(--danger)',
              color: '#fff',
              border: 'none',
            }}
          >
            {clearIssueLogs.isPending || clearIssueSubLogs.isPending ? 'Clearing…' : 'Clear all'}
          </button>
        </div>
      </Modal>
    </>
  );
}
