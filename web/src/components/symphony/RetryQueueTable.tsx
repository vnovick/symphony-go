import { useState } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useCancelIssue } from '../../queries/issues';
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

  const handleCancel = (e: React.MouseEvent, identifier: string) => {
    e.stopPropagation(); // don't open the detail modal
    if (cancelling) return;
    setCancelling(identifier);
    cancelMutation.mutate(identifier, {
      onSettled: () => { setCancelling(null); },
    });
  };

  if (retrying.length === 0) return null;

  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--line)] bg-[var(--bg-elevated)]">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[var(--line)] px-[18px] py-[14px]">
        <div>
          <h2 className="flex items-center gap-2 text-sm font-semibold text-[var(--text)]">
            Retry Queue
            <span className="rounded-full bg-[var(--warning-soft)] px-1.5 py-0.5 text-[10px] font-bold text-[var(--warning)]">
              {retrying.length}
            </span>
          </h2>
          <p className="mt-1 text-xs text-[var(--text-secondary)]">
            Issues waiting to be re-dispatched after a failure
          </p>
        </div>
      </div>

      {/* Table */}
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-[var(--line)]">
            <th className="px-[18px] py-2 text-left text-[11px] font-semibold uppercase tracking-wide text-[var(--muted)]">Issue</th>
            <th className="px-3 py-2 text-left text-[11px] font-semibold uppercase tracking-wide text-[var(--muted)]">Attempt</th>
            <th className="px-3 py-2 text-left text-[11px] font-semibold uppercase tracking-wide text-[var(--muted)]">Retries in</th>
            <th className="px-[18px] py-2 text-left text-[11px] font-semibold uppercase tracking-wide text-[var(--muted)]">Last error</th>
            <th className="px-[18px] py-2" />
          </tr>
        </thead>
        <tbody>
          {retrying.map((row) => (
            <tr
              key={row.identifier}
              className="cursor-pointer border-b border-[var(--line)] last:border-b-0 hover:bg-[var(--bg-soft)] transition-colors"
              onClick={() => { setSelectedIdentifier(row.identifier); }}
            >
              <td className="px-[18px] py-3">
                <span className="font-mono text-xs font-semibold text-[var(--accent)]">
                  {row.identifier}
                </span>
              </td>
              <td className="px-3 py-3">
                <span className="rounded bg-[var(--warning-soft)] px-1.5 py-0.5 font-mono text-[11px] font-medium text-[var(--warning)]">
                  #{row.attempt}
                </span>
              </td>
              <td className="px-3 py-3 font-mono text-xs text-[var(--text-secondary)]">
                {fmtDueAt(row.dueAt)}
              </td>
              <td className="max-w-xs px-[18px] py-3 text-xs text-[var(--muted)]">
                {row.error ? (
                  <span className="line-clamp-1" title={row.error}>{row.error}</span>
                ) : (
                  <span className="italic">—</span>
                )}
              </td>
              <td className="px-[18px] py-3 text-right">
                <button
                  onClick={(e) => { handleCancel(e, row.identifier); }}
                  disabled={cancelling === row.identifier}
                  className="text-[11px] font-medium transition-opacity"
                  style={{
                    color: 'var(--danger)',
                    background: 'transparent',
                    border: 'none',
                    cursor: cancelling === row.identifier ? 'wait' : 'pointer',
                    opacity: cancelling === row.identifier ? 0.5 : 1,
                  }}
                >
                  {cancelling === row.identifier ? 'Cancelling…' : 'Cancel'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
