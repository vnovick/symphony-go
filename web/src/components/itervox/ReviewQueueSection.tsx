import { useMemo } from 'react';
import { useItervoxStore } from '../../store/itervoxStore';
import { useShallow } from 'zustand/react/shallow';
import { useIssues, useTriggerAIReview } from '../../queries/issues';
import { fmtMs } from '../../utils/format';
import { EMPTY_RUNNING, EMPTY_HISTORY } from '../../utils/constants';

/**
 * ReviewQueueSection shows a dashboard section with:
 * - Issues awaiting review (in completionState, no active reviewer)
 * - Issues currently being reviewed (running with kind="reviewer")
 * - Recent review completions from history
 *
 * Only rendered when reviewerProfile is configured.
 */
export function ReviewQueueSection() {
  const { reviewerProfile, completionState, running, history } = useItervoxStore(
    useShallow((s) => ({
      reviewerProfile: s.snapshot?.reviewerProfile ?? '',
      completionState: s.snapshot?.completionState ?? '',
      running: s.snapshot?.running ?? EMPTY_RUNNING,
      history: s.snapshot?.history ?? EMPTY_HISTORY,
    })),
  );

  const { data: issues = [] } = useIssues();
  const triggerReview = useTriggerAIReview();

  // Issues in completionState awaiting review
  const awaitingReview = useMemo(() => {
    if (!completionState) return [];
    const reviewingIdentifiers = new Set(
      running.filter((r) => r.kind === 'reviewer').map((r) => r.identifier),
    );
    return issues.filter(
      (i) =>
        i.state.toLowerCase() === completionState.toLowerCase() &&
        !reviewingIdentifiers.has(i.identifier),
    );
  }, [issues, completionState, running]);

  // Currently being reviewed
  const reviewing = useMemo(() => running.filter((r) => r.kind === 'reviewer'), [running]);

  // Recent review completions (last 5)
  const recentReviews = useMemo(
    () => history.filter((h) => h.kind === 'reviewer').slice(0, 5),
    [history],
  );

  // Don't render if no reviewer profile
  if (!reviewerProfile) return null;

  const totalItems = awaitingReview.length + reviewing.length + recentReviews.length;

  return (
    <div className="border-theme-line bg-theme-bg-elevated shadow-theme-sm overflow-hidden rounded-[var(--radius-lg)] border">
      {/* Header */}
      <div className="border-theme-line flex items-center justify-between border-b px-4 py-3">
        <div className="flex items-center gap-2">
          <h2 className="text-theme-text text-sm font-semibold tracking-tight">Review Queue</h2>
          <span className="bg-theme-bg-soft text-theme-text-secondary rounded-full px-1.5 py-0.5 text-[10px] font-bold">
            {totalItems}
          </span>
        </div>
        <span className="bg-theme-accent-soft text-theme-accent-strong rounded-full px-2 py-0.5 text-[10px] font-medium">
          {reviewerProfile}
        </span>
      </div>

      {totalItems === 0 ? (
        <div className="text-theme-muted px-4 py-8 text-center text-sm">
          No issues in review queue
        </div>
      ) : (
        <div className="divide-theme-line divide-y">
          {/* Awaiting review */}
          {awaitingReview.map((issue) => (
            <div
              key={issue.identifier}
              className="hover:bg-theme-bg-soft flex items-center gap-3 px-4 py-2.5 transition-colors"
            >
              <span className="text-xs text-amber-400">⏳</span>
              <span className="text-theme-accent font-mono text-xs font-semibold">
                {issue.identifier}
              </span>
              <span className="text-theme-text-secondary flex-1 truncate text-xs">
                {issue.title}
              </span>
              <button
                onClick={() => {
                  triggerReview.mutate(issue.identifier);
                }}
                disabled={triggerReview.isPending}
                className="border-theme-line text-theme-accent flex-shrink-0 rounded-[var(--radius-sm)] border px-2.5 py-1 text-[10px] font-medium transition-colors hover:opacity-80"
              >
                {triggerReview.isPending ? '…' : '▶ Review'}
              </button>
            </div>
          ))}

          {/* Currently reviewing */}
          {reviewing.map((row) => (
            <div
              key={row.identifier}
              className="bg-theme-success-soft/30 flex items-center gap-3 px-4 py-2.5"
            >
              <span className="text-theme-success text-xs">🔍</span>
              <span className="text-theme-accent font-mono text-xs font-semibold">
                {row.identifier}
              </span>
              <span className="text-theme-text-secondary flex-1 text-xs">
                Reviewing…
                {row.turnCount > 0 && ` (turn ${String(row.turnCount)})`}
              </span>
              <span className="text-theme-muted font-mono text-[10px]">{fmtMs(row.elapsedMs)}</span>
            </div>
          ))}

          {/* Recent completions */}
          {recentReviews.map((row) => (
            <div
              key={`${row.identifier}-${String(row.sessionId)}`}
              className="flex items-center gap-3 px-4 py-2.5 opacity-70"
            >
              <span className="text-xs">{row.status === 'succeeded' ? '✓' : '✗'}</span>
              <span className="text-theme-text-secondary font-mono text-xs font-semibold">
                {row.identifier}
              </span>
              <span className="text-theme-muted flex-1 text-xs">Review {row.status}</span>
              <span className="text-theme-muted font-mono text-[10px]">{fmtMs(row.elapsedMs)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
