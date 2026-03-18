import { useState, useMemo, useCallback, useEffect } from 'react';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
  type DragOverEvent,
} from '@dnd-kit/core';
import PageMeta from '../../components/common/PageMeta';
import RateLimitBar from '../../components/symphony/RateLimitBar';
import RunningSessionsTable from '../../components/symphony/RunningSessionsTable';
import StatusStrip from '../../components/symphony/StatusStrip';
import IssueCard from '../../components/symphony/IssueCard';
import BoardColumn from '../../components/symphony/BoardColumn';
import Badge from '../../components/ui/badge/Badge';
import { useSymphonyStore } from '../../store/symphonyStore';
import type { TrackerIssue } from '../../types/symphony';
import {
  useIssues,
  useInvalidateIssues,
  useUpdateIssueState,
  useCancelIssue,
  useResumeIssue,
} from '../../queries/issues';
import { orchDotClass, stateBadgeColor } from '../../utils/format';

// ─── Board view ───────────────────────────────────────────────────────────────

interface BoardProps {
  issues: TrackerIssue[];
  onSelect: (id: string) => void;
  onStateChange: (identifier: string, newState: string) => void;
}

function BoardView({ issues, onSelect, onStateChange }: BoardProps) {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const snapshotLoaded = snapshot !== null;
  const [activeIssue, setActiveIssue] = useState<TrackerIssue | null>(null);
  const [overId, setOverId] = useState<string | null>(null);

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));

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
    setActiveIssue((e.active.data.current as { issue: TrackerIssue }).issue);
  }, []);

  const onDragOver = useCallback((e: DragOverEvent) => {
    setOverId(e.over ? String(e.over.id) : null);
  }, []);

  const onDragEnd = useCallback(
    (e: DragEndEvent) => {
      setActiveIssue(null);
      setOverId(null);
      if (!e.over) return;
      const newState = String(e.over.id);
      const oldState = (e.active.data.current as { issue: TrackerIssue }).issue.state;
      if (newState !== oldState) {
        onStateChange(String(e.active.id), newState);
      }
    },
    [onStateChange],
  );

  if (columns.length === 0) {
    return (
      <div className="rounded-2xl border border-gray-200 bg-white px-6 py-12 text-center text-sm text-gray-400 dark:border-gray-800 dark:bg-white/[0.03]">
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
      <div className="flex gap-4 overflow-x-auto pb-4">
        {columns.map(([state, colIssues]) => (
          <BoardColumn
            key={state}
            state={state}
            issues={colIssues}
            isOver={overId === state}
            onSelect={onSelect}
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
    <span className={`ml-1 ${active ? 'text-brand-500' : 'text-gray-300 dark:text-gray-600'}`}>
      {active ? (dir === 'asc' ? '↑' : '↓') : '↕'}
    </span>
  );
}

function ListView({
  issues,
  onSelect,
}: {
  issues: TrackerIssue[];
  onSelect: (id: string) => void;
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

  const sorted = useMemo(
    () =>
      [...issues].sort((a, b) => {
        const aVal = sortKey === 'identifier' ? a.identifier : a[sortKey].toLowerCase();
        const bVal = sortKey === 'identifier' ? b.identifier : b[sortKey].toLowerCase();
        const cmp = aVal.localeCompare(bVal);
        return sortDir === 'asc' ? cmp : -cmp;
      }),
    [issues, sortKey, sortDir],
  );

  const th =
    'px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider select-none cursor-pointer hover:text-gray-700 dark:hover:text-gray-200';

  return (
    <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 dark:bg-gray-900/50">
            <tr>
              <th
                className={th}
                onClick={() => {
                  handleSort('identifier');
                }}
              >
                Identifier <SortIcon active={sortKey === 'identifier'} dir={sortDir} />
              </th>
              <th
                className={th}
                onClick={() => {
                  handleSort('title');
                }}
              >
                Title <SortIcon active={sortKey === 'title'} dir={sortDir} />
              </th>
              <th
                className={th}
                onClick={() => {
                  handleSort('state');
                }}
              >
                State <SortIcon active={sortKey === 'state'} dir={sortDir} />
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase dark:text-gray-400">
                Agent
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase dark:text-gray-400">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
            {sorted.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-10 text-center text-sm text-gray-400">
                  No issues match the current filters
                </td>
              </tr>
            )}
            {sorted.map((issue) => (
              <tr
                key={issue.identifier}
                className="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-gray-900/30"
                onClick={() => {
                  onSelect(issue.identifier);
                }}
              >
                <td className="px-4 py-3 whitespace-nowrap">
                  {issue.url ? (
                    <a
                      href={issue.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
                      onClick={(e) => {
                        e.stopPropagation();
                      }}
                    >
                      {issue.identifier}
                    </a>
                  ) : (
                    <span className="font-mono text-sm font-medium text-gray-900 dark:text-white">
                      {issue.identifier}
                    </span>
                  )}
                </td>
                <td className="max-w-xs truncate px-4 py-3 text-gray-700 dark:text-gray-300">
                  {issue.title}
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <Badge size="sm" color={stateBadgeColor(issue.state)}>
                    {issue.state}
                  </Badge>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span className={`inline-flex items-center gap-1 text-xs`}>
                    <span
                      className={`h-2 w-2 rounded-full ${orchDotClass(issue.orchestratorState)}`}
                    />
                    {issue.orchestratorState}
                  </span>
                </td>
                <td
                  className="px-4 py-3 whitespace-nowrap"
                  onClick={(e) => {
                    e.stopPropagation();
                  }}
                >
                  {issue.orchestratorState === 'running' && (
                    <button
                      onClick={() => {
                        cancelIssueMutation.mutate(issue.identifier);
                      }}
                      className="rounded border border-red-200 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400"
                    >
                      ⏸ Pause
                    </button>
                  )}
                  {issue.orchestratorState === 'paused' && (
                    <button
                      onClick={() => {
                        resumeIssueMutation.mutate(issue.identifier);
                      }}
                      className="rounded border border-green-300 px-2 py-1 text-xs text-green-700 hover:bg-green-50 dark:border-green-700 dark:text-green-400"
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

// ─── Main Dashboard ───────────────────────────────────────────────────────────

export default function Dashboard() {
  const { data: issues = [] } = useIssues();
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const invalidateIssues = useInvalidateIssues();
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const updateIssueStateMutation = useUpdateIssueState();

  const [viewMode, setViewMode] = useState<'board' | 'list'>('board');
  const [search, setSearch] = useState('');
  const [stateFilter, setStateFilter] = useState('all');
  const [loading, setLoading] = useState(false);
  const [apiOffline, setApiOffline] = useState(false);

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
    } catch {
      /* network error */
    } finally {
      setLoading(false);
    }
  }, [invalidateIssues]);

  const handleStateChange = useCallback(
    async (identifier: string, newState: string) => {
      await updateIssueStateMutation.mutateAsync({ identifier, state: newState });
      // Invalidation handled by the mutation's onSuccess
    },
    [updateIssueStateMutation],
  );

  const btnBase = 'px-3 py-1.5 text-xs font-medium rounded-full transition-colors';
  const btnActive = `${btnBase} bg-brand-500 text-white`;
  const btnInactive = `${btnBase} text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700`;

  return (
    <>
      <PageMeta title="Symphony | Dashboard" description="Symphony agent orchestration dashboard" />
      <div className="space-y-5">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">
              Simphony
              <span className="bg-brand-50 text-brand-600 dark:bg-brand-900/30 dark:text-brand-400 border-brand-100 dark:border-brand-800 ml-2 inline-flex items-center rounded-full border px-2 py-0.5 align-middle text-xs font-medium">
                live
              </span>
            </h1>
            <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
              Parallel Claude Code agent orchestration
            </p>
          </div>
        </div>

        {/* API offline banner */}
        {apiOffline && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
            <p className="mb-1 font-semibold">Cannot reach the Symphony API</p>
            <p className="mb-2 text-amber-700 dark:text-amber-400">
              Make sure your{' '}
              <code className="rounded bg-amber-100 px-1 font-mono dark:bg-amber-900/40">
                WORKFLOW.md
              </code>{' '}
              front matter includes the following and the symphony binary is running:
            </p>
            <pre className="rounded bg-amber-100 p-2 font-mono text-xs dark:bg-amber-900/40">
              {'server:\n  port: 8090'}
            </pre>
          </div>
        )}

        {/* Status strip */}
        <StatusStrip />

        {/* Running sessions with logs and subagent details */}
        <RunningSessionsTable />

        {/* Rate limits — collapsible */}
        <details className="rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
          <summary className="flex cursor-pointer list-none items-center justify-between px-4 py-2 text-sm font-medium text-gray-700 select-none dark:text-gray-300">
            <span>API Rate Limits</span>
            <span className="text-xs text-gray-400">▸</span>
          </summary>
          <div className="px-4 pb-3">
            <RateLimitBar />
          </div>
        </details>

        {/* Issues board */}
        <div>
          <div className="mb-3 flex flex-wrap items-start justify-between gap-3 sm:flex-nowrap">
            <div className="min-w-0">
              <h2 className="flex items-center gap-2 text-base font-semibold text-gray-900 dark:text-white">
                Issues
                <span className="inline-flex items-center rounded-full bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-400">
                  {filtered.length}
                </span>
              </h2>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                {issues.length} total
                {filtered.length !== issues.length ? ` · ${String(filtered.length)} shown` : ''}
              </p>
            </div>
            <div className="flex flex-shrink-0 items-center gap-2">
              {/* View toggle */}
              <div className="flex items-center gap-0.5 rounded-lg bg-gray-100 p-1 dark:bg-gray-800">
                <button
                  className={viewMode === 'board' ? btnActive : btnInactive}
                  onClick={() => {
                    setViewMode('board');
                  }}
                >
                  ⊞ Board
                </button>
                <button
                  className={viewMode === 'list' ? btnActive : btnInactive}
                  onClick={() => {
                    setViewMode('list');
                  }}
                >
                  ☰ List
                </button>
              </div>
              {/* Refresh */}
              <button
                onClick={handleRefresh}
                disabled={loading}
                className="bg-brand-500 hover:bg-brand-600 flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-medium whitespace-nowrap text-white transition-colors disabled:opacity-50"
              >
                ↻ {loading ? 'Refreshing…' : 'Refresh'}
              </button>
            </div>
          </div>

          {/* Filters */}
          <div className="mb-4 flex flex-wrap gap-3">
            <input
              type="text"
              placeholder="Search identifier or title…"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
              }}
              className="focus:ring-brand-500 min-w-48 flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:ring-2 focus:outline-none dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:placeholder-gray-500"
            />
            <select
              value={stateFilter}
              onChange={(e) => {
                setStateFilter(e.target.value);
              }}
              className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
            >
              <option value="all">All States</option>
              {uniqueStates.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </div>

          {viewMode === 'board' ? (
            <div className="-mx-4 overflow-x-auto px-4 pb-2 md:-mx-6 md:px-6">
              <BoardView
                issues={filtered}
                onSelect={setSelectedIdentifier}
                onStateChange={handleStateChange}
              />
            </div>
          ) : (
            <ListView issues={filtered} onSelect={setSelectedIdentifier} />
          )}
        </div>
      </div>
    </>
  );
}
