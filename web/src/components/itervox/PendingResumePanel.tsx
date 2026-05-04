import { useEffect, useMemo, useRef } from 'react';
import { useLocation } from 'react-router';
import { useShallow } from 'zustand/react/shallow';
import { useItervoxStore } from '../../store/itervoxStore';

// Dashboard panel that makes `pending_input_resume` issues visible at a
// glance — previously only surfaced as a count pill in the app header and
// a "Resuming" badge on the per-issue card, which is easy to miss. Mirrors
// the shape of RetryQueueTable (collapsible rows, accent header, hides
// when empty).
export function PendingResumePanel({ onSelect }: { onSelect?: (identifier: string) => void }) {
  const { inputRequired, setSelectedIdentifier } = useItervoxStore(
    useShallow((s) => ({
      inputRequired: s.snapshot?.inputRequired ?? EMPTY_INPUT_REQUIRED,
      setSelectedIdentifier: s.setSelectedIdentifier,
    })),
  );

  const resuming = useMemo(
    () => inputRequired.filter((entry) => entry.state === 'pending_input_resume'),
    [inputRequired],
  );

  // React Router v7 doesn't auto-scroll to hash targets. When the AppHeader's
  // "N resuming" pill links to "/#pending-resume", this effect scrolls the
  // panel into view once both the route and the panel are mounted.
  const { hash } = useLocation();
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (hash === '#pending-resume' && ref.current) {
      ref.current.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [hash, resuming.length]);

  if (resuming.length === 0) return null;

  const handleClick = (identifier: string) => {
    if (onSelect) onSelect(identifier);
    else setSelectedIdentifier(identifier);
  };

  return (
    <div
      id="pending-resume"
      ref={ref}
      className="border-theme-line bg-theme-bg-elevated scroll-mt-20 overflow-hidden rounded-[var(--radius-lg)] border"
    >
      <div className="border-theme-line flex items-center justify-between border-b px-4 py-3">
        <div>
          <h2 className="text-theme-text flex items-center gap-2 text-sm font-semibold">
            Resuming
            <span className="rounded-full bg-orange-500/15 px-1.5 py-0.5 text-[10px] font-bold text-orange-400">
              {resuming.length}
            </span>
          </h2>
          <p className="text-theme-text-secondary mt-0.5 text-xs">
            Issues whose human reply has been received — the agent is resuming shortly.
          </p>
        </div>
      </div>

      {resuming.map((row) => (
        <button
          key={row.identifier}
          type="button"
          onClick={() => {
            handleClick(row.identifier);
          }}
          className="border-theme-line flex w-full flex-wrap items-center gap-2 border-b px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-[var(--bg-soft)]"
        >
          <span className="text-theme-text font-mono text-xs font-semibold">{row.identifier}</span>
          {row.profile && (
            <span className="bg-theme-bg-soft text-theme-text-secondary rounded px-1.5 py-0.5 text-[10px] font-medium">
              {row.profile}
            </span>
          )}
          {row.backend && (
            <span className="rounded bg-[var(--accent-soft)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--accent-strong)]">
              {row.backend}
            </span>
          )}
          <span className="text-theme-muted truncate text-xs" title={row.context}>
            {truncateContext(row.context)}
          </span>
        </button>
      ))}
    </div>
  );
}

const EMPTY_INPUT_REQUIRED: [] = [];

function truncateContext(s: string, max = 120): string {
  const clean = s.replace(/\s+/g, ' ').trim();
  if (clean.length <= max) return clean;
  return clean.slice(0, max) + '…';
}
