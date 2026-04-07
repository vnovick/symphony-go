import { useEffect, useRef } from 'react';

export type LogLevel = 'info' | 'action' | 'warn' | 'error' | 'subagent';

export interface LogEntry {
  ts: number;
  level: LogLevel;
  message: string;
}

interface TerminalProps {
  entries: LogEntry[];
  follow?: boolean;
  showTime?: boolean;
  className?: string;
}

const LEVEL_COLOR: Record<LogLevel, string> = {
  info: 'var(--text-secondary)',
  action: '#818cf8', // indigo
  warn: 'var(--warning)',
  error: 'var(--danger)',
  subagent: '#a855f7', // purple
};

function formatTime(ts: number) {
  return new Date(ts).toLocaleTimeString('en-GB', { hour12: false });
}

export function Terminal({ entries, follow = true, showTime = false, className }: TerminalProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (follow && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [follow, entries.length]);

  return (
    <div
      ref={scrollRef}
      className={['overflow-y-auto font-mono text-xs', className ?? ''].join(' ')}
      style={{ background: 'var(--bg)', color: 'var(--text-secondary)' }}
    >
      {entries.map((entry, i) => (
        <div key={`${String(entry.ts)}-${String(i)}`} className="flex gap-2 px-2 py-0.5 leading-5">
          {showTime && (
            <span data-testid={`terminal-time-${String(i)}`} className="shrink-0 opacity-40">
              {formatTime(entry.ts)}
            </span>
          )}
          <span
            data-level={entry.level}
            className="whitespace-pre-wrap"
            style={{ color: LEVEL_COLOR[entry.level] }}
          >
            {entry.message}
          </span>
        </div>
      ))}
    </div>
  );
}
