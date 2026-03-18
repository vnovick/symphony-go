import { useEffect, useRef, useState } from 'react';
import PageMeta from '../../components/common/PageMeta';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useIssues } from '../../queries/issues';
import { useIssueLogs } from '../../queries/logs';
import { orchDotClass } from '../../utils/format';
import { toTermLine } from '../../utils/logFormatting';

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Logs() {
  const { data: issues = [] } = useIssues();
  const snapshot = useSymphonyStore((s) => s.snapshot);

  const sortedIssues = [...issues].sort((a, b) => {
    const order = (s: string) =>
      s === 'running' ? 0 : s === 'retrying' ? 1 : s === 'paused' ? 2 : 3;
    const diff = order(a.orchestratorState) - order(b.orchestratorState);
    return diff !== 0 ? diff : a.identifier.localeCompare(b.identifier);
  });

  const [selectedId, setSelectedId] = useState<string>('');
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const followRef = useRef(true);
  const [isFollowing, setIsFollowing] = useState(true);

  useEffect(() => {
    if (!selectedId || !sortedIssues.find((i) => i.identifier === selectedId)) {
      const first = sortedIssues[0];
      // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
      if (first) {
        setSelectedId(first.identifier);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sortedIssues.map((i) => i.identifier).join(',')]);

  const isLive = !!(
    snapshot?.running.some((r) => r.identifier === selectedId) ||
    snapshot?.retrying.some((r) => r.identifier === selectedId)
  );
  const { data: entries = [], isLoading: loading } = useIssueLogs(selectedId, isLive);

  const onScroll = () => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const atBottom = scrollHeight - scrollTop - clientHeight < 40;
    followRef.current = atBottom;
    setIsFollowing(atBottom);
  };

  useEffect(() => {
    if (followRef.current) bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [entries]);

  const scrollToBottom = () => {
    followRef.current = true;
    setIsFollowing(true);
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
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

  const activeCount = issues.filter((i) => i.orchestratorState !== 'idle').length;
  const selectedIssue = issues.find((i) => i.identifier === selectedId);

  return (
    <>
      <PageMeta title="Simphony | Logs" description="Agent logs — all issues" />
      <div className="flex h-[calc(100vh-64px)]">
        {/* Sidebar */}
        <div
          className="flex w-52 flex-shrink-0 flex-col border-r border-gray-800"
          style={{ background: '#111318' }}
        >
          <div className="border-b border-gray-800 px-3 py-3">
            <p className="font-mono text-[10px] font-semibold tracking-widest text-[#4b5563] uppercase">
              Issues
            </p>
            <p className="mt-0.5 font-mono text-[10px] text-[#374151]">
              {activeCount} active · {issues.length} total
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
        <div className="flex flex-1 flex-col overflow-hidden" style={{ background: '#0d0f0e' }}>
          {/* Terminal title bar */}
          <div
            className="flex flex-shrink-0 items-center justify-between border-b border-[#1e2420] px-4 py-2"
            style={{ background: '#111814' }}
          >
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
                        {loading && <span className="ml-2 text-[#374151]">· refreshing…</span>}
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
                  onClick={handleExport}
                  className="font-mono text-[10px] text-[#4b5563] transition-colors hover:text-[#9ca3af]"
                >
                  ↓ export
                </button>
              )}
            </div>
          </div>

          {/* Jump-to-live */}
          {!isFollowing && (
            <div
              className="flex flex-shrink-0 justify-end border-b border-[#1e2420] px-4 py-1"
              style={{ background: '#111814' }}
            >
              <button
                onClick={scrollToBottom}
                className="font-mono text-[10px] text-[#4ade80] transition-colors hover:text-[#86efac]"
              >
                ▼ jump to live
              </button>
            </div>
          )}

          {/* Log output */}
          <div
            ref={containerRef}
            onScroll={onScroll}
            className="flex-1 space-y-px overflow-y-auto px-5 py-4 font-mono text-xs leading-[1.65]"
            style={{ background: '#0d0f0e' }}
          >
            {!selectedId && (
              <div className="flex justify-center gap-2 py-8 text-[#374151]">
                <span>$ select an issue from the sidebar</span>
              </div>
            )}

            {selectedId && entries.length === 0 && !loading && (
              <div className="flex justify-center gap-2 py-8">
                <span className="text-[#4b5563]">$ waiting for agent output…</span>
              </div>
            )}

            {entries.map((entry, i) => {
              const { prefix, prefixColor, text, textColor, time } = toTermLine(entry);
              return (
                <div key={i} className="flex min-h-[1.3em] items-baseline gap-2.5">
                  {time && (
                    <span className="w-14 shrink-0 text-right text-[10px] text-[#374151] tabular-nums">
                      {time}
                    </span>
                  )}
                  <span
                    className="w-3.5 shrink-0 text-right font-bold select-none"
                    style={{ color: prefixColor }}
                  >
                    {prefix}
                  </span>
                  <span className="break-all whitespace-pre-wrap" style={{ color: textColor }}>
                    {text}
                  </span>
                </div>
              );
            })}

            {/* Blinking cursor when active */}
            {isLive && (
              <div className="mt-1 flex gap-2.5">
                {entries.length > 0 && entries[0].time && <span className="w-14 shrink-0" />}
                <span className="w-3.5 shrink-0" />
                <span
                  className="inline-block h-[13px] w-[7px]"
                  style={{
                    background: '#4ade80',
                    animation: 'blink 1.1s step-end infinite',
                  }}
                />
              </div>
            )}

            <div ref={bottomRef} />
          </div>
        </div>
      </div>
    </>
  );
}
