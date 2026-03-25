import { useEffect, useMemo, useRef, useState, useCallback } from 'react';
import { LOG_STABLE_DELAY_MS } from '../../utils/timings';
import { useSymphonyStore } from '../../store/symphonyStore';
import type { IssueLogEntry, RunningRow } from '../../types/symphony';
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
import { useIssueLogs } from '../../queries/logs';
import { EMPTY_PROFILE_LABEL, EMPTY_PROFILES } from '../../utils/format';
import { Terminal } from '../ui/Terminal/Terminal';
import type { LogEntry, LogLevel } from '../ui/Terminal/Terminal';

// Stable empty references — module-level to avoid reference equality churn
const EMPTY_RUNNING: RunningRow[] = [];
const EMPTY_PAUSED: string[] = [];
const EMPTY_PAUSED_WITH_PR: Record<string, string> = {};

// Map issue log event types to Terminal log levels
function toTermLevel(event: string, level?: string): LogLevel {
  if (event === 'action') return 'action';
  if (event === 'subagent') return 'subagent';
  if (event === 'warn' || level === 'warn') return 'warn';
  if (event === 'error' || level === 'error') return 'error';
  return 'info';
}

function toTermEntries(logs: IssueLogEntry[]): LogEntry[] {
  return logs.map((e, i) => ({
    ts: i, // ordinal index as ts; time parsing is best-effort
    level: toTermLevel(e.event, e.level),
    message: e.message,
  }));
}

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

interface LogSection {
  label: string;
  isSubagent: boolean;
  entries: IssueLogEntry[];
}

function buildSections(entries: IssueLogEntry[]): LogSection[] {
  const sections: LogSection[] = [{ label: 'Main', isSubagent: false, entries: [] }];
  for (const entry of entries) {
    if (entry.event === 'subagent') {
      sections.push({
        label: entry.message.slice(0, 45),
        isSubagent: true,
        entries: [entry],
      });
    } else {
      sections[sections.length - 1].entries.push(entry);
    }
  }
  return sections;
}

function SessionAccordion({
  identifier,
  workerHost,
  sessionId,
}: {
  identifier: string;
  workerHost: string | undefined;
  sessionId: string | undefined;
}) {
  const [selectedIdx, setSelectedIdx] = useState(0);
  const prevSectionCountRef = useRef(0);
  const { data: logs = [] } = useIssueLogs(identifier, true);
  const sections = buildSections(logs);

  useEffect(() => {
    const prev = prevSectionCountRef.current;
    if (sections.length > prev && selectedIdx === prev - 1) {
      setSelectedIdx(sections.length - 1);
    }
    prevSectionCountRef.current = sections.length;
  }, [sections.length, selectedIdx]);

  const active = sections[selectedIdx] ?? sections[0];
  const termEntries = toTermEntries(active.entries);

  return (
    <div
      className="border-t"
      style={{ borderColor: 'var(--line)', background: 'var(--bg)' }}
    >
      {/* Meta row */}
      <div
        className="flex items-center gap-6 border-b px-4 py-2 font-mono text-xs"
        style={{ borderColor: 'var(--line)', color: 'var(--muted)' }}
      >
        <span>
          Worker:{' '}
          <span style={{ color: 'var(--text-secondary)' }}>{workerHost ?? 'local'}</span>
        </span>
        {sessionId && (
          <span title={sessionId}>
            Session:{' '}
            <span style={{ color: 'var(--text-secondary)' }}>{sessionId.slice(0, 8)}</span>
          </span>
        )}
      </div>

      {/* Subagent tabs (left) + Terminal (right) */}
      <div className="flex" style={{ height: 240 }}>
        {/* Subagent sidebar */}
        <div
          className="flex w-44 flex-shrink-0 flex-col border-r"
          style={{ borderColor: 'var(--line)' }}
        >
          <div
            className="border-b px-3 py-2 text-[10px] font-semibold tracking-wider uppercase"
            style={{ borderColor: 'var(--line)', color: 'var(--muted)' }}
          >
            {sections.length > 1
              ? `${String(sections.length - 1)} subagent${sections.length > 2 ? 's' : ''}`
              : 'Logs'}
          </div>
          <div className="flex-1 overflow-y-auto">
            {sections.map((sec, i) => (
              <button
                key={sec.label}
                onClick={() => { setSelectedIdx(i); }}
                className={`terminal-tab flex w-full items-center gap-2 border-b px-3 py-2 text-left text-xs transition-colors`}
                style={{
                  borderColor: 'var(--line)',
                  background: i === selectedIdx ? 'var(--accent-soft)' : 'transparent',
                  color: i === selectedIdx ? 'var(--accent-strong)' : 'var(--text-secondary)',
                }}
              >
                <span style={{ color: sec.isSubagent ? 'var(--purple)' : 'var(--muted)' }}>
                  {sec.isSubagent ? '↗' : '◈'}
                </span>
                <span className="flex-1 truncate font-mono">{sec.label}</span>
                <span style={{ color: 'var(--muted)' }}>{sec.entries.length}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Terminal */}
        <Terminal
          entries={termEntries}
          follow
          showTime={false}
          className="flex-1 h-full"
        />
      </div>
    </div>
  );
}


export default function RunningSessionsTable() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const cancelIssueMutation = useCancelIssue();
  const terminateIssueMutation = useTerminateIssue();
  const resumeIssueMutation = useResumeIssue();
  const reanalyzeIssueMutation = useReanalyzeIssue();
  const setIssueProfileMutation = useSetIssueProfile();
  const { data: issues } = useIssues();

  const rawRunning = snapshot?.running ?? EMPTY_RUNNING;
  const running = useStableRunning(rawRunning);
  const paused = snapshot?.paused ?? EMPTY_PAUSED;
  const pausedWithPR = snapshot?.pausedWithPR ?? EMPTY_PAUSED_WITH_PR;
  const availableProfiles = snapshot?.availableProfiles ?? EMPTY_PROFILES;

  const profileMap = useMemo(
    () =>
      Object.fromEntries(
        (issues ?? [])
          .filter((i): i is typeof i & { agentProfile: string } => Boolean(i.agentProfile))
          .map((i) => [i.identifier, i.agentProfile]),
      ),
    [issues],
  );

  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [expandedPausedId, setExpandedPausedId] = useState<string | null>(null);

  const sorted = [...running].sort(
    (a, b) => new Date(a.startedAt).getTime() - new Date(b.startedAt).getTime(),
  );

  const toggle = useCallback((id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  }, []);

  const togglePaused = useCallback((id: string) => {
    setExpandedPausedId((prev) => (prev === id ? null : id));
  }, []);

  if (sorted.length === 0 && paused.length === 0) {
    return (
      <div
        className="rounded-[var(--radius-md)] p-8 text-center text-sm"
        style={{
          border: '1px solid var(--line)',
          background: 'var(--panel)',
          color: 'var(--muted)',
        }}
      >
        No agents running
      </div>
    );
  }

  return (
    <div
      className="overflow-hidden rounded-[var(--radius-md)]"
      style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)' }}
    >
      {/* Header — visible whenever there are running or paused sessions */}
      {sorted.length > 0 && (
        <div
          className="flex items-center justify-between px-4 py-[14px]"
          style={{ borderBottom: paused.length > 0 ? '1px solid var(--line)' : undefined, borderColor: 'var(--line)' }}
        >
          <h3 className="text-[15px] font-semibold" style={{ color: 'var(--text)' }}>
            Running Sessions
          </h3>
          <span
            className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium"
            style={{ background: 'var(--success-soft)', color: 'var(--success)' }}
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
          className="border-t"
          style={{ borderColor: 'var(--line)' }}
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
              className="font-mono text-sm font-semibold cursor-pointer hover:underline truncate"
              style={{ color: 'var(--accent)' }}
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
            <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              {row.turnCount ?? '—'}
            </span>

            {/* Last event */}
            <span
              className="truncate text-xs font-mono"
              style={{ color: 'var(--text-secondary)' }}
              title={row.lastEvent ?? undefined}
            >
              {row.lastEvent ? row.lastEvent.slice(0, 100) : '—'}
            </span>

            {/* Elapsed */}
            <span className="font-mono text-xs" style={{ color: 'var(--muted)' }}>
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
              className="text-xs font-semibold uppercase tracking-[0.05em]"
              style={{ color: 'var(--warning)' }}
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
                  className="border-b last:border-b-0"
                  style={{ borderColor: 'var(--line)' }}
                >
                  {/* Paused row — click to expand accordion */}
                  <div
                    role="button"
                    tabIndex={0}
                    onClick={() => { togglePaused(identifier); }}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') togglePaused(identifier); }}
                    className="grid items-center px-4 py-[14px] cursor-pointer transition-colors hover:bg-[var(--bg-soft)]"
                    style={{ gridTemplateColumns: '24px 100px 1fr auto', gap: '14px' }}
                  >
                    {/* Chevron */}
                    <span
                      className="text-[10px] transition-transform duration-200"
                      style={{
                        color: 'var(--muted)',
                        transform: isExpanded ? 'rotate(90deg)' : 'none',
                        display: 'inline-block',
                      }}
                    >
                      ▶
                    </span>

                    {/* Identifier — click opens detail slide */}
                    <span
                      role="button"
                      tabIndex={0}
                      className="font-mono text-sm font-semibold cursor-pointer hover:underline"
                      style={{ color: 'var(--warning)' }}
                      onClick={(e) => { e.stopPropagation(); setSelectedIdentifier(identifier); }}
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.stopPropagation(); setSelectedIdentifier(identifier); } }}
                    >
                      {identifier}
                    </span>

                    {/* Title + PR + profile */}
                    <div className="min-w-0 flex items-center gap-3" onClick={(e) => { e.stopPropagation(); }}>
                      {issueTitle && (
                        <span className="truncate text-[13px]" style={{ color: 'var(--text-secondary)' }}>
                          {issueTitle}
                        </span>
                      )}
                      {prURL && (
                        <a
                          href={prURL}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium flex-shrink-0"
                          style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}
                          onClick={(e) => { e.stopPropagation(); }}
                        >
                          Open PR
                        </a>
                      )}
                      {availableProfiles.length > 0 && (
                        <select
                          value={profileMap[identifier] ?? ''}
                          onChange={(e) => { setIssueProfileMutation.mutate({ identifier, profile: e.target.value }); }}
                          onClick={(e) => { e.stopPropagation(); }}
                          className="rounded-[var(--radius-sm)] border px-2 py-1 text-xs focus:outline-none flex-shrink-0"
                          style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)', color: 'var(--text)', cursor: 'pointer' }}
                        >
                          <option value="">{EMPTY_PROFILE_LABEL}</option>
                          {availableProfiles.map((p) => (
                            <option key={p} value={p}>{p}</option>
                          ))}
                        </select>
                      )}
                    </div>

                    {/* Actions */}
                    <div className="flex flex-shrink-0 gap-2" onClick={(e) => { e.stopPropagation(); }}>
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
        <div className="px-4 py-8 text-center text-sm" style={{ color: 'var(--muted)' }}>
          No agents running
        </div>
      )}
    </div>
  );
}
