import { useState, useMemo, useCallback, useEffect } from 'react';
import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
  type DragOverEvent,
} from '@dnd-kit/core';
import PageMeta from '../../components/common/PageMeta';
import RunningSessionsTable from '../../components/symphony/RunningSessionsTable';
import RetryQueueTable from '../../components/symphony/RetryQueueTable';
import { HostPool } from '../../components/symphony/HostPool';
import { ProjectSelector } from '../../components/symphony/ProjectSelector';
import { NarrativeFeed } from '../../components/symphony/NarrativeFeed';
import IssueCard from '../../components/symphony/IssueCard';
import BoardColumn from '../../components/symphony/BoardColumn';
import AgentQueueView from '../../components/symphony/AgentQueueView';
import Badge from '../../components/ui/badge/Badge';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useToastStore } from '../../store/toastStore';
import type { TrackerIssue } from '../../types/schemas';
import {
  useIssues,
  useInvalidateIssues,
  useUpdateIssueState,
  useSetIssueProfile,
  useCancelIssue,
  useResumeIssue,
} from '../../queries/issues';
import { orchDotClass, stateBadgeColor, EMPTY_PROFILE_LABEL } from '../../utils/format';

// ─── Board view ───────────────────────────────────────────────────────────────

interface BoardProps {
  issues: TrackerIssue[];
  onSelect: (id: string) => void;
  onStateChange: (identifier: string, newState: string) => void;
  availableProfiles: string[];
  onProfileChange: (identifier: string, profile: string) => void;
}

function BoardView({
  issues,
  onSelect,
  onStateChange,
  availableProfiles,
  onProfileChange,
}: BoardProps) {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const snapshotLoaded = snapshot !== null;
  const [activeIssue, setActiveIssue] = useState<TrackerIssue | null>(null);
  const [overId, setOverId] = useState<string | null>(null);

  const profileDefs = snapshot?.profileDefs;
  const runningBackendByIdentifier = useMemo(() => {
    const map: Record<string, string> = {};
    for (const r of snapshot?.running ?? []) {
      if (r.backend) map[r.identifier] = r.backend;
    }
    return map;
  }, [snapshot?.running]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor),
  );

  const backlogStateSet = useMemo(
    () => new Set(snapshot?.backlogStates ?? []),
    [snapshot?.backlogStates],
  );
  const firstActiveState = snapshot?.activeStates?.[0] ?? '';

  const handleDispatch = useCallback(
    (identifier: string) => {
      if (firstActiveState) void onStateChange(identifier, firstActiveState);
    },
    [onStateChange, firstActiveState],
  );

  const columnNames = useMemo(() => {
    const backlog = snapshot?.backlogStates ?? [];
    const active = snapshot?.activeStates ?? [];
    const completion = snapshot?.completionState ? [snapshot.completionState] : [];
    const terminal = snapshot?.terminalStates ?? [];
    // Deduplicate while preserving order: backlog → active → completion → terminal
    const seen = new Set<string>();
    const cols: string[] = [];
    for (const s of [...backlog, ...active, ...completion, ...terminal]) {
      if (!seen.has(s)) {
        seen.add(s);
        cols.push(s);
      }
    }
    // If snapshot not loaded yet, fall back to unique states from issues
    if (cols.length === 0) {
      return [...new Set(issues.map((i) => i.state))];
    }
    return cols;
  }, [snapshot, issues]);

  const columns = useMemo<[string, TrackerIssue[]][]>(() => {
    const byState = new Map<string, TrackerIssue[]>();
    for (const name of columnNames) byState.set(name, []);
    for (const issue of issues) {
      if (byState.has(issue.state)) {
        (byState.get(issue.state) as TrackerIssue[]).push(issue);
      } else {
        // state exists in issues but not in config — add a column for it
        byState.set(issue.state, []);
        (byState.get(issue.state) as TrackerIssue[]).push(issue);
      }
    }
    return [...byState.entries()];
  }, [columnNames, issues]);

  const onDragStart = useCallback((e: DragStartEvent) => {
    const data = e.active.data.current as { issue: TrackerIssue } | undefined;
    if (data?.issue) setActiveIssue(data.issue);
  }, []);

  const onDragOver = useCallback((e: DragOverEvent) => {
    setOverId(e.over ? String(e.over.id) : null);
  }, []);

  const onDragEnd = useCallback(
    (e: DragEndEvent) => {
      setActiveIssue(null);
      setOverId(null);
      if (!e.over) return;
      const data = e.active.data.current as { issue: TrackerIssue } | undefined;
      if (!data?.issue) return;
      const newState = String(e.over.id);
      if (newState !== data.issue.state) {
        onStateChange(String(e.active.id), newState);
      }
    },
    [onStateChange],
  );

  if (columns.length === 0) {
    return (
      <div
        className="rounded-[var(--radius-md)] px-6 py-12 text-center text-sm"
        style={{ border: '1px solid var(--line)', background: 'var(--panel)', color: 'var(--muted)' }}
      >
        {!snapshotLoaded ? 'Connecting to Symphony…' : 'No issues match the current filters'}
      </div>
    );
  }

  return (
    <DndContext
      sensors={sensors}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDragEnd={onDragEnd}
    >
      <div className="board-grid pb-4">
        {columns.map(([state, colIssues]) => (
          <BoardColumn
            key={state}
            state={state}
            issues={colIssues}
            isOver={overId === state}
            onSelect={onSelect}
            availableProfiles={availableProfiles}
            profileDefs={profileDefs}
            runningBackendByIdentifier={runningBackendByIdentifier}
            onProfileChange={onProfileChange}
            onDispatch={backlogStateSet.has(state) ? handleDispatch : undefined}
          />
        ))}
      </div>
      <DragOverlay>
        {activeIssue && <IssueCard issue={activeIssue} isDragging onSelect={() => {}} />}
      </DragOverlay>
    </DndContext>
  );
}

// ─── List view ────────────────────────────────────────────────────────────────

type SortKey = 'identifier' | 'title' | 'state';
type SortDir = 'asc' | 'desc';

function SortIcon({ active, dir }: { active: boolean; dir: SortDir }) {
  return (
    <span className="ml-1" style={{ color: active ? 'var(--accent)' : 'var(--muted)' }}>
      {active ? (dir === 'asc' ? '↑' : '↓') : '↕'}
    </span>
  );
}

function ListView({
  issues,
  onSelect,
  availableProfiles,
  onProfileChange,
}: {
  issues: TrackerIssue[];
  onSelect: (id: string) => void;
  availableProfiles: string[];
  onProfileChange: (identifier: string, profile: string) => void;
}) {
  const [sortKey, setSortKey] = useState<SortKey>('identifier');
  const [sortDir, setSortDir] = useState<SortDir>('asc');
  const cancelIssueMutation = useCancelIssue();
  const resumeIssueMutation = useResumeIssue();

  const handleSort = (key: SortKey) => {
    if (sortKey === key) setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    else {
      setSortKey(key);
      setSortDir('asc');
    }
  };

  const getVal = (issue: TrackerIssue, key: SortKey): string => {
    if (key === 'identifier') return issue.identifier;
    if (key === 'title') return issue.title.toLowerCase();
    return issue.state.toLowerCase();
  };

  const sorted = useMemo(
    () =>
      [...issues].sort((a, b) => {
        const cmp = getVal(a, sortKey).localeCompare(getVal(b, sortKey));
        return sortDir === 'asc' ? cmp : -cmp;
      }),
    [issues, sortKey, sortDir],
  );

  const thStyle: React.CSSProperties = { color: 'var(--text-secondary)' };
  const thClass = 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider select-none cursor-pointer';

  return (
    <div
      className="overflow-hidden rounded-[var(--radius-md)]"
      style={{ border: '1px solid var(--line)', background: 'var(--panel)' }}
    >
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead style={{ background: 'var(--bg-soft)' }}>
            <tr>
              <th className={thClass} style={thStyle} onClick={() => { handleSort('identifier'); }}>
                Identifier <SortIcon active={sortKey === 'identifier'} dir={sortDir} />
              </th>
              <th className={thClass} style={thStyle} onClick={() => { handleSort('title'); }}>
                Title <SortIcon active={sortKey === 'title'} dir={sortDir} />
              </th>
              <th className={thClass} style={thStyle} onClick={() => { handleSort('state'); }}>
                State <SortIcon active={sortKey === 'state'} dir={sortDir} />
              </th>
              <th className={thClass} style={thStyle}>Agent</th>
              <th className={thClass} style={thStyle}>Actions</th>
            </tr>
          </thead>
          <tbody style={{ borderTop: '1px solid var(--line)' }}>
            {sorted.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-10 text-center text-sm" style={{ color: 'var(--muted)' }}>
                  No issues match the current filters
                </td>
              </tr>
            )}
            {sorted.map((issue) => (
              <tr
                key={issue.identifier}
                className="cursor-pointer transition-colors hover:bg-[var(--bg-soft)]"
                style={{ borderTop: '1px solid var(--line)' }}
                onClick={() => { onSelect(issue.identifier); }}
              >
                <td className="px-4 py-3 whitespace-nowrap">
                  {issue.url ? (
                    <a
                      href={issue.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono text-sm font-medium hover:underline"
                      style={{ color: 'var(--accent)' }}
                      onClick={(e) => { e.stopPropagation(); }}
                    >
                      {issue.identifier}
                    </a>
                  ) : (
                    <span className="font-mono text-sm font-medium" style={{ color: 'var(--text)' }}>
                      {issue.identifier}
                    </span>
                  )}
                </td>
                <td className="max-w-xs truncate px-4 py-3" style={{ color: 'var(--text-secondary)' }}>
                  {issue.title}
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <Badge size="sm" color={stateBadgeColor(issue.state)}>
                    {issue.state}
                  </Badge>
                </td>
                <td className="px-4 py-3 whitespace-nowrap" onClick={(e) => { e.stopPropagation(); }}>
                  {availableProfiles.length > 0 ? (
                    <select
                      value={issue.agentProfile ?? ''}
                      onChange={(e) => { onProfileChange(issue.identifier, e.target.value); }}
                      className="rounded px-1.5 py-0.5 text-xs focus:outline-none"
                      style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)', color: 'var(--text-secondary)' }}
                    >
                      <option value="">{EMPTY_PROFILE_LABEL}</option>
                      {availableProfiles.map((p) => (
                        <option key={p} value={p}>{p}</option>
                      ))}
                    </select>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-xs" style={{ color: 'var(--muted)' }}>
                      <span className={`h-2 w-2 rounded-full ${orchDotClass(issue.orchestratorState)}`} />
                      {issue.orchestratorState}
                    </span>
                  )}
                </td>
                <td className="px-4 py-3 whitespace-nowrap" onClick={(e) => { e.stopPropagation(); }}>
                  {issue.orchestratorState === 'running' && (
                    <button
                      onClick={() => { cancelIssueMutation.mutate(issue.identifier); }}
                      className="rounded px-2 py-1 text-xs transition-colors"
                      style={{ border: '1px solid var(--danger-soft)', color: 'var(--danger)', background: 'transparent' }}
                    >
                      ⏸ Pause
                    </button>
                  )}
                  {issue.orchestratorState === 'paused' && (
                    <button
                      onClick={() => { resumeIssueMutation.mutate(issue.identifier); }}
                      className="rounded px-2 py-1 text-xs transition-colors"
                      style={{ border: '1px solid var(--success-soft)', color: 'var(--success)', background: 'transparent' }}
                    >
                      ▶ Resume
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ─── Compact hero stats ───────────────────────────────────────────────────────

function StatTile({
  label,
  value,
  sub,
  valueColor,
}: {
  label: string;
  value: string | number;
  sub: string;
  valueColor?: string;
}) {
  return (
    <div
      className="rounded-lg px-3 py-2.5 min-w-[80px]"
      style={{ background: 'rgba(0,0,0,0.18)', border: '1px solid var(--line)' }}
    >
      <div
        className="text-[10px] font-semibold uppercase tracking-[0.06em] mb-1"
        style={{ color: 'var(--muted)' }}
      >
        {label}
      </div>
      <div
        className="font-mono text-[20px] font-bold leading-none"
        style={{ color: valueColor ?? 'var(--text)' }}
      >
        {value}
      </div>
      <div className="text-[11px] mt-1" style={{ color: 'var(--text-secondary)' }}>{sub}</div>
    </div>
  );
}

function HeroStats() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const running  = snapshot?.counts.running   ?? 0;
  const retrying = snapshot?.counts.retrying  ?? 0;
  const paused   = snapshot?.counts.paused    ?? 0;
  const max      = snapshot?.maxConcurrentAgents ?? 0;

  return (
    <div className="flex-shrink-0 grid grid-cols-2 gap-2">
      <StatTile label="Running"  value={running}  sub="Active"     valueColor={running  > 0 ? 'var(--success)'  : undefined} />
      <StatTile label="Backoff"  value={retrying} sub="Retrying"   valueColor={retrying > 0 ? 'var(--warning)'  : undefined} />
      <StatTile label="Paused"   value={paused}   sub="Waiting"    valueColor={paused   > 0 ? 'var(--danger)'   : undefined} />
      <StatTile
        label="Capacity"
        value={max > 0 ? `${String(running)}/${String(max)}` : '—'}
        sub={max > 0 ? `${String(Math.round((running / max) * 100))}% used` : 'No cap'}
        valueColor={max > 0 && running / max >= 0.9 ? 'var(--danger)' : max > 0 && running > 0 ? 'var(--success)' : undefined}
      />
    </div>
  );
}

// ─── Main Dashboard ───────────────────────────────────────────────────────────

// Stable fallbacks — snapshot is replaced on every SSE push, so useMemo on
// snapshot-derived arrays always misses. Module-level constants guarantee a
// stable reference without any memoization overhead.
const EMPTY_PROFILES: string[] = [];
const EMPTY_BACKLOG_STATES: string[] = [];

export default function Dashboard() {
  const { data: issues = [] } = useIssues();
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const invalidateIssues = useInvalidateIssues();
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const { mutateAsync: updateIssueState } = useUpdateIssueState();
  const setIssueProfileMutation = useSetIssueProfile();

  const availableProfiles = snapshot?.availableProfiles ?? EMPTY_PROFILES;
  const backlogStates = snapshot?.backlogStates ?? EMPTY_BACKLOG_STATES;

  const [viewMode, setViewMode] = useState<'board' | 'list' | 'agents'>('board');
  const [search, setSearch] = useState('');
  const [stateFilter, setStateFilter] = useState('all');
  const [loading, setLoading] = useState(false);
  const [apiOffline, setApiOffline] = useState(false);
  const [searchVisible, setSearchVisible] = useState(false);

  useEffect(() => {
    if (snapshot) {
      setApiOffline(false);
      return;
    }
    const t = setTimeout(() => {
      setApiOffline(true);
    }, 8000);
    return () => {
      clearTimeout(t);
    };
  }, [snapshot]);

  const uniqueStates = useMemo(
    () => Array.from(new Set(issues.map((i) => i.state))).sort(),
    [issues],
  );

  const filtered = useMemo(
    () =>
      issues.filter((issue) => {
        const q = search.trim().toLowerCase();
        if (
          q &&
          !issue.identifier.toLowerCase().includes(q) &&
          !issue.title.toLowerCase().includes(q)
        )
          return false;
        if (stateFilter !== 'all' && issue.state !== stateFilter) return false;
        return true;
      }),
    [issues, search, stateFilter],
  );

  const handleRefresh = useCallback(async () => {
    setLoading(true);
    try {
      await fetch('/api/v1/refresh', { method: 'POST' });
      await invalidateIssues();
      // Refresh the state snapshot so running counts and token data update
      // immediately rather than waiting for the next SSE heartbeat (FE-R10-6).
      await useSymphonyStore.getState().refreshSnapshot();
    } catch {
      useToastStore.getState().addToast('Refresh failed — check the server.', 'error');
    } finally {
      setLoading(false);
    }
  }, [invalidateIssues]);

  const handleStateChange = useCallback(
    async (identifier: string, newState: string) => {
      try {
        await updateIssueState({ identifier, state: newState });
        // Invalidation handled by the mutation's onSuccess
      } catch {
        // network / API error — mutation's onError already rolls back optimistic update
      }
    },
    [updateIssueState],
  );

  const handleProfileChange = useCallback(
    (identifier: string, profile: string) => {
      setIssueProfileMutation.mutate({ identifier, profile });
    },
    [setIssueProfileMutation],
  );

  return (
    <>
      <PageMeta title="Symphony | Dashboard" description="Symphony agent orchestration dashboard" />
      <div className="space-y-[14px]">
        {/* Project / repo filter indicator */}
        <ProjectSelector />

        {/* Hero-compact banner with inline stats */}
        <div
          className="relative overflow-hidden rounded-[var(--radius-lg)] px-[22px] py-[18px]"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--line)' }}
        >
          {/* radial glow per spec */}
          <div
            className="pointer-events-none absolute inset-0"
            style={{ background: 'radial-gradient(ellipse at top left, var(--accent-soft) 0%, transparent 60%)' }}
          />
          <div className="relative z-10 flex items-center justify-between gap-6 flex-wrap">
            {/* Title block */}
            <div className="min-w-0">
              <div className="mb-2">
                <span
                  className="inline-flex items-center rounded-full px-3 py-[5px] text-[11px] font-semibold uppercase tracking-[0.03em]"
                  style={{ background: 'var(--accent-soft)', color: 'var(--accent-strong)' }}
                >
                  Symphony
                </span>
              </div>
              <h1
                className="text-2xl font-bold leading-tight tracking-[-0.03em]"
                style={{
                  background: 'var(--gradient-accent)',
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                }}
              >
                Parallel agent orchestration
              </h1>
              <p className="mt-2 text-[13px] leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                Manage running agents and track issues across states.
              </p>
            </div>

            {/* Compact stats — 2 × 2 grid */}
            <HeroStats />
          </div>
        </div>

        {/* API offline banner */}
        {apiOffline && (
          <div
            className="rounded-[var(--radius-md)] p-4 text-sm"
            style={{ border: '1px solid var(--warning-soft)', background: 'var(--warning-soft)', color: 'var(--warning)' }}
          >
            <p className="mb-1 font-semibold">Cannot reach the Symphony API</p>
            <p className="mb-2 opacity-80">
              Make sure your{' '}
              <code className="rounded px-1 font-mono" style={{ background: 'var(--bg-elevated)' }}>
                WORKFLOW.md
              </code>{' '}
              front matter includes the following and the symphony binary is running:
            </p>
            <pre className="rounded p-2 font-mono text-xs" style={{ background: 'var(--bg-elevated)' }}>
              {'server:\n  port: 8090'}
            </pre>
          </div>
        )}

        {/* Host Pool — shown always */}
        <HostPool />

        {/* Running sessions — header + paused section only */}
        <RunningSessionsTable />

        {/* Retry queue — only rendered when issues are backing off */}
        <RetryQueueTable />

        {/* Issues panel */}
        <div
          className="overflow-hidden rounded-[var(--radius-lg)]"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--line)', boxShadow: 'var(--shadow-sm)' }}
        >
          {/* panel-header */}
          <div
            className="flex items-center justify-between gap-3 border-b px-[18px] py-[14px]"
            style={{ borderColor: 'var(--line)' }}
          >
            <div className="min-w-0">
              <h2
                className="flex items-center gap-2 text-sm font-semibold"
                style={{ color: 'var(--text)', letterSpacing: '-0.01em' }}
              >
                Issues
                <span
                  className="rounded-full px-1.5 py-0.5 text-[10px] font-bold"
                  style={{ background: 'var(--bg-soft)', color: 'var(--text-secondary)' }}
                >
                  {filtered.length}
                </span>
              </h2>
              <p className="mt-1 text-xs" style={{ color: 'var(--text-secondary)' }}>
                Click on any issue card to see full details
              </p>
            </div>
            <div className="flex flex-shrink-0 items-center gap-2">
              {/* Search toggle */}
              <button
                onClick={() => { setSearchVisible((v) => !v); }}
                className="flex h-7 w-7 items-center justify-center rounded-lg text-sm transition-colors"
                style={{
                  border: '1px solid var(--line)',
                  color: searchVisible ? 'var(--accent)' : 'var(--text-secondary)',
                  background: searchVisible ? 'var(--accent-soft)' : 'transparent',
                }}
                title="Toggle search"
                aria-label="Toggle search"
              >
                ⌕
              </button>

              {/* View toggle — segmented */}
              <div
                className="inline-flex items-center gap-0.5 rounded-[var(--radius-md)] border p-[3px]"
                style={{ background: 'var(--bg-elevated)', borderColor: 'var(--line)' }}
              >
                {(['board', 'list', ...(availableProfiles.length > 0 ? ['agents'] : [])] as ('board' | 'list' | 'agents')[]).map((mode) => (
                  <button
                    key={mode}
                    onClick={() => { setViewMode(mode); }}
                    className="rounded-[var(--radius-sm)] px-[14px] text-xs font-semibold transition-all"
                    style={{
                      minHeight: 32,
                      background: viewMode === mode ? 'var(--accent)' : 'transparent',
                      color: viewMode === mode ? '#fff' : 'var(--muted)',
                    }}
                  >
                    {mode === 'board' ? 'Board' : mode === 'list' ? 'List' : 'Agents'}
                  </button>
                ))}
              </div>

              {/* Refresh */}
              <button
                onClick={handleRefresh}
                disabled={loading}
                className="flex h-7 w-7 items-center justify-center rounded-lg text-sm transition-colors disabled:opacity-50"
                style={{ border: '1px solid var(--line)', color: 'var(--text-secondary)' }}
                title={loading ? 'Refreshing…' : 'Refresh issues'}
                aria-label="Refresh issues"
              >
                {loading ? '…' : '↻'}
              </button>
            </div>
          </div>

          {/* section-body */}
          <div className="px-[18px] pb-[18px] pt-[14px]">
            {/* Collapsible search + state filter */}
            {searchVisible && (
              <div className="mb-4 flex flex-wrap gap-3">
                <input
                  type="text"
                  placeholder="Search identifier or title…"
                  value={search}
                  onChange={(e) => { setSearch(e.target.value); }}
                  className="min-w-48 flex-1 rounded-lg px-3 py-2 text-sm focus:outline-none"
                  style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)', color: 'var(--text)' }}
                />
                <select
                  value={stateFilter}
                  onChange={(e) => { setStateFilter(e.target.value); }}
                  className="rounded-lg px-3 py-2 text-sm focus:outline-none"
                  style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)', color: 'var(--text)' }}
                >
                  <option value="all">All States</option>
                  {uniqueStates.map((s) => (
                    <option key={s} value={s}>{s}</option>
                  ))}
                </select>
              </div>
            )}

            {viewMode === 'board' && (
              <BoardView
                issues={filtered}
                onSelect={setSelectedIdentifier}
                onStateChange={handleStateChange}
                availableProfiles={availableProfiles}
                onProfileChange={handleProfileChange}
              />
            )}
            {viewMode === 'list' && (
              <ListView
                issues={filtered}
                onSelect={setSelectedIdentifier}
                availableProfiles={availableProfiles}
                onProfileChange={handleProfileChange}
              />
            )}
            {viewMode === 'agents' && (
              <div className="-mx-4 overflow-x-auto px-4 pb-2 md:-mx-6 md:px-6">
                <AgentQueueView
                  issues={issues}
                  backlogStates={backlogStates}
                  availableProfiles={availableProfiles}
                  onProfileChange={handleProfileChange}
                  onSelect={setSelectedIdentifier}
                />
              </div>
            )}
          </div>
        </div>

        {/* Narrative feed — last 20 orchestration events */}
        <NarrativeFeed />
      </div>
    </>
  );
}
