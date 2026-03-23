import { fmtMs, orchDotClass, priorityDotClass } from '../../utils/format';
import type { TrackerIssue } from '../../types/symphony';

interface CardProps {
  issue: TrackerIssue;
  isDragging?: boolean;
  onSelect: (id: string) => void;
  availableProfiles?: string[];
  onProfileChange?: (identifier: string, profile: string) => void;
}

export default function IssueCard({
  issue,
  isDragging,
  onSelect,
  availableProfiles,
  onProfileChange,
}: CardProps) {
  const priorityDot = priorityDotClass(issue.priority);
  const orchDot = orchDotClass(issue.orchestratorState);
  const showProfileSelector = availableProfiles && availableProfiles.length > 0 && onProfileChange;

  return (
    <div
      onClick={() => {
        onSelect(issue.identifier);
      }}
      className={`cursor-pointer rounded-xl border bg-white p-3 shadow-xs transition-all select-none dark:bg-gray-900/70 ${
        isDragging
          ? 'border-brand-400 rotate-1 opacity-90 shadow-lg'
          : 'hover:border-brand-300 dark:hover:border-brand-600 border-gray-200 hover:shadow-md dark:border-gray-700'
      }`}
    >
      <div className="mb-1.5 flex items-center justify-between gap-2">
        <div className="min-w-0 flex-1">
          {issue.url ? (
            <a
              href={issue.url}
              target="_blank"
              rel="noopener noreferrer"
              className="block truncate font-mono text-xs font-semibold text-blue-600 hover:underline dark:text-blue-400"
              onClick={(e) => {
                e.stopPropagation();
              }}
            >
              {issue.identifier}
            </a>
          ) : (
            <span className="block truncate font-mono text-xs font-semibold text-gray-700 dark:text-gray-300">
              {issue.identifier}
            </span>
          )}
        </div>
        <div className="flex flex-shrink-0 items-center gap-1.5">
          {priorityDot && (
            <span
              className={`h-2 w-2 rounded-full ${priorityDot}`}
              title={`P${String(issue.priority ?? '')}`}
            />
          )}
          <span className={`h-2 w-2 rounded-full ${orchDot}`} title={issue.orchestratorState} />
        </div>
      </div>
      <p className="line-clamp-2 text-xs leading-relaxed text-gray-700 dark:text-gray-300">
        {issue.title}
      </p>
      {(issue.elapsedMs ?? 0) > 0 && (
        <p className="mt-1 text-[10px] text-gray-400 dark:text-gray-500">
          ⏱ {fmtMs(issue.elapsedMs ?? 0)}
        </p>
      )}
      {showProfileSelector && (
        <div
          className="mt-2 border-t border-gray-100 pt-1.5 dark:border-gray-800"
          onClick={(e) => {
            e.stopPropagation();
          }}
        >
          <select
            value={issue.agentProfile ?? ''}
            onChange={(e) => {
              onProfileChange(issue.identifier, e.target.value);
            }}
            className="w-full rounded border border-gray-200 bg-transparent px-1.5 py-0.5 text-[10px] text-gray-500 focus:outline-none dark:border-gray-700 dark:bg-transparent dark:text-gray-400"
          >
            <option value="">◈ No agent</option>
            {availableProfiles.map((p) => (
              <option key={p} value={p}>
                ◈ {p}
              </option>
            ))}
          </select>
        </div>
      )}
    </div>
  );
}
