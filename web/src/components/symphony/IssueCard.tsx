import { fmtMs, EMPTY_PROFILE_LABEL } from '../../utils/format';
import type { TrackerIssue, ProfileDef } from '../../types/schemas';
import { Card } from '../ui/Card/Card';

interface CardProps {
  issue: TrackerIssue;
  isDragging?: boolean;
  onSelect: (id: string) => void;
  availableProfiles?: string[];
  profileDefs?: Record<string, ProfileDef>;
  runningBackend?: string;
  onProfileChange?: (identifier: string, profile: string) => void;
  onDispatch?: (identifier: string) => void;
}

// Resolve the actual runner backend for display:
// 1. Prefer the live backend from RunningRow (most accurate for active sessions)
// 2. Fall back to the declared backend in the profile definition
// 3. Last resort: detect "codex" in the profile name
function resolveBackend(
  profile: string | undefined,
  profileDefs: Record<string, ProfileDef> | undefined,
  runningBackend: string | undefined,
): 'claude' | 'codex' {
  const src = runningBackend ?? (profile ? profileDefs?.[profile]?.backend : undefined) ?? profile ?? '';
  return /codex/i.test(src) ? 'codex' : 'claude';
}

export default function IssueCard({
  issue,
  isDragging,
  onSelect,
  availableProfiles,
  profileDefs,
  runningBackend,
  onProfileChange,
  onDispatch,
}: CardProps) {
  const showProfileSelector = availableProfiles && availableProfiles.length > 0 && onProfileChange;
  const isRunning = issue.orchestratorState === 'running';
  const backend = resolveBackend(issue.agentProfile, profileDefs, runningBackend);

  return (
    <Card
      variant={isDragging ? 'elevated' : 'default'}
      padding="md"
      className={`issue-card group min-h-[130px] cursor-pointer select-none ${isDragging ? 'rotate-1 opacity-90' : ''}`}
      onClick={() => { onSelect(issue.identifier); }}
    >
      {/* Top row: identifier + backend badge + state badge | profile selector (top-right) */}
      <div className="mb-2 flex items-start gap-1.5">
        {/* Left: identifier + badges */}
        <div className="flex flex-1 flex-wrap items-center gap-1.5 min-w-0">
          {issue.url ? (
            <a
              href={issue.url}
              target="_blank"
              rel="noopener noreferrer"
              className="rounded px-1.5 py-0.5 font-mono text-[10px] font-semibold hover:underline bg-[var(--bg-soft)] text-[var(--accent)]"
              onClick={(e) => { e.stopPropagation(); }}
            >
              {issue.identifier}
            </a>
          ) : (
            <span className="rounded px-1.5 py-0.5 font-mono text-[10px] font-semibold bg-[var(--bg-soft)] text-[var(--text-secondary)]">
              {issue.identifier}
            </span>
          )}
          <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium uppercase ${
            backend === 'codex'
              ? 'bg-[rgba(16,185,129,0.12)] text-[#34d399]'
              : 'bg-[var(--accent-soft)] text-[var(--accent-strong)]'
          }`}>
            {backend === 'codex' ? 'CODEX' : 'CLAUDE'}
          </span>
          {issue.orchestratorState !== 'idle' && (
            <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium capitalize ${
              issue.orchestratorState === 'running'
                ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
                : 'bg-[var(--warning-soft)] text-[var(--warning)]'
            }`}>
              {issue.orchestratorState}
            </span>
          )}
        </div>

        {/* Right: profile selector — always visible in top-right corner; disabled while running */}
        {showProfileSelector && (
          <div
            className="flex-shrink-0"
            onClick={(e) => { e.stopPropagation(); }}
            title={isRunning ? 'Cannot change agent while In Progress' : undefined}
          >
            <select
              value={issue.agentProfile ?? ''}
              onChange={(e) => { onProfileChange(issue.identifier, e.target.value); }}
              disabled={isRunning}
              className={`min-w-[90px] max-w-[140px] rounded-[var(--radius-sm)] border border-[var(--line)] bg-[var(--panel-strong)] text-[var(--text-secondary)] px-2 py-1 text-xs font-medium focus:outline-none disabled:opacity-40 ${isRunning ? 'cursor-not-allowed' : 'cursor-pointer'}`}
            >
              <option value="">{EMPTY_PROFILE_LABEL}</option>
              {availableProfiles.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
          </div>
        )}

        {/* Queue icon — compact ▶ with tooltip, visible on hover */}
        {onDispatch && (
          <button
            title="Send to queue"
            onClick={(e) => { e.stopPropagation(); onDispatch(issue.identifier); }}
            className="flex-shrink-0 rounded px-1.5 py-0.5 text-[10px] opacity-0 transition-opacity group-hover:opacity-100 text-[var(--accent)] bg-[var(--accent-soft)]"
          >
            ▶
          </button>
        )}
      </div>

      {/* Title — 2-line clamp, larger text */}
      <p className="line-clamp-2 text-sm font-medium leading-relaxed text-[var(--text-secondary)]">
        {issue.title}
      </p>

      {/* Elapsed time */}
      {(issue.elapsedMs ?? 0) > 0 && (
        <p className="mt-1.5 font-mono text-[11px] text-[var(--muted)]">
          ⏱ {fmtMs(issue.elapsedMs ?? 0)}
        </p>
      )}
    </Card>
  );
}
