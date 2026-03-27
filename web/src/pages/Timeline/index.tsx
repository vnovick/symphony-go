import { useEffect, useMemo, useState } from 'react';

import PageMeta from '../../components/common/PageMeta';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useIssueLogs, useSubagentLogs } from '../../queries/logs';
import { useStableValue } from '../../hooks/useStableValue';
import { useClearIssueLogs, useClearIssueSubLogs } from '../../queries/issues';
import { Modal } from '../../components/ui/modal';

import { AgentLogPanel } from '../../components/symphony/timeline/AgentLogPanel';
import { TimeAxis } from '../../components/symphony/timeline/TimeAxis';
import { IssueRunsView } from '../../components/symphony/timeline/IssueRunsView';
import {
  EMPTY_RUNNING,
  EMPTY_HISTORY,
  fromRunning,
  fromHistory,
  extractSubagents,
  filterByRun,
  dotStyle,
} from '../../components/symphony/timeline/types';
import type { NormalisedSession } from '../../components/symphony/timeline/types';

// ─── Main page ────────────────────────────────────────────────────────────────

export default function Timeline() {
  const rawRunning = useSymphonyStore((s) => s.snapshot?.running ?? EMPTY_RUNNING);
  const rawHistory = useSymphonyStore((s) => s.snapshot?.history ?? EMPTY_HISTORY);
  const currentAppSessionId = useSymphonyStore((s) => s.snapshot?.currentAppSessionId);
  const liveRunning = useStableValue(rawRunning, 5000);

  const liveSessions = useMemo(() => liveRunning.map(fromRunning), [liveRunning]);
  const historySessions = useMemo(() => rawHistory.map(fromHistory), [rawHistory]);

  const allSessions = useMemo<NormalisedSession[]>(() => {
    return [...historySessions, ...liveSessions].sort(
      (a, b) => new Date(a.startedAt).getTime() - new Date(b.startedAt).getTime(),
    );
  }, [historySessions, liveSessions]);

  const issueGroups = useMemo(() => {
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
  const selectedId = useSymphonyStore((s) => s.activeIssueId);
  const setSelectedId = useSymphonyStore((s) => s.setActiveIssueId);
  const [expandedRunAt, setExpandedRunAt] = useState<string | null>(null);
  const [selectedSubagentIdx, setSelectedSubagentIdx] = useState<number | null>(null);
  const [expandedForId, setExpandedForId] = useState<string | null>(null);

  // Auto-select first issue when current selection is invalid.
  // Use a fingerprint of issue identifiers (not the full issueGroups reference)
  // to avoid firing this effect on every SSE push.
  const groupIds = useMemo(
    () => issueGroups.map((g) => g.identifier).join('\0'),
    [issueGroups],
  );
  const firstGroupId = issueGroups[0]?.identifier ?? null;
  useEffect(() => {
    if (!firstGroupId) return;
    if (!selectedId || !groupIds.includes(selectedId)) {
      setSelectedId(firstGroupId);
    }
  }, [groupIds, firstGroupId, selectedId, setSelectedId]);

  // Synchronous reset: when selectedId changes, immediately auto-expand
  // the latest run and clear subagent selection.
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
  const { data: logsForSelected = [] } = useIssueLogs(selectedId ?? '', isSelectedLive);
  const { data: sublogs = [] } = useSubagentLogs(selectedId ?? '', isSelectedLive);

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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedGroup, liveTick]);

  useEffect(() => {
    setViewport((prev) =>
      prev
        ? { start: Math.min(prev.start, wantStart), end: Math.max(prev.end, wantEnd) }
        : { start: wantStart, end: wantEnd },
    );
  }, [wantStart, wantEnd]);

  // ── Viewport zoom ─────────────────────────────────────────────────────────
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

  // ── Log panel data ────────────────────────────────────────────────────────
  const expandedRun = selectedGroup?.runs.find((r) => r.startedAt === expandedRunAt) ?? null;
  const expandedSessionId = expandedRun?.sessionId;
  const subagentsForExpanded = useMemo(
    () => extractSubagents(logsForSelected, expandedSessionId),
    [logsForSelected, expandedSessionId],
  );

  const expandedRunFinishedAt = expandedRun?.finishedAt;

  const sublogsForExpanded = useMemo(() => {
    if (!expandedRun) return [];
    if (sublogs.length === 0) return [];
    const hasSessionIds = sublogs.some((e) => e.sessionId);
    if (!hasSessionIds) return sublogs;
    return filterByRun(sublogs, expandedRun);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sublogs, expandedSessionId, expandedRunAt, expandedRunFinishedAt]);

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
        : filterByRun(logsForSelected, expandedRun);
  const selectedSession = selectedGroup?.runs.find((r) => r.startedAt === expandedRunAt) ?? null;
  const anyLive = allSessions.some((s) => s.status === 'live');

  useEffect(() => {
    if (!anyLive) return;
    const id = setInterval(() => { setLiveTick((n) => n + 1); }, 10_000);
    return () => { clearInterval(id); };
  }, [anyLive]);

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <>
      <PageMeta title="Symphony | Timeline" description="Agent timeline" />

      <div className="flex" style={{ height: 'calc(100vh - 100px)', minHeight: 500 }}>
        {/* ── Sidebar: issue chips ─────────────────────────────────────────── */}
        <aside
          className="flex flex-shrink-0 flex-col border-r border-theme-line bg-theme-panel"
          style={{ width: 180 }}
        >
          <div
            className="border-b px-3 py-3 border-theme-line"
          >
            <p className="text-[10px] font-bold tracking-[0.08em] uppercase text-theme-muted">
              Issues
            </p>
          </div>

          <div className="flex-1 overflow-y-auto p-2" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {issueGroups.length === 0 ? (
              <p className="px-2 py-4 text-xs italic text-theme-muted">
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
                      <span
                        className={`flex-shrink-0 rounded-full ${isLive ? 'animate-pulse' : ''}`}
                        style={{ width: 8, height: 8, ...dotStyle(group.latestStatus) }}
                      />
                      <div className="min-w-0 pr-4">
                        <div
                          className="font-mono font-semibold truncate text-theme-text"
                          style={{ fontSize: 14 }}
                        >
                          {group.identifier}
                        </div>
                        <div className="text-theme-text-secondary" style={{ fontSize: 12, marginTop: 2 }}>
                          {statusText}
                        </div>
                      </div>
                    </button>
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
          <div
            className="flex flex-shrink-0 items-center justify-between border-b px-4 py-3 border-theme-line"
          >
            <div>
              <h2 className="text-base font-semibold text-theme-text">
                {selectedId ? `Timeline: ${selectedId}` : 'Timeline'}
              </h2>
              <p className="mt-1 text-[12px] text-theme-text-secondary">
                {anyLive
                  ? 'Running · click a run bar to expand subagents'
                  : 'Completed sessions · click a run to drill down'}
              </p>
              {currentAppSessionId && (
                <p className="mt-0.5 font-mono text-[10px] text-theme-muted">
                  Session {currentAppSessionId.slice(0, 8)}
                </p>
              )}
            </div>
            {selectedGroup && (
              <span
                className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[12px] font-semibold bg-theme-accent-soft text-theme-accent-strong"
              >
                {selectedGroup.runs.some((r) => r.status === 'live') && (
                  <span
                    className="inline-block animate-pulse rounded-full bg-theme-success"
                    style={{ width: 6, height: 6 }}
                  />
                )}
                {selectedGroup.runs.length} run{selectedGroup.runs.length !== 1 ? 's' : ''}
              </span>
            )}
          </div>

          <div
            className="flex-1 overflow-y-auto border-b border-theme-line"
            style={{ padding: 16 }}
          >
            {!selectedGroup ? (
              <div className="flex h-12 items-center justify-center text-sm text-theme-muted">
                Select an issue from the sidebar
              </div>
            ) : (
              <>
                <TimeAxis viewStart={viewStart} viewEnd={viewEnd} />
                <IssueRunsView
                  group={selectedGroup}
                  logs={logsForSelected}
                  viewStart={viewStart}
                  viewEnd={viewEnd}
                  expandedRunAt={expandedRunAt}
                  selectedSubagentIdx={selectedSubagentIdx}
                  onToggleExpand={(runStartedAt) => {
                    setExpandedRunAt((prev) => {
                      setSelectedSubagentIdx(null);
                      return prev === runStartedAt ? null : runStartedAt;
                    });
                  }}
                  onSelectSubagent={setSelectedSubagentIdx}
                />
              </>
            )}
          </div>

          <div className="flex flex-1 flex-col overflow-hidden">
            {selectedId ? (
              <>
                <div
                  className="flex flex-shrink-0 items-center justify-between border-b px-3 py-2 border-theme-line bg-theme-panel-strong"
                >
                  <div className="flex items-center gap-2">
                    <span
                      className="text-[11px] font-bold uppercase tracking-[0.08em]"
                    >
                      Logs — {selectedId}
                    </span>
                    {!isSelectedLive && sublogs.length > 0 && (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-medium bg-theme-bg-soft text-theme-text-secondary"
                        title="Full session logs from CLAUDE_CODE_LOG_DIR (includes all subagents)"
                      >
                        session logs · {sublogs.length} events
                      </span>
                    )}
                    {validSubagentIdx !== null && subagentsForExpanded[validSubagentIdx] && (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-medium bg-theme-accent-soft text-theme-accent-strong"
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
                        className="rounded px-1.5 py-0.5 text-[9px] font-medium bg-theme-success-soft text-theme-success"
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

                <div className="flex-1 overflow-hidden">
                  {expandedRunAt == null ? (
                    <div
                      className="flex h-full items-center justify-center font-mono text-[12px] italic text-theme-muted bg-theme-panel-dark"
                    >
                      Select a run to show logs per run
                    </div>
                  ) : (
                    <AgentLogPanel key={expandedRunAt} identifier={selectedId} logSlice={logPanelSlice} />
                  )}
                </div>
              </>
            ) : (
              <div className="flex flex-1 items-center justify-center text-sm text-theme-muted">
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
        <p className="text-sm font-semibold text-theme-text">
          Clear session logs for <span className="font-mono">{confirmClearId}</span>?
        </p>
        <p className="mt-1 text-xs text-theme-muted">
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
