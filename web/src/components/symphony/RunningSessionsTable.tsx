import { useEffect, useMemo, useRef, useState, useCallback } from 'react';
import { useShallow } from 'zustand/react/shallow';
import { LOG_STABLE_DELAY_MS } from '../../utils/timings';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useUIStore } from '../../store/uiStore';
import type { RunningRow } from '../../types/schemas';
import {
  useCancelIssue,
  useTerminateIssue,
  useResumeIssue,
  useReanalyzeIssue,
  useSetIssueProfile,
  useIssues,
} from '../../queries/issues';
import { fmtMs, stateBadgeColor } from '../../utils/format';
import Badge from '../ui/badge/Badge';
import { EMPTY_PROFILE_LABEL, EMPTY_PROFILES } from '../../utils/format';
import { SessionAccordion } from './SessionAccordion';

// Stable empty references — module-level to avoid reference equality churn
const EMPTY_RUNNING: RunningRow[] = [];
const EMPTY_PAUSED: string[] = [];
const EMPTY_PAUSED_WITH_PR: Record<string, string> = {};


function useStableRunning(running: RunningRow[]): RunningRow[] {
  const [stable, setStable] = useState<RunningRow[]>(running);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (running.length > 0) {
      setStable(running);
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    } else if (timerRef.current === null) {
      timerRef.current = setTimeout(() => {
        setStable([]);
        timerRef.current = null;
      }, LOG_STABLE_DELAY_MS);
    }
    return () => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [running]);

  return running.length > 0 ? running : stable;
}

export default function RunningSessionsTable() {
  const { rawRunning, paused, pausedWithPR, availableProfiles } = useSymphonyStore(
    useShallow((s) => ({
      rawRunning: s.snapshot?.running ?? EMPTY_RUNNING,
      paused: s.snapshot?.paused ?? EMPTY_PAUSED,
      pausedWithPR: s.snapshot?.pausedWithPR ?? EMPTY_PAUSED_WITH_PR,
      availableProfiles: s.snapshot?.availableProfiles ?? EMPTY_PROFILES,
    })),
  );
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const cancelIssueMutation = useCancelIssue();
  const terminateIssueMutation = useTerminateIssue();
  const resumeIssueMutation = useResumeIssue();
  const reanalyzeIssueMutation = useReanalyzeIssue();
  const setIssueProfileMutation = useSetIssueProfile();
  const { data: issues } = useIssues();

  const running = useStableRunning(rawRunning);

  const profileMap = useMemo(
    () =>
      Object.fromEntries(
        (issues ?? [])
          .filter((i): i is typeof i & { agentProfile: string } => Boolean(i.agentProfile))
          .map((i) => [i.identifier, i.agentProfile]),
      ),
    [issues],
  );

  const expandedId = useUIStore((s) => s.expandedRunningId);
  const setExpandedId = useUIStore((s) => s.setExpandedRunningId);
  const expandedPausedId = useUIStore((s) => s.expandedPausedId);
  const setExpandedPausedId = useUIStore((s) => s.setExpandedPausedId);

  const sorted = useMemo(
    () => [...running].sort(
      (a, b) => new Date(a.startedAt).getTime() - new Date(b.startedAt).getTime(),
    ),
    [running],
  );

  const toggle = useCallback((id: string) => {
    setExpandedId(expandedId === id ? null : id);
  }, [expandedId, setExpandedId]);

  const togglePaused = useCallback((id: string) => {
    setExpandedPausedId(expandedPausedId === id ? null : id);
  }, [expandedPausedId, setExpandedPausedId]);

  if (sorted.length === 0 && paused.length === 0) {
    return (
      <div
        className="rounded-[var(--radius-md)] p-8 text-center text-sm border border-theme-line bg-theme-panel text-theme-muted"
      >
        No agents running
      </div>
    );
  }

  return (
    <div
      className="overflow-hidden rounded-[var(--radius-md)] border border-theme-line bg-theme-bg-elevated"
    >
      {/* Header — visible whenever there are running or paused sessions */}
      {sorted.length > 0 && (
        <div
          className="flex items-center justify-between px-4 py-[14px]"
          style={{ borderBottom: paused.length > 0 ? '1px solid var(--line)' : undefined, borderColor: 'var(--line)' }}
        >
          <h3 className="text-[15px] font-semibold text-theme-text">
            Running Sessions
          </h3>
          <span
            className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium bg-theme-success-soft text-theme-success"
          >
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-current" />
            {sorted.length} active
          </span>
        </div>
      )}

      {/* Running session rows */}
      {sorted.map((row) => (
        <div
          key={row.identifier}
          className="border-t border-theme-line"
        >
          {/* Clickable row */}
          <div
            role="button"
            tabIndex={0}
            onClick={() => { toggle(row.identifier); }}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') toggle(row.identifier); }}
            className="grid items-center px-4 py-[14px] cursor-pointer transition-colors hover:bg-[var(--bg-soft)] select-none"
            style={{ gridTemplateColumns: '24px 100px 80px 56px 1fr 72px auto', gap: '14px' }}
          >
            {/* Chevron */}
            <span
              className="text-[10px] transition-transform duration-200"
              style={{
                color: 'var(--muted)',
                transform: expandedId === row.identifier ? 'rotate(90deg)' : 'none',
                display: 'inline-block',
              }}
            >
              ▶
            </span>

            {/* Identifier — click opens detail slide */}
            <span
              role="button"
              tabIndex={0}
              className="font-mono text-sm font-semibold cursor-pointer hover:underline truncate text-theme-accent"
              
              onClick={(e) => { e.stopPropagation(); setSelectedIdentifier(row.identifier); }}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.stopPropagation(); setSelectedIdentifier(row.identifier); } }}
            >
              {row.identifier}
            </span>

            {/* State badge */}
            <Badge color={stateBadgeColor(row.state)} size="sm">
              {row.state}
            </Badge>

            {/* Turn count */}
            <span className="text-sm text-theme-text-secondary">
              {row.turnCount ?? '—'}
            </span>

            {/* Last event */}
            <span
              className="truncate text-xs font-mono text-theme-text-secondary"
              
              title={row.lastEvent ?? undefined}
            >
              {row.lastEvent ? row.lastEvent.slice(0, 100) : '—'}
            </span>

            {/* Elapsed */}
            <span className="font-mono text-xs text-theme-muted">
              {fmtMs(row.elapsedMs)}
            </span>

            {/* Actions */}
            <div className="flex gap-2 flex-shrink-0" onClick={(e) => { e.stopPropagation(); }}>
              <button
                onClick={() => { cancelIssueMutation.mutate(row.identifier); }}
                className="inline-flex items-center rounded-[var(--radius-sm)] border px-3 py-1.5 text-xs font-medium transition-all"
                style={{ background: 'var(--warning-soft)', borderColor: 'var(--warning-soft)', color: 'var(--warning)' }}
              >
                ⏸ Pause
              </button>
              <button
                onClick={() => { terminateIssueMutation.mutate(row.identifier); }}
                className="inline-flex items-center rounded-[var(--radius-sm)] border px-3 py-1.5 text-xs font-medium transition-all"
                style={{ background: 'var(--danger-soft)', borderColor: 'var(--danger-soft)', color: 'var(--danger)' }}
              >
                ✕ Cancel
              </button>
            </div>
          </div>

          {/* Expandable log accordion */}
          {expandedId === row.identifier && (
            <SessionAccordion
              identifier={row.identifier}
              workerHost={row.workerHost}
              sessionId={row.sessionId}
            />
          )}
        </div>
      ))}

      {/* Paused section — inside the same panel */}
      {paused.length > 0 && (
        <div style={{ borderTop: sorted.length > 0 ? '1px solid var(--line)' : undefined, background: 'rgba(245,158,11,0.03)' }}>
          <div className="px-4 py-3">
            <span
              className="text-xs font-semibold uppercase tracking-[0.05em] text-theme-warning"
            >
              ⏸ Paused ({paused.length})
            </span>
          </div>

          <div className="pb-3 space-y-0">
            {paused.map((identifier) => {
              const prURL = pausedWithPR[identifier];
              const issueTitle = issues?.find((i) => i.identifier === identifier)?.title;
              const isExpanded = expandedPausedId === identifier;
              return (
                <div
                  key={identifier}
                  className="border-b last:border-b-0 border-theme-line"
                >
                  {/* Paused row — click to expand accordion */}
                  <div
                    role="button"
                    tabIndex={0}
                    onClick={() => { togglePaused(identifier); }}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') togglePaused(identifier); }}
                    className="flex flex-wrap items-center gap-2 px-4 py-3 cursor-pointer transition-colors hover:bg-[var(--bg-soft)]"
                  >
                    {/* Chevron */}
                    <span
                      className="text-[10px] transition-transform duration-200 text-theme-muted"
                      style={{ transform: isExpanded ? 'rotate(90deg)' : 'none' }}
                    >
                      ▶
                    </span>

                    {/* Identifier */}
                    <span
                      role="button"
                      tabIndex={0}
                      className="font-mono text-sm font-semibold cursor-pointer hover:underline text-theme-warning"
                      onClick={(e) => { e.stopPropagation(); setSelectedIdentifier(identifier); }}
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.stopPropagation(); setSelectedIdentifier(identifier); } }}
                    >
                      {identifier}
                    </span>

                    {/* Title — truncated, fills remaining space */}
                    {issueTitle && (
                      <span className="hidden sm:inline truncate text-[13px] text-theme-text-secondary min-w-0 flex-1">
                        {issueTitle}
                      </span>
                    )}

                    {/* PR + profile badges */}
                    <div className="flex items-center gap-2 ml-auto" onClick={(e) => { e.stopPropagation(); }}>
                      {prURL && (
                        <a
                          href={prURL}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium flex-shrink-0 bg-theme-accent-soft text-theme-accent"
                          onClick={(e) => { e.stopPropagation(); }}
                        >
                          PR
                        </a>
                      )}
                      {availableProfiles.length > 0 && (
                        <select
                          value={profileMap[identifier] ?? ''}
                          onChange={(e) => { setIssueProfileMutation.mutate({ identifier, profile: e.target.value }); }}
                          onClick={(e) => { e.stopPropagation(); }}
                          className="rounded border border-theme-line bg-theme-panel-strong text-theme-text px-1.5 py-0.5 text-[10px] focus:outline-none flex-shrink-0"
                        >
                          <option value="">{EMPTY_PROFILE_LABEL}</option>
                          {availableProfiles.map((p) => (
                            <option key={p} value={p}>{p}</option>
                          ))}
                        </select>
                      )}
                    </div>

                    {/* Actions — wrap on mobile */}
                    <div className="flex flex-shrink-0 gap-1.5 w-full sm:w-auto sm:ml-0 mt-1 sm:mt-0" onClick={(e) => { e.stopPropagation(); }}>
                      {prURL && (
                        <button
                          onClick={() => { reanalyzeIssueMutation.mutate(identifier); }}
                          className="btn-action-reanalyze inline-flex items-center rounded-[var(--radius-sm)] border px-3 py-1.5 text-xs font-medium transition-all"
                          style={{ background: 'var(--accent-soft)', borderColor: 'var(--accent-soft)', color: 'var(--accent)' }}
                        >
                          Re-analyze
                        </button>
                      )}
                      <button
                        onClick={() => { resumeIssueMutation.mutate(identifier); }}
                        className="btn-action-resume inline-flex items-center rounded-[var(--radius-sm)] border px-3 py-1.5 text-xs font-medium transition-all"
                        style={{ background: 'var(--success-soft)', borderColor: 'var(--success-soft)', color: 'var(--success)' }}
                      >
                        ▶ Resume
                      </button>
                      <button
                        onClick={() => { terminateIssueMutation.mutate(identifier); }}
                        className="btn-action-cancel inline-flex items-center rounded-[var(--radius-sm)] border px-3 py-1.5 text-xs font-medium transition-all"
                        style={{ background: 'var(--danger-soft)', borderColor: 'var(--danger-soft)', color: 'var(--danger)' }}
                      >
                        ✕ Discard
                      </button>
                    </div>
                  </div>

                  {/* Expandable accordion — reuses SessionAccordion */}
                  {isExpanded && (
                    <SessionAccordion
                      identifier={identifier}
                      workerHost={undefined}
                      sessionId={undefined}
                    />
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Empty state */}
      {sorted.length === 0 && paused.length === 0 && (
        <div className="px-4 py-8 text-center text-sm text-theme-muted">
          No agents running
        </div>
      )}
    </div>
  );
}
