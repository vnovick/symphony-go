import { useItervoxStore } from '../../store/itervoxStore';
import { Terminal } from '../ui/Terminal/Terminal';
import type { LogEntry, LogLevel } from '../ui/Terminal/Terminal';

const MAX_FEED_LINES = 20;

// Map log line text patterns to Terminal log levels
function lineToLevel(line: string): LogLevel {
  const l = line.toLowerCase();
  if (l.includes('error') || l.includes('fail')) return 'error';
  if (l.includes('warn') || l.includes('rate limit') || l.includes('retry')) return 'warn';
  if (l.includes('subagent') || l.includes('spawn')) return 'subagent';
  if (
    l.includes('pull request') ||
    l.includes('pr opened') ||
    l.includes('done') ||
    l.includes('complete')
  )
    return 'action';
  return 'info';
}

export function NarrativeFeed() {
  const logs = useItervoxStore((s) => s.logs);

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
      className="border-theme-line bg-theme-panel overflow-hidden rounded-[var(--radius-md)] border"
    >
      <div className="border-theme-line flex items-center justify-between border-b px-4 py-2.5">
        <h3 className="text-theme-text text-sm font-semibold">Recent Events</h3>
        <span className="text-theme-muted font-mono text-xs">last {MAX_FEED_LINES}</span>
      </div>

      {entries.length === 0 ? (
        <div className="text-theme-muted px-4 py-6 text-center text-sm">No events yet</div>
      ) : (
        <Terminal entries={entries} follow showTime={false} />
      )}
    </div>
  );
}
