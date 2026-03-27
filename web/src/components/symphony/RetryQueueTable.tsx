import { useState, useCallback } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useCancelIssue } from '../../queries/issues';
import { SessionAccordion } from './SessionAccordion';
import type { RetryRow } from '../../types/schemas';

const EMPTY_RETRYING: RetryRow[] = [];

function fmtDueAt(dueAt: string): string {
  const diff = new Date(dueAt).getTime() - Date.now();
  const abs = Math.abs(diff);
  const secs = Math.round(abs / 1000);
  const mins = Math.round(abs / 60_000);
  const label = abs < 60_000 ? `${String(secs)}s` : `${String(mins)}m`;
  return diff > 0 ? `in ${label}` : `${label} ago`;
}

export default function RetryQueueTable() {
  const retrying = useSymphonyStore((s) => s.snapshot?.retrying ?? EMPTY_RETRYING);
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const cancelMutation = useCancelIssue();
  const [cancelling, setCancelling] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const handleCancel = (e: React.MouseEvent, identifier: string) => {
    e.stopPropagation();
    if (cancelling) return;
    setCancelling(identifier);
    cancelMutation.mutate(identifier, {
      onSettled: () => { setCancelling(null); },
    });
  };

  const toggle = useCallback((id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  }, []);

  if (retrying.length === 0) return null;

  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-theme-line bg-theme-bg-elevated">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-theme-line px-4 py-3">
        <div>
          <h2 className="flex items-center gap-2 text-sm font-semibold text-theme-text">
            Retry Queue
            <span className="rounded-full px-1.5 py-0.5 text-[10px] font-bold bg-theme-warning-soft text-theme-warning">
              {retrying.length}
            </span>
          </h2>
          <p className="mt-0.5 text-xs text-theme-text-secondary">
            Issues waiting to be re-dispatched after a failure
          </p>
        </div>
      </div>

      {/* Rows */}
      {retrying.map((row) => (
        <div key={row.identifier} className="border-b last:border-b-0 border-theme-line">
          <div
            role="button"
            tabIndex={0}
            onClick={() => { toggle(row.identifier); }}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') toggle(row.identifier); }}
            className="flex flex-wrap items-center gap-2 px-4 py-3 cursor-pointer transition-colors hover:bg-[var(--bg-soft)]"
          >
            {/* Chevron */}
            <span
              className="text-[10px] text-theme-muted transition-transform duration-200"
              style={{ transform: expandedId === row.identifier ? 'rotate(90deg)' : 'none' }}
            >
              ▶
            </span>

            {/* Identifier */}
            <span
              role="button"
              tabIndex={0}
              className="font-mono text-xs font-semibold text-theme-accent cursor-pointer hover:underline"
              onClick={(e) => { e.stopPropagation(); setSelectedIdentifier(row.identifier); }}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.stopPropagation(); setSelectedIdentifier(row.identifier); } }}
            >
              {row.identifier}
            </span>

            {/* Attempt badge */}
            <span className="rounded px-1.5 py-0.5 font-mono text-[10px] font-medium bg-theme-warning-soft text-theme-warning">
              #{row.attempt}
            </span>

            {/* Due at */}
            <span className="font-mono text-[11px] text-theme-text-secondary">
              {fmtDueAt(row.dueAt)}
            </span>

            {/* Error — truncated, hidden on mobile */}
            {row.error && (
              <span className="hidden sm:inline truncate text-xs text-theme-muted min-w-0 flex-1" title={row.error}>
                {row.error}
              </span>
            )}

            {/* Cancel button */}
            <button
              onClick={(e) => { handleCancel(e, row.identifier); }}
              disabled={cancelling === row.identifier}
              className="ml-auto text-[11px] font-medium text-theme-danger disabled:opacity-50"
            >
              {cancelling === row.identifier ? 'Cancelling…' : 'Cancel'}
            </button>
          </div>

          {/* Expandable accordion */}
          {expandedId === row.identifier && (
            <SessionAccordion
              identifier={row.identifier}
              workerHost={undefined}
              sessionId={undefined}
            />
          )}
        </div>
      ))}
    </div>
  );
}
