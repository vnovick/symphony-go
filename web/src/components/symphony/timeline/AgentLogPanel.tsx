import { useEffect, useRef, useState } from 'react';
import { useSymphonyStore } from '../../../store/symphonyStore';
import { useIssueLogs } from '../../../queries/logs';
import { toTermLine } from '../../../utils/logFormatting';
import type { IssueLogEntry } from '../../../types/schemas';

interface AgentLogPanelProps {
  identifier: string;
  logSlice?: IssueLogEntry[];
}

export function AgentLogPanel({ identifier, logSlice }: AgentLogPanelProps) {
  const isLive = useSymphonyStore(
    (s) =>
      !!(
        s.snapshot?.running.some((r) => r.identifier === identifier) ||
        s.snapshot?.retrying.some((r) => r.identifier === identifier)
      ),
  );
  const { data: liveEntries } = useIssueLogs(identifier, isLive);
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const followRef = useRef(true);
  const [isFollowing, setIsFollowing] = useState(true);

  // logSlice is provided when viewing a specific subagent's logs; fall back to live entries
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  const entries = logSlice ?? liveEntries ?? [];

  const onScroll = () => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    followRef.current = scrollHeight - scrollTop - clientHeight < 40;
    setIsFollowing(followRef.current);
  };

  useEffect(() => {
    if (followRef.current) bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [entries]);

  const scrollToBottom = () => {
    followRef.current = true;
    setIsFollowing(true);
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {!isFollowing && (
        <div
          className="flex flex-shrink-0 justify-end border-b px-3 py-1 border-theme-line bg-theme-panel"
        >
          <button
            onClick={scrollToBottom}
            className="text-[10px] font-medium transition-colors hover:opacity-80 text-theme-accent"
          >
            ▼ Jump to live
          </button>
        </div>
      )}
      <div
        ref={containerRef}
        onScroll={onScroll}
        className="flex-1 overflow-y-auto p-3 font-mono text-[12px] leading-[1.6] bg-theme-panel-dark"
      >
        {entries.length === 0 ? (
          <p className="italic text-theme-muted">
            No logs yet for {identifier}.
          </p>
        ) : (
          entries.map((entry) => {
            const line = toTermLine(entry);
            return (
              <div
                key={`${entry.time ?? ''}-${entry.event}-${entry.message.slice(0, 24)}`}
                className="mb-0.5 flex gap-2"
              >
                {line.time && (
                  <span className="w-[50px] shrink-0 text-theme-muted">
                    {line.time}
                  </span>
                )}
                <span className="shrink-0" style={{ color: line.prefixColor }}>{line.prefix}</span>
                <span className="break-all whitespace-pre-wrap" style={{ color: line.textColor, wordBreak: 'break-word' }}>
                  {line.text}
                </span>
              </div>
            );
          })
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
