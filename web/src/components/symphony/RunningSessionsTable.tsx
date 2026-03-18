import { useEffect, useRef, useState } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import Badge from '../ui/badge/Badge';
import type { IssueLogEntry, RunningRow } from '../../types/symphony';
import {
  useCancelIssue,
  useTerminateIssue,
  useResumeIssue,
  useReanalyzeIssue,
} from '../../queries/issues';
import { useIssueLogs } from '../../queries/logs';
import { fmtMs, stateBadgeColor } from '../../utils/format';
import { entryStyle } from '../../utils/logFormatting';

// Prevents SSE-reconnect flicker: keeps last non-empty array for 5 s before clearing.
function useStableRunning(running: RunningRow[]): RunningRow[] {
  const stableRef = useRef<RunningRow[]>([]);
  const [, forceUpdate] = useState(0);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (running.length > 0) {
      stableRef.current = running;
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    } else if (stableRef.current.length > 0 && timerRef.current === null) {
      timerRef.current = setTimeout(() => {
        stableRef.current = [];
        timerRef.current = null;
        forceUpdate((n) => n + 1);
      }, 5000);
    }
  }, [running]);

  // eslint-disable-next-line react-hooks/refs
  return running.length > 0 ? running : stableRef.current;
}

interface LogSection {
  label: string;
  isSubagent: boolean;
  entries: IssueLogEntry[];
}

// Split log entries into sections: one "Main" section + one per subagent boundary.
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

// LogPanel: auto-scrolls to bottom unless user has scrolled up (follow mode).
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
      <div className="flex h-full items-center justify-center text-xs text-gray-400 dark:text-gray-500">
        No entries yet
      </div>
    );
  }

  return (
    <div ref={containerRef} onScroll={onScroll} className="h-full space-y-1 overflow-y-auto">
      {entries.map((entry, i) => {
        const style = entryStyle(entry.event, entry.level);
        return (
          <div
            key={i}
            className={`flex gap-1.5 border-l-2 pl-2 font-mono text-xs leading-relaxed ${style.borderClass}`}
          >
            <span className={`flex-1 break-words whitespace-pre-wrap ${style.textClass}`}>
              {style.prefixChar}
              {style.prefixChar !== '·' ? ' ' : ''}
              {entry.message}
            </span>
          </div>
        );
      })}
    </div>
  );
}

// SessionAccordion: expanded panel showing subagent sections + logs.
function SessionAccordion({
  identifier,
  workerHost,
  sessionId,
}: {
  identifier: string;
  workerHost: string;
  sessionId: string;
}) {
  const [selectedIdx, setSelectedIdx] = useState(0);
  const prevSectionCountRef = useRef(0);

  // Sessions in the accordion are always live (running), so isLive = true
  const { data: logs = [] } = useIssueLogs(identifier, true);

  const sections = buildSections(logs);

  // Auto-advance selection to the newest subagent when a new one appears.
  useEffect(() => {
    const count = sections.length;
    if (count > prevSectionCountRef.current) {
      // Only follow if user was already on the last section.
      if (selectedIdx === prevSectionCountRef.current - 1 || prevSectionCountRef.current === 0) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setSelectedIdx(count - 1);
      }
      prevSectionCountRef.current = count;
    }
  }, [sections.length, selectedIdx]);

  const active = sections[selectedIdx] ?? sections[0];

  return (
    <div className="flex flex-col bg-gray-50 dark:bg-gray-900/40" style={{ height: 308 }}>
      {/* Metadata strip — full width above the two-column layout */}
      <div className="flex flex-shrink-0 gap-6 border-b border-gray-200 px-4 py-2 font-mono text-xs text-gray-500 dark:border-gray-700 dark:text-gray-400">
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
      <div className="flex" style={{ height: 280 }}>
        {/* Left: section list */}
        <div className="flex w-52 flex-shrink-0 flex-col border-r border-gray-200 dark:border-gray-700">
          <div className="border-b border-gray-200 px-3 py-2 dark:border-gray-700">
            <p className="text-xs font-semibold tracking-wider text-gray-500 uppercase dark:text-gray-400">
              {sections.length > 1
                ? `${String(sections.length - 1)} subagent${sections.length > 2 ? 's' : ''}`
                : 'Logs'}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto">
            {sections.map((sec, i) => (
              <button
                key={i}
                onClick={() => {
                  setSelectedIdx(i);
                }}
                className={`flex w-full items-start gap-1.5 border-b border-gray-100 px-3 py-2 text-left text-xs dark:border-gray-800 ${
                  i === selectedIdx
                    ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                    : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'
                }`}
              >
                <span className="mt-0.5 flex-shrink-0 text-gray-400">
                  {sec.isSubagent ? '↗' : '◈'}
                </span>
                <span className="flex-1 truncate font-mono">{sec.label}</span>
                <span className="flex-shrink-0 text-gray-400">{sec.entries.length}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Right: log entries */}
        <div className="flex-1 overflow-hidden p-3">
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
  const rawRunning = snapshot?.running ?? [];
  const running = useStableRunning(rawRunning);
  const paused = snapshot?.paused ?? [];
  const pausedWithPR = snapshot?.pausedWithPR ?? {};
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const sorted = [...running].sort(
    (a, b) => new Date(a.startedAt).getTime() - new Date(b.startedAt).getTime(),
  );

  const toggle = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  };

  const colGrid =
    'grid grid-cols-[24px_140px_110px_64px_48px_minmax(0,1fr)_72px_120px] gap-x-4 min-w-[720px]';

  return (
    <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
      {/* Header */}
      <div className="border-b border-gray-200 px-6 py-4 dark:border-gray-800">
        <h3 className="text-base font-semibold text-gray-900 dark:text-white">
          Running Sessions
          {sorted.length > 0 && (
            <span className="ml-2 inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              {sorted.length}
            </span>
          )}
        </h3>
      </div>

      {sorted.length === 0 ? (
        <p className="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
          No agents running
        </p>
      ) : (
        <>
          {/* Column headers */}
          <div className="overflow-x-auto border-b border-gray-100 dark:border-gray-800">
            <div className={`${colGrid} bg-gray-50 px-4 py-2 dark:bg-gray-900/50`}>
              {[
                '',
                'Identifier',
                'State',
                'Backend',
                'Turn',
                'Last Activity',
                'Elapsed',
                'Actions',
              ].map((h) => (
                <span
                  key={h}
                  className="text-xs font-medium tracking-wider text-gray-500 uppercase dark:text-gray-400"
                >
                  {h}
                </span>
              ))}
            </div>
          </div>

          {/* Rows — each row scrolls horizontally as a unit; accordion stays full-width */}
          {sorted.map((row) => (
            <div
              key={row.identifier}
              className="border-b border-gray-100 last:border-b-0 dark:border-gray-800"
            >
              <div className="overflow-x-auto">
                <div
                  onClick={() => {
                    toggle(row.identifier);
                  }}
                  className={`${colGrid} cursor-pointer items-center px-4 py-3 select-none hover:bg-gray-50 dark:hover:bg-gray-900/30`}
                >
                  <span
                    className={`text-xs text-gray-400 transition-transform duration-150 ${expandedId === row.identifier ? 'rotate-90' : ''}`}
                  >
                    ▶
                  </span>
                  <span className="truncate font-mono text-sm font-medium text-gray-900 dark:text-white">
                    {row.identifier}
                  </span>
                  <span>
                    <Badge color={stateBadgeColor(row.state)} size="sm">
                      {row.state}
                    </Badge>
                  </span>
                  <span>
                    {row.backend && (
                      <span
                        className={`inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium ${
                          row.backend === 'claude'
                            ? 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
                            : 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                        }`}
                      >
                        {row.backend}
                      </span>
                    )}
                  </span>
                  <span className="text-sm text-gray-600 dark:text-gray-400">{row.turnCount}</span>
                  <span
                    className="min-w-0 truncate text-sm text-gray-600 dark:text-gray-400"
                    title={row.lastEvent}
                  >
                    {row.lastEvent ? row.lastEvent.slice(0, 80) : '—'}
                  </span>
                  <span className="font-mono text-xs text-gray-600 dark:text-gray-400">
                    {fmtMs(row.elapsedMs)}
                  </span>
                  <span
                    onClick={(e) => {
                      e.stopPropagation();
                    }}
                    className="flex gap-1"
                  >
                    <button
                      onClick={() => {
                        cancelIssueMutation.mutate(row.identifier);
                      }}
                      className="rounded border border-amber-200 px-2 py-1 text-xs text-amber-600 hover:bg-amber-50 dark:border-amber-800 dark:text-amber-400 dark:hover:bg-amber-900/20"
                    >
                      ⏸ Pause
                    </button>
                    <button
                      onClick={() => {
                        terminateIssueMutation.mutate(row.identifier);
                      }}
                      className="rounded border border-red-200 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/20"
                    >
                      ✕ Cancel
                    </button>
                  </span>
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
        </>
      )}

      {/* Paused agents section — always rendered when there are paused issues */}
      {paused.length > 0 && (
        <>
          <div className="border-t border-red-100 bg-red-50/40 px-6 py-2 dark:border-red-900/30 dark:bg-red-900/10">
            <span className="text-xs font-semibold tracking-wider text-red-600 uppercase dark:text-red-400">
              ⏸ Paused ({paused.length})
            </span>
          </div>
          {paused.map((identifier) => {
            const prURL = pausedWithPR[identifier];
            return (
              <div
                key={identifier}
                className="flex items-center justify-between border-t border-gray-100 bg-red-50/20 px-4 py-3 dark:border-gray-800 dark:bg-red-900/5"
              >
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm font-medium text-red-700 dark:text-red-400">
                    {identifier}
                  </span>
                  {prURL && (
                    <a
                      href={prURL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center rounded bg-purple-100 px-1.5 py-0.5 text-xs font-medium text-purple-700 hover:underline dark:bg-purple-900/30 dark:text-purple-400"
                      onClick={(e) => {
                        e.stopPropagation();
                      }}
                    >
                      Open PR
                    </a>
                  )}
                </div>
                <div className="flex gap-2">
                  {prURL && (
                    <button
                      onClick={() => {
                        reanalyzeIssueMutation.mutate(identifier);
                      }}
                      className="rounded border border-purple-300 px-2 py-1 text-xs text-purple-700 hover:bg-purple-50 dark:border-purple-700 dark:text-purple-400 dark:hover:bg-purple-900/20"
                    >
                      🔄 Re-analyze
                    </button>
                  )}
                  <button
                    onClick={() => {
                      resumeIssueMutation.mutate(identifier);
                    }}
                    className="rounded border border-green-300 px-2 py-1 text-xs text-green-700 hover:bg-green-50 dark:border-green-700 dark:text-green-400 dark:hover:bg-green-900/20"
                  >
                    ▶ Resume
                  </button>
                  <button
                    onClick={() => {
                      terminateIssueMutation.mutate(identifier);
                    }}
                    className="rounded border border-red-200 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/20"
                  >
                    ✕ Discard
                  </button>
                </div>
              </div>
            );
          })}
        </>
      )}
    </div>
  );
}
