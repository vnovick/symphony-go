import { memo } from 'react';
import { fmtMs, EMPTY_PROFILE_LABEL } from '../../utils/format';
import type { TrackerIssue, ProfileDef } from '../../types/schemas';

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

function resolveBackend(
  profile: string | undefined,
  profileDefs: Record<string, ProfileDef> | undefined,
  runningBackend: string | undefined,
): 'claude' | 'codex' {
  const src = runningBackend ?? (profile ? profileDefs?.[profile]?.backend : undefined) ?? profile ?? '';
  return /codex/i.test(src) ? 'codex' : 'claude';
}

// Status dot color per orchestrator state
function statusDotClass(state: string): string {
  switch (state) {
    case 'running': return 'bg-theme-success';
    case 'paused': return 'bg-theme-warning';
    case 'retrying': return 'bg-theme-danger';
    default: return 'bg-theme-muted';
  }
}

export default memo(function IssueCard({
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
  const hasActivity = issue.orchestratorState !== 'idle';

  return (
    <div
      className={`issue-card group cursor-pointer select-none rounded-lg border border-theme-line bg-theme-bg-elevated p-3 transition-all ${
        isDragging ? 'rotate-1 opacity-90 shadow-lg' : ''
      }`}
      onClick={() => { onSelect(issue.identifier); }}
    >
      {/* Row 1: Status dot + Identifier + profile/dispatch (right) */}
      <div className="flex items-center gap-2">
        {/* Status dot — Linear-style */}
        <span className={`h-3.5 w-3.5 flex-shrink-0 rounded-full border-2 border-theme-line ${
          hasActivity ? statusDotClass(issue.orchestratorState) : 'bg-transparent'
        }`} />

        {/* Identifier */}
        {issue.url ? (
          <a
            href={issue.url}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-xs font-medium text-theme-text-secondary hover:underline"
            onClick={(e) => { e.stopPropagation(); }}
          >
            {issue.identifier}
          </a>
        ) : (
          <span className="font-mono text-xs font-medium text-theme-text-secondary">
            {issue.identifier}
          </span>
        )}

        <div className="ml-auto flex items-center gap-1.5">
          {/* Profile selector */}
          {showProfileSelector && (
            <div
              className="flex-shrink-0"
              onClick={(e) => { e.stopPropagation(); }}
              title={isRunning ? 'Cannot change agent while running' : undefined}
            >
              <select
                value={issue.agentProfile ?? ''}
                onChange={(e) => { onProfileChange(issue.identifier, e.target.value); }}
                disabled={isRunning}
                className="max-w-[100px] rounded border border-theme-line bg-theme-panel-strong px-1.5 py-0.5 text-[10px] font-medium text-theme-text-secondary focus:outline-none disabled:opacity-40"
              >
                <option value="">{EMPTY_PROFILE_LABEL}</option>
                {availableProfiles.map((p) => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
            </div>
          )}

          {/* Dispatch button */}
          {onDispatch && (
            <button
              title="Send to queue"
              onClick={(e) => { e.stopPropagation(); onDispatch(issue.identifier); }}
              className="flex-shrink-0 rounded px-1.5 py-0.5 text-[10px] opacity-0 transition-opacity group-hover:opacity-100 text-theme-accent bg-theme-accent-soft"
            >
              ▶
            </button>
          )}
        </div>
      </div>

      {/* Row 2: Title — Linear-style: single line, truncated */}
      <p className="mt-1.5 truncate text-sm font-medium text-theme-text">
        {issue.title}
      </p>

      {/* Row 3: Bottom metadata — compact badges */}
      <div className="mt-2 flex items-center gap-1.5">
        {/* Backend badge */}
        <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${
          backend === 'codex'
            ? 'bg-theme-teal-soft text-theme-teal'
            : 'bg-theme-accent-soft text-theme-accent-strong'
        }`}>
          {backend === 'codex' ? 'Codex' : 'Claude'}
        </span>

        {/* State badge — only when active */}
        {hasActivity && (
          <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium capitalize ${
            isRunning
              ? 'bg-theme-success-soft text-theme-success'
              : 'bg-theme-warning-soft text-theme-warning'
          }`}>
            {issue.orchestratorState}
          </span>
        )}

        {/* Elapsed */}
        {(issue.elapsedMs ?? 0) > 0 && (
          <span className="ml-auto font-mono text-[10px] text-theme-muted">
            {fmtMs(issue.elapsedMs ?? 0)}
          </span>
        )}
      </div>
    </div>
  );
});
