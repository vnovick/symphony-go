import { useState, useMemo, useCallback, useEffect } from 'react';
import { useShallow } from 'zustand/react/shallow';
import PageMeta from '../../components/common/PageMeta';
import RunningSessionsTable from '../../components/symphony/RunningSessionsTable';
import RetryQueueTable from '../../components/symphony/RetryQueueTable';
import { HostPool } from '../../components/symphony/HostPool';
import { ProjectSelector } from '../../components/symphony/ProjectSelector';
import { NarrativeFeed } from '../../components/symphony/NarrativeFeed';
import AgentQueueView from '../../components/symphony/AgentQueueView';
import { FilterPills, type FilterPill } from '../../components/symphony/FilterPills';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useUIStore } from '../../store/uiStore';
import { useToastStore } from '../../store/toastStore';
import {
  useIssues,
  useInvalidateIssues,
  useUpdateIssueState,
  useSetIssueProfile,
} from '../../queries/issues';

import { BoardView } from './components/BoardView';
import { ListView } from './components/ListView';
import { HeroStats } from './components/HeroStats';

// ─── Stable fallbacks ─────────────────────────────────────────────────────────
import { EMPTY_PROFILES, EMPTY_STATES, EMPTY_RUNNING, EMPTY_HISTORY } from '../../utils/constants';
const EMPTY_BACKLOG_STATES = EMPTY_STATES;
const EMPTY_ACTIVE_STATES = EMPTY_STATES;
const EMPTY_TERMINAL_STATES = EMPTY_STATES;

export default function Dashboard() {
  const { data: issues = [] } = useIssues();
  const { hasSnapshot, availableProfiles, backlogStates, activeStates, terminalStates, completionState, profileDefs, defaultBackend, running, runHistory } = useSymphonyStore(
    useShallow((s) => ({
      hasSnapshot: s.snapshot !== null,
      availableProfiles: s.snapshot?.availableProfiles ?? EMPTY_PROFILES,
      backlogStates: s.snapshot?.backlogStates ?? EMPTY_BACKLOG_STATES,
      activeStates: s.snapshot?.activeStates ?? EMPTY_ACTIVE_STATES,
      terminalStates: s.snapshot?.terminalStates ?? EMPTY_TERMINAL_STATES,
      completionState: s.snapshot?.completionState ?? '',
      profileDefs: s.snapshot?.profileDefs,
      defaultBackend: s.snapshot?.defaultBackend,
      running: s.snapshot?.running ?? EMPTY_RUNNING,
      runHistory: s.snapshot?.history ?? EMPTY_HISTORY,
    })),
  );
  const backlogStateSet = useMemo(() => new Set(backlogStates), [backlogStates]);
  const runningBackendByIdentifier = useMemo(() => {
    const map: Record<string, string> = {};
    const backlogIdentifiers = new Set(
      issues.filter((i) => backlogStateSet.has(i.state)).map((i) => i.identifier),
    );
    for (const h of runHistory) {
      if (h.backend && !backlogIdentifiers.has(h.identifier)) map[h.identifier] = h.backend;
    }
    for (const r of running) {
      if (r.backend) map[r.identifier] = r.backend;
    }
    return map;
  }, [running, runHistory, issues, backlogStateSet]);

  const invalidateIssues = useInvalidateIssues();
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const { mutateAsync: updateIssueState } = useUpdateIssueState();
  const setIssueProfileMutation = useSetIssueProfile();

  // UI preferences — persisted in Zustand so they survive navigation
  const viewMode = useUIStore((s) => s.dashboardViewMode);
  const setViewMode = useUIStore((s) => s.setDashboardViewMode);
  const search = useUIStore((s) => s.dashboardSearch);
  const setSearch = useUIStore((s) => s.setDashboardSearch);
  const stateFilter = useUIStore((s) => s.dashboardStateFilter);
  const setStateFilter = useUIStore((s) => s.setDashboardStateFilter);

  const [loading, setLoading] = useState(false);
  const [apiOffline, setApiOffline] = useState(false);

  useEffect(() => {
    if (hasSnapshot) {
      setApiOffline(false);
      return;
    }
    const t = setTimeout(() => {
      setApiOffline(true);
    }, 8000);
    return () => {
      clearTimeout(t);
    };
  }, [hasSnapshot]);

  // Build Linear-style filter pills from snapshot states
  const filterPills = useMemo<FilterPill[]>(() => {
    const pills: FilterPill[] = [{ id: 'all', label: 'All Issues', states: [] }];
    if (activeStates.length > 0) {
      pills.push({ id: 'active', label: 'Active', states: activeStates });
    }
    if (backlogStates.length > 0) {
      pills.push({ id: 'backlog', label: 'Backlog', states: backlogStates });
    }
    if (completionState) {
      pills.push({ id: 'review', label: completionState, states: [completionState] });
    }
    if (terminalStates.length > 0) {
      pills.push({ id: 'done', label: 'Done', states: terminalStates });
    }
    return pills;
  }, [activeStates, backlogStates, terminalStates, completionState]);

  // Find matching pill states for filtering
  const activePillStates = useMemo(() => {
    if (stateFilter === 'all') return null;
    const pill = filterPills.find((p) => p.id === stateFilter);
    return pill?.states ?? null;
  }, [stateFilter, filterPills]);

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
        if (activePillStates !== null) {
          const match = activePillStates.some(
            (s) => s.toLowerCase() === issue.state.toLowerCase(),
          );
          if (!match) return false;
        }
        return true;
      }),
    [issues, search, activePillStates],
  );

  const handleRefresh = useCallback(async () => {
    setLoading(true);
    try {
      await fetch('/api/v1/refresh', { method: 'POST' });
      await invalidateIssues();
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
      } catch {
        // mutation's onError already rolls back optimistic update
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
        <ProjectSelector />

        {/* Hero-compact banner — responsive: stacks on mobile */}
        <div
          className="relative overflow-hidden rounded-[var(--radius-lg)] px-4 py-4 sm:px-[22px] sm:py-[18px] border border-theme-line bg-theme-bg-elevated"
        >
          <div
            className="pointer-events-none absolute inset-0"
            style={{ background: 'radial-gradient(ellipse at top left, var(--accent-soft) 0%, transparent 60%)' }}
          />
          <div className="relative z-10 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between sm:gap-6">
            <div className="min-w-0">
              <div className="mb-2">
                <span
                  className="inline-flex items-center rounded-full px-3 py-[5px] text-[11px] font-semibold uppercase tracking-[0.03em] bg-theme-accent-soft text-theme-accent-strong"
                >
                  Symphony
                </span>
              </div>
              <h1
                className="text-xl sm:text-2xl font-bold leading-tight tracking-[-0.03em]"
                style={{
                  background: 'var(--gradient-accent)',
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                }}
              >
                Parallel agent orchestration
              </h1>
              <p className="mt-2 text-[13px] leading-relaxed text-theme-text-secondary">
                Manage running agents and track issues across states.
              </p>
            </div>
            <HeroStats />
          </div>
        </div>

        {apiOffline && (
          <div
            className="rounded-[var(--radius-md)] p-4 text-sm border border-theme-warning-soft bg-theme-warning-soft text-theme-warning"
          >
            <p className="mb-1 font-semibold">Cannot reach the Symphony API</p>
            <p className="mb-2 opacity-80">
              Make sure your{' '}
              <code className="rounded px-1 font-mono bg-theme-bg-elevated">
                WORKFLOW.md
              </code>{' '}
              front matter includes the following and the symphony binary is running:
            </p>
            <pre className="rounded p-2 font-mono text-xs bg-theme-bg-elevated">
              {'server:\n  port: 8090'}
            </pre>
          </div>
        )}

        <HostPool />
        <RunningSessionsTable />
        <RetryQueueTable />

        {/* Issues panel */}
        <div
          className="overflow-hidden rounded-[var(--radius-lg)] border border-theme-line bg-theme-bg-elevated shadow-theme-sm"
        >
          {/* Panel header — search always visible */}
          <div
            className="flex flex-col gap-3 border-b px-4 py-3 sm:px-[18px] sm:py-[14px] border-theme-line"
          >
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <h2
                  className="flex items-center gap-2 text-sm font-semibold tracking-tight text-theme-text"
                >
                  Issues
                  <span
                    className="rounded-full px-1.5 py-0.5 text-[10px] font-bold bg-theme-bg-soft text-theme-text-secondary"
                  >
                    {filtered.length}
                  </span>
                </h2>
              </div>
              <div className="flex flex-shrink-0 items-center gap-2">
                {/* View toggle — segmented */}
                <div
                  className="inline-flex items-center gap-0.5 rounded-[var(--radius-md)] border p-[3px] bg-theme-bg-elevated border-theme-line"
                >
                  {(['board', 'list', ...(availableProfiles.length > 0 ? ['agents'] : [])] as ('board' | 'list' | 'agents')[]).map((mode) => (
                    <button
                      key={mode}
                      onClick={() => { setViewMode(mode); }}
                      className={`rounded-[var(--radius-sm)] px-3 py-1.5 text-xs font-semibold transition-all ${
                        viewMode === mode
                          ? 'bg-theme-accent text-white'
                          : 'text-theme-muted'
                      }`}
                    >
                      {mode === 'board' ? 'Board' : mode === 'list' ? 'List' : 'Agents'}
                    </button>
                  ))}
                </div>

                {/* Refresh */}
                <button
                  onClick={handleRefresh}
                  disabled={loading}
                  className="flex h-7 w-7 items-center justify-center rounded-lg text-sm transition-colors disabled:opacity-50 border border-theme-line text-theme-text-secondary"
                  title={loading ? 'Refreshing…' : 'Refresh issues'}
                  aria-label="Refresh issues"
                >
                  {loading ? '…' : '↻'}
                </button>
              </div>
            </div>

            {/* Filter pills + search — hidden in agents view (agents group by profile, not state) */}
            {viewMode !== 'agents' && (
              <>
                <FilterPills pills={filterPills} activeId={stateFilter} onChange={setStateFilter} />
                <div className="flex gap-3">
                  <input
                    type="text"
                    placeholder="Search identifier or title…"
                    value={search}
                    onChange={(e) => { setSearch(e.target.value); }}
                    className="min-w-0 flex-1 rounded-lg px-3 py-1.5 text-sm focus:outline-none border border-theme-line bg-theme-bg-elevated text-theme-text"
                  />
                </div>
              </>
            )}
          </div>

          <div className="px-4 pb-4 pt-3 sm:px-[18px] sm:pb-[18px] sm:pt-[14px]">
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
                profileDefs={profileDefs}
                runningBackendByIdentifier={runningBackendByIdentifier}
                defaultBackend={defaultBackend}
                backlogStates={backlogStates}
                onProfileChange={handleProfileChange}
              />
            )}
            {viewMode === 'agents' && (
              <div className="-mx-4 overflow-x-auto px-4 pb-2 md:-mx-6 md:px-6">
                <AgentQueueView
                  issues={issues}
                  backlogStates={backlogStates}
                  availableProfiles={availableProfiles}
                  profileDefs={profileDefs}
                  onProfileChange={handleProfileChange}
                  onSelect={setSelectedIdentifier}
                />
              </div>
            )}
          </div>
        </div>

        <NarrativeFeed />
      </div>
    </>
  );
}
