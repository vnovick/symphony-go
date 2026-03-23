import { useEffect, useMemo, useRef, useState } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import Badge from '../ui/badge/Badge';
import type { IssueLogEntry, RunningRow } from '../../types/symphony';
import {
  useCancelIssue,
  useTerminateIssue,
  useResumeIssue,
  useReanalyzeIssue,
  useSetIssueProfile,
  useIssues,
} from '../../queries/issues';
import { useIssueLogs } from '../../queries/logs';
import { fmtMs, stateBadgeColor } from '../../utils/format';
import { entryStyle } from '../../utils/logFormatting';

// Stable empty references — avoids creating new array/object instances on every render,
// which would break the reference-equality guards in useStableRunning's render-phase setState.
const EMPTY_RUNNING: RunningRow[] = [];
const EMPTY_PAUSED: string[] = [];
const EMPTY_PAUSED_WITH_PR: Record<string, string> = {};
const EMPTY_PROFILES: string[] = [];

function useStableRunning(running: RunningRow[]): RunningRow[] {
  const [stable, setStable] = useState<RunningRow[]>(running);
  const [prevRunning, setPrevRunning] = useState(running);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Render-phase derived-state: sync stable whenever running changes while non-empty.
  // This is React's getDerivedStateFromProps equivalent for hooks (see React docs on
  // "Storing information from previous renders").
  if (running !== prevRunning) {
    setPrevRunning(running);
    if (running.length > 0) {
      setStable(running);
    }
  }

  useEffect(() => {
    if (running.length === 0 && timerRef.current === null) {
      // Delay clearing stable by 5 s to avoid layout flicker on transient empty states.
      timerRef.current = setTimeout(() => {
        setStable([]);
        timerRef.current = null;
      }, 5000);
    } else if (running.length > 0 && timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
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

function LogPanel({ entries }: { entries: IssueLogEntry[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const followRef = useRef(true);

  const onScroll = () => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    followRef.current = scrollHeight - scrollTop - clientHeight < 40;
  };

  useEffect(() => {
    if (followRef.current && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [entries]);

  if (entries.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-gray-400">
        No entries yet
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      onScroll={onScroll}
      className="h-full space-y-0.5 overflow-y-auto p-3 font-mono text-xs leading-relaxed"
    >
      {entries.map((entry, i) => {
        const style = entryStyle(entry.event, entry.level);
        return (
          <div key={i} className="flex gap-2">
            {entry.time && (
              <span className="w-14 shrink-0 text-right text-gray-500">{entry.time}</span>
            )}
            <span className={`shrink-0 ${style.textClass}`}>{style.prefixChar}</span>
            <span className={`flex-1 break-words whitespace-pre-wrap ${style.textClass}`}>
              {entry.message}
            </span>
          </div>
        );
      })}
    </div>
  );
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
  const [prevSectionCount, setPrevSectionCount] = useState(0);
  const { data: logs = [] } = useIssueLogs(identifier, true);
  const sections = buildSections(logs);

  // Render-phase derived state: auto-advance to newest subagent when sections grow.
  if (sections.length !== prevSectionCount) {
    setPrevSectionCount(sections.length);
    if (
      sections.length > prevSectionCount &&
      (selectedIdx === prevSectionCount - 1 || prevSectionCount === 0)
    ) {
      setSelectedIdx(sections.length - 1);
    }
  }

  const active = sections[selectedIdx] ?? sections[0];

  return (
    <div className="border-t border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-900/30">
      <div className="flex items-center gap-6 border-b border-gray-200 px-4 py-2 font-mono text-xs text-gray-500 dark:border-gray-700">
        <span>
          Worker: <span className="text-gray-700 dark:text-gray-300">{workerHost || 'local'}</span>
        </span>
        {sessionId && (
          <span title={sessionId}>
            Session:{' '}
            <span className="text-gray-700 dark:text-gray-300">{sessionId.slice(0, 8)}</span>
          </span>
        )}
      </div>
      <div className="flex" style={{ height: 240 }}>
        <div className="flex w-48 flex-shrink-0 flex-col border-r border-gray-200 dark:border-gray-700">
          <div className="border-b border-gray-200 px-3 py-2 dark:border-gray-700">
            <span className="text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
              {sections.length > 1
                ? `${String(sections.length - 1)} subagent${sections.length > 2 ? 's' : ''}`
                : 'Logs'}
            </span>
          </div>
          <div className="flex-1 overflow-y-auto">
            {sections.map((sec, i) => (
              <button
                key={i}
                onClick={() => {
                  setSelectedIdx(i);
                }}
                className={`flex w-full items-center gap-2 border-b border-gray-100 px-3 py-2 text-left text-xs transition-colors dark:border-gray-800 ${
                  i === selectedIdx
                    ? 'bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300'
                    : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800/50'
                }`}
              >
                <span className={`text-xs ${sec.isSubagent ? 'text-purple-500' : 'text-gray-400'}`}>
                  {sec.isSubagent ? '↗' : '◈'}
                </span>
                <span className="flex-1 truncate font-mono">{sec.label}</span>
                <span className="text-gray-400">{sec.entries.length}</span>
              </button>
            ))}
          </div>
        </div>
        <div className="flex-1 overflow-hidden">
          <LogPanel entries={active.entries} />
        </div>
      </div>
    </div>
  );
}

export default function RunningSessionsTable() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
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

  const sorted = [...running].sort(
    (a, b) => new Date(a.startedAt).getTime() - new Date(b.startedAt).getTime(),
  );

  const toggle = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  };

  if (sorted.length === 0 && paused.length === 0) {
    return (
      <div className="rounded-xl border border-gray-200 bg-white p-8 text-center text-sm text-gray-400 dark:border-gray-700 dark:bg-gray-900">
        No agents running
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {sorted.length > 0 && (
        <div className="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900">
          <div className="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-gray-700">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">
              Running Sessions
            </h3>
            <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-100 px-2.5 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-blue-500" />
              {sorted.length} active
            </span>
          </div>

          <div>
            {sorted.map((row) => (
              <div
                key={row.identifier}
                className="border-b border-gray-100 last:border-b-0 dark:border-gray-800"
              >
                <div
                  onClick={() => {
                    toggle(row.identifier);
                  }}
                  className="flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-gray-50 dark:hover:bg-gray-800/30"
                >
                  <button className="text-xs text-gray-400 transition-transform duration-150">
                    <span
                      className={`inline-block transition-transform ${expandedId === row.identifier ? 'rotate-90' : ''}`}
                    >
                      ▶
                    </span>
                  </button>

                  <span className="min-w-[100px] font-mono text-sm font-medium text-gray-900 dark:text-white">
                    {row.identifier}
                  </span>

                  <Badge color={stateBadgeColor(row.state)} size="sm">
                    {row.state}
                  </Badge>

                  {row.backend && (
                    <span
                      className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ${
                        row.backend === 'claude'
                          ? 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
                          : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                      }`}
                    >
                      {row.backend}
                    </span>
                  )}

                  {profileMap[row.identifier] && (
                    <span className="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-400">
                      {profileMap[row.identifier]}
                    </span>
                  )}

                  <span className="w-12 text-center text-sm text-gray-500 dark:text-gray-400">
                    t{row.turnCount}
                  </span>

                  <span
                    className="min-w-0 flex-1 truncate text-sm text-gray-500 dark:text-gray-400"
                    title={row.lastEvent}
                  >
                    {row.lastEvent ? row.lastEvent.slice(0, 60) : '—'}
                  </span>

                  <span className="w-16 font-mono text-xs text-gray-500 dark:text-gray-400">
                    {fmtMs(row.elapsedMs)}
                  </span>

                  <div
                    className="flex gap-2"
                    onClick={(e) => {
                      e.stopPropagation();
                    }}
                  >
                    <button
                      onClick={() => {
                        cancelIssueMutation.mutate(row.identifier);
                      }}
                      className="rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400 dark:hover:bg-amber-900/30"
                    >
                      ⏸ Pause
                    </button>
                    <button
                      onClick={() => {
                        terminateIssueMutation.mutate(row.identifier);
                      }}
                      className="rounded-md border border-red-200 bg-red-50 px-2.5 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400 dark:hover:bg-red-900/30"
                    >
                      ✕ Cancel
                    </button>
                  </div>
                </div>

                {expandedId === row.identifier && (
                  <SessionAccordion
                    identifier={row.identifier}
                    workerHost={row.workerHost}
                    sessionId={row.sessionId}
                  />
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {paused.length > 0 && (
        <div className="overflow-hidden rounded-xl border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/10">
          <div className="flex items-center gap-2 border-b border-amber-200 px-4 py-2 dark:border-amber-800">
            <span className="text-xs font-semibold tracking-wider text-amber-700 uppercase dark:text-amber-400">
              ⏸ Paused ({paused.length})
            </span>
          </div>
          {paused.map((identifier) => {
            const prURL = pausedWithPR[identifier];
            return (
              <div
                key={identifier}
                className="flex items-center justify-between border-t border-amber-200 px-4 py-3 first:border-t-0 dark:border-amber-800"
              >
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm font-medium text-amber-800 dark:text-amber-300">
                    {identifier}
                  </span>
                  {prURL && (
                    <a
                      href={prURL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center rounded-md bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700 hover:bg-purple-200 dark:bg-purple-900/30 dark:text-purple-300"
                    >
                      Open PR
                    </a>
                  )}
                  {availableProfiles.length > 0 && (
                    <select
                      value={profileMap[identifier] ?? ''}
                      onChange={(e) => {
                        setIssueProfileMutation.mutate({ identifier, profile: e.target.value });
                      }}
                      onClick={(e) => {
                        e.stopPropagation();
                      }}
                      className="rounded border border-amber-200 bg-white px-2 py-0.5 text-xs text-amber-800 focus:outline-none dark:border-amber-700 dark:bg-transparent dark:text-amber-300"
                    >
                      <option value="">No agent</option>
                      {availableProfiles.map((p) => (
                        <option key={p} value={p}>
                          {p}
                        </option>
                      ))}
                    </select>
                  )}
                </div>
                <div className="flex gap-2">
                  {prURL && (
                    <button
                      onClick={() => {
                        reanalyzeIssueMutation.mutate(identifier);
                      }}
                      className="rounded-md border border-purple-200 bg-white px-2 py-1 text-xs font-medium text-purple-700 transition-colors hover:bg-purple-50 dark:border-purple-700 dark:bg-transparent dark:text-purple-400"
                    >
                      🔄 Re-analyze
                    </button>
                  )}
                  <button
                    onClick={() => {
                      resumeIssueMutation.mutate(identifier);
                    }}
                    className="rounded-md border border-green-200 bg-white px-2 py-1 text-xs font-medium text-green-700 transition-colors hover:bg-green-50 dark:border-green-700 dark:text-green-400"
                  >
                    ▶ Resume
                  </button>
                  <button
                    onClick={() => {
                      terminateIssueMutation.mutate(identifier);
                    }}
                    className="rounded-md border border-red-200 bg-white px-2 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-50 dark:border-red-800 dark:text-red-400"
                  >
                    ✕ Discard
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
