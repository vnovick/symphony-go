import { useMemo } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
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
  const { reviewerProfile, completionState, running, history } = useSymphonyStore(
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
      (i) => i.state.toLowerCase() === completionState.toLowerCase() && !reviewingIdentifiers.has(i.identifier),
    );
  }, [issues, completionState, running]);

  // Currently being reviewed
  const reviewing = useMemo(
    () => running.filter((r) => r.kind === 'reviewer'),
    [running],
  );

  // Recent review completions (last 5)
  const recentReviews = useMemo(
    () => history
      .filter((h) => h.kind === 'reviewer')
      .slice(0, 5),
    [history],
  );

  // Don't render if no reviewer profile
  if (!reviewerProfile) return null;

  const totalItems = awaitingReview.length + reviewing.length + recentReviews.length;

  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-theme-line bg-theme-bg-elevated shadow-theme-sm">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-3 border-theme-line">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-semibold tracking-tight text-theme-text">
            Review Queue
          </h2>
          <span className="rounded-full px-1.5 py-0.5 text-[10px] font-bold bg-theme-bg-soft text-theme-text-secondary">
            {totalItems}
          </span>
        </div>
        <span className="rounded-full px-2 py-0.5 text-[10px] font-medium bg-theme-accent-soft text-theme-accent-strong">
          {reviewerProfile}
        </span>
      </div>

      {totalItems === 0 ? (
        <div className="px-4 py-8 text-center text-sm text-theme-muted">
          No issues in review queue
        </div>
      ) : (
        <div className="divide-y divide-theme-line">
          {/* Awaiting review */}
          {awaitingReview.map((issue) => (
            <div
              key={issue.identifier}
              className="flex items-center gap-3 px-4 py-2.5 hover:bg-theme-bg-soft transition-colors"
            >
              <span className="text-amber-400 text-xs">⏳</span>
              <span className="font-mono text-xs font-semibold text-theme-accent">
                {issue.identifier}
              </span>
              <span className="truncate text-xs text-theme-text-secondary flex-1">
                {issue.title}
              </span>
              <button
                onClick={() => { triggerReview.mutate(issue.identifier); }}
                disabled={triggerReview.isPending}
                className="flex-shrink-0 rounded-[var(--radius-sm)] border px-2.5 py-1 text-[10px] font-medium transition-colors hover:opacity-80 border-theme-line text-theme-accent"
              >
                {triggerReview.isPending ? '…' : '▶ Review'}
              </button>
            </div>
          ))}

          {/* Currently reviewing */}
          {reviewing.map((row) => (
            <div
              key={row.identifier}
              className="flex items-center gap-3 px-4 py-2.5 bg-theme-success-soft/30"
            >
              <span className="text-theme-success text-xs">🔍</span>
              <span className="font-mono text-xs font-semibold text-theme-accent">
                {row.identifier}
              </span>
              <span className="text-xs text-theme-text-secondary flex-1">
                Reviewing…
                {row.turnCount > 0 && ` (turn ${row.turnCount})`}
              </span>
              <span className="font-mono text-[10px] text-theme-muted">
                {fmtMs(row.elapsedMs)}
              </span>
            </div>
          ))}

          {/* Recent completions */}
          {recentReviews.map((row) => (
            <div
              key={`${row.identifier}-${row.sessionId}`}
              className="flex items-center gap-3 px-4 py-2.5 opacity-70"
            >
              <span className="text-xs">
                {row.status === 'succeeded' ? '✓' : '✗'}
              </span>
              <span className="font-mono text-xs font-semibold text-theme-text-secondary">
                {row.identifier}
              </span>
              <span className="text-xs text-theme-muted flex-1">
                Review {row.status}
              </span>
              <span className="font-mono text-[10px] text-theme-muted">
                {fmtMs(row.elapsedMs)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
