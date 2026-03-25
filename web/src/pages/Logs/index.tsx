import { useEffect, useMemo, useState } from 'react';
import PageMeta from '../../components/common/PageMeta';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useIssues, useClearIssueLogs } from '../../queries/issues';
import { useIssueLogs, useLogIdentifiers } from '../../queries/logs';
import { orchDotClass } from '../../utils/format';
import { Terminal } from '../../components/ui/Terminal/Terminal';
import type { LogEntry, LogLevel } from '../../components/ui/Terminal/Terminal';
import type { IssueLogEntry } from '../../types/symphony';

// ─── Helpers ──────────────────────────────────────────────────────────────────

const FILTER_CHIPS = ['text', 'action', 'subagent', 'warn', 'error'] as const;
type FilterChip = (typeof FILTER_CHIPS)[number];

function entryToLevel(entry: IssueLogEntry): LogLevel {
  if (entry.event === 'action') return 'action';
  if (entry.event === 'subagent') return 'subagent';
  if (entry.event === 'warn') return 'warn';
  if (entry.event === 'error' || entry.level === 'ERROR') return 'error';
  return 'info';
}

function entryToLogEntry(entry: IssueLogEntry, idx: number): LogEntry {
  const text = entry.tool ? `${entry.tool}  ${entry.message}` : entry.message;
  return { ts: idx, level: entryToLevel(entry), message: text };
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Logs() {
  const { data: issues = [] } = useIssues();
  const logIdentifiers = useLogIdentifiers();
  const snapshot = useSymphonyStore((s) => s.snapshot);

  // Build a lookup map from issues for orchestratorState enrichment
  const issueMap = useMemo(
    () => new Map(issues.map((i) => [i.identifier, i])),
    [issues],
  );

  // Sidebar uses log identifiers as source of truth, enriched with issue metadata
  const sortedIssues = useMemo(() => {
    const order = (state: string) =>
      state === 'running' ? 0 : state === 'retrying' ? 1 : state === 'paused' ? 2 : 3;
    return [...logIdentifiers]
      .map((id) => ({
        identifier: id,
        orchestratorState: issueMap.get(id)?.orchestratorState ?? 'idle',
        branchName: issueMap.get(id)?.branchName,
        agentProfile: issueMap.get(id)?.agentProfile,
      }))
      .sort((a, b) => {
        const diff = order(a.orchestratorState) - order(b.orchestratorState);
        return diff !== 0 ? diff : a.identifier.localeCompare(b.identifier);
      });
  }, [logIdentifiers, issueMap]);

  const [selectedId, setSelectedId] = useState<string>('');
  const [activeChips, setActiveChips] = useState<Set<FilterChip>>(new Set(FILTER_CHIPS));

  useEffect(() => {
    if (!selectedId || !sortedIssues.find((i) => i.identifier === selectedId)) {
      const first = sortedIssues[0];
      // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
      if (first) setSelectedId(first.identifier);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- selectedId intentionally omitted: effect only auto-selects first issue when the issue list changes, not on every selectedId transition
  }, [sortedIssues]);

  const isLive = !!(
    snapshot?.running.some((r) => r.identifier === selectedId) ||
    snapshot?.retrying.some((r) => r.identifier === selectedId)
  );
  const { data: entries = [], isLoading: loading } = useIssueLogs(selectedId, isLive);

  const clearLogsMutation = useClearIssueLogs();
  const handleClearLogs = () => {
    if (selectedId) clearLogsMutation.mutate(selectedId);
  };

  const handleExport = () => {
    const text = entries.map((e) => `${e.time ? `[${e.time}] ` : ''}${e.message}`).join('\n');
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `logs-${selectedId}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const activeCount = sortedIssues.filter((i) => i.orchestratorState !== 'idle').length;
  const selectedIssue = sortedIssues.find((i) => i.identifier === selectedId);
  const runningRow = snapshot?.running.find((r) => r.identifier === selectedId);

  const toggleChip = (chip: FilterChip) => {
    setActiveChips((prev) => {
      const next = new Set(prev);
      if (next.has(chip)) next.delete(chip);
      else next.add(chip);
      return next;
    });
  };

  // Filter entries by active chips, then map to Terminal LogEntry
  const filteredEntries = useMemo(
    () =>
      entries.filter(
        (e) =>
          !FILTER_CHIPS.includes(e.event as FilterChip) ||
          activeChips.has(e.event as FilterChip),
      ),
    [entries, activeChips],
  );

  const logEntries = useMemo(() => filteredEntries.map(entryToLogEntry), [filteredEntries]);

  return (
    <>
      <PageMeta title="Symphony | Logs" description="Agent logs — all issues" />
      <div className="flex h-[calc(100vh-64px)]">
        {/* Sidebar */}
        <div className="flex w-52 flex-shrink-0 flex-col border-r border-gray-800 bg-terminal-base">
          <div className="border-b border-gray-800 px-3 py-3">
            <p className="font-mono text-[10px] font-semibold tracking-widest text-[#4b5563] uppercase">
              Issues
            </p>
            <p className="mt-0.5 font-mono text-[10px] text-[#374151]">
              {activeCount} active · {logIdentifiers.length} total
            </p>
          </div>
          <div className="flex-1 overflow-y-auto">
            {sortedIssues.length === 0 && (
              <p className="px-3 py-4 font-mono text-xs text-[#374151]">No issues loaded</p>
            )}
            {sortedIssues.map((issue) => (
              <button
                key={issue.identifier}
                onClick={() => {
                  setSelectedId(issue.identifier);
                }}
                className={`flex w-full items-center gap-2 border-b border-gray-900 px-3 py-2 text-left font-mono text-xs transition-colors ${
                  selectedId === issue.identifier ? 'bg-[#1a1f2e]' : 'hover:bg-[#161a22]'
                }`}
              >
                <span
                  className={`h-1.5 w-1.5 flex-shrink-0 rounded-full ${orchDotClass(issue.orchestratorState)}`}
                />
                <span
                  className={`truncate ${
                    selectedId === issue.identifier ? 'text-[#4ade80]' : 'text-[#9ca3af]'
                  }`}
                >
                  {issue.identifier}
                </span>
              </button>
            ))}
          </div>
        </div>

        {/* Terminal panel */}
        <div className="flex flex-1 flex-col overflow-hidden bg-terminal-void">
          {/* Terminal title bar */}
          <div className="flex flex-shrink-0 items-center justify-between border-b border-[#1e2420] bg-terminal-header px-4 py-2">
            <div className="flex items-center gap-3">
              {/* Traffic light dots */}
              <span className="flex gap-1.5">
                <span className="h-3 w-3 rounded-full bg-[#ff5f57]" />
                <span className="h-3 w-3 rounded-full bg-[#febc2e]" />
                <span className="h-3 w-3 rounded-full bg-[#28c840]" />
              </span>
              <span className="font-mono text-xs text-[#4b5563]">
                {selectedId ? (
                  <>
                    <span className="text-[#4ade80]">{selectedId}</span>
                    {selectedIssue && (
                      <span className="ml-2 text-[#374151]">
                        — {isLive ? selectedIssue.orchestratorState : 'idle'}
                        {loading && <span className="ml-2">· refreshing…</span>}
                      </span>
                    )}
                  </>
                ) : (
                  <span className="text-[#374151]">select an issue</span>
                )}
              </span>
            </div>
            <div className="flex items-center gap-3">
              {entries.length > 0 && (
                <span className="font-mono text-[10px] text-[#374151]">{entries.length} lines</span>
              )}
              {entries.length > 0 && (
                <button
                  onClick={handleClearLogs}
                  className="font-mono text-[10px] text-[#4b5563] transition-colors hover:text-[#ef4444]"
                >
                  ✕ clear
                </button>
              )}
              {entries.length > 0 && (
                <button
                  onClick={handleExport}
                  className="font-mono text-[10px] text-[#4b5563] transition-colors hover:text-[#9ca3af]"
                >
                  ↓ export
                </button>
              )}
            </div>
          </div>

          {/* Contextual strip (5.1): state | host | session | branch | profile */}
          {selectedId && (
            <div
              data-testid="logs-context-strip"
              className="flex flex-shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-b border-[#1e2420] bg-terminal-header px-4 py-1.5"
            >
              <span className="font-mono text-[10px] text-[#4b5563]">
                state{' '}
                <span className="text-[#9ca3af]">{selectedIssue?.orchestratorState ?? 'idle'}</span>
              </span>
              {runningRow?.workerHost && (
                <span className="font-mono text-[10px] text-[#4b5563]">
                  host <span className="text-[#9ca3af]">{runningRow.workerHost}</span>
                </span>
              )}
              {runningRow?.sessionId && (
                <span className="font-mono text-[10px] text-[#4b5563]">
                  session{' '}
                  <span className="text-[#9ca3af]">{runningRow.sessionId.slice(0, 8)}</span>
                </span>
              )}
              {selectedIssue?.branchName && (
                <span className="font-mono text-[10px] text-[#4b5563]">
                  branch <span className="text-[#4ade80]">{selectedIssue.branchName}</span>
                </span>
              )}
              {selectedIssue?.agentProfile && (
                <span className="font-mono text-[10px] text-[#4b5563]">
                  profile <span className="text-[#9ca3af]">{selectedIssue.agentProfile}</span>
                </span>
              )}
            </div>
          )}

          {/* Quick filter chips (5.2) */}
          {selectedId && (
            <div
              data-testid="logs-filter-chips"
              className="flex flex-shrink-0 items-center gap-2 border-b border-[#1e2420] bg-terminal-header px-4 py-1.5"
            >
              {FILTER_CHIPS.map((chip) => (
                <button
                  key={chip}
                  data-testid={`chip-${chip}`}
                  onClick={() => toggleChip(chip)}
                  className={`rounded px-2 py-0.5 font-mono text-[10px] transition-colors ${
                    activeChips.has(chip)
                      ? 'bg-[#1a1f2e] text-[#9ca3af]'
                      : 'text-[#374151] line-through'
                  }`}
                >
                  {chip}
                </button>
              ))}
              <span className="ml-auto font-mono text-[10px] text-[#374151]">
                {filteredEntries.length} / {entries.length}
              </span>
            </div>
          )}

          {/* Log output via Terminal (5.3) */}
          <div className="flex flex-1 flex-col overflow-hidden">
            {!selectedId ? (
              <div className="flex flex-1 items-center justify-center font-mono text-xs text-[#374151]">
                $ select an issue from the sidebar
              </div>
            ) : logEntries.length === 0 && !loading ? (
              <div className="flex flex-1 items-center justify-center font-mono text-xs text-[#4b5563]">
                $ waiting for agent output…
              </div>
            ) : (
              <Terminal entries={logEntries} follow showTime={false} />
            )}
          </div>
        </div>
      </div>
    </>
  );
}
