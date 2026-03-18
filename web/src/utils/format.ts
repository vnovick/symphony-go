/** Format elapsed milliseconds as "Xs" or "Xm YYs". */
export function fmtMs(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${String(s)}s`;
  return `${String(Math.floor(s / 60))}m ${String(s % 60).padStart(2, '0')}s`;
}

/** Tailwind classes for the orchestrator state indicator dot. */
export function orchDotClass(state: string): string {
  if (state === 'running') return 'bg-green-500 animate-pulse';
  if (state === 'retrying') return 'bg-yellow-400 animate-pulse';
  if (state === 'paused') return 'bg-red-400';
  return 'bg-gray-300 dark:bg-gray-600';
}

/** Tailwind classes for the priority indicator dot. Returns null when no priority. */
export function priorityDotClass(p: number | null | undefined): string | null {
  if (!p) return null;
  if (p === 1) return 'bg-red-500';
  if (p === 2) return 'bg-orange-400';
  if (p === 3) return 'bg-yellow-400';
  return 'bg-gray-400';
}

export type BadgeColor = 'primary' | 'success' | 'error' | 'warning' | 'info' | 'light' | 'dark';

/** Map a tracker state string to a Badge color variant. */
export function stateBadgeColor(state: string): BadgeColor {
  const s = state.toLowerCase();
  if (s.includes('progress')) return 'warning';
  if (s.includes('review') || s.includes('done')) return 'success';
  if (s.includes('todo')) return 'primary';
  return 'light';
}
