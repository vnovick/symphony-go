import { useSymphonyStore } from '../../store/symphonyStore';
import { Terminal } from '../ui/Terminal/Terminal';
import type { LogEntry, LogLevel } from '../ui/Terminal/Terminal';

const MAX_FEED_LINES = 20;

// Map log line text patterns to Terminal log levels
function lineToLevel(line: string): LogLevel {
  const l = line.toLowerCase();
  if (l.includes('error') || l.includes('fail')) return 'error';
  if (l.includes('warn') || l.includes('rate limit') || l.includes('retry')) return 'warn';
  if (l.includes('subagent') || l.includes('spawn')) return 'subagent';
  if (l.includes('pull request') || l.includes('pr opened') || l.includes('done') || l.includes('complete')) return 'action';
  return 'info';
}

export function NarrativeFeed() {
  const logs = useSymphonyStore((s) => s.logs);

  // Take the last 20 lines
  const recent = logs.slice(-MAX_FEED_LINES);

  const entries: LogEntry[] = recent.map((line, i) => ({
    ts: i,
    level: lineToLevel(line),
    message: line,
  }));

  return (
    <div
      data-testid="narrative-feed"
      className="rounded-[var(--radius-md)] overflow-hidden"
      style={{ border: '1px solid var(--line)', background: 'var(--panel)' }}
    >
      <div
        className="flex items-center justify-between px-4 py-2.5 border-b"
        style={{ borderColor: 'var(--line)' }}
      >
        <h3 className="text-sm font-semibold" style={{ color: 'var(--text)' }}>
          Recent Events
        </h3>
        <span className="text-xs font-mono" style={{ color: 'var(--muted)' }}>
          last {MAX_FEED_LINES}
        </span>
      </div>

      {entries.length === 0 ? (
        <div className="px-4 py-6 text-center text-sm" style={{ color: 'var(--muted)' }}>
          No events yet
        </div>
      ) : (
        <Terminal entries={entries} follow showTime={false} />
      )}
    </div>
  );
}
