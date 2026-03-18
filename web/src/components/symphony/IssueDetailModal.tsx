import { useState } from 'react';

// Stable fallbacks — prevent new array references on every render, which would
// cause Zustand's useSyncExternalStore to call forceStoreRerender in a loop.
const EMPTY_PROFILES: string[] = [];
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useSymphonyStore } from '../../store/symphonyStore';
import Badge from '../ui/badge/Badge';
import {
  useIssues,
  useCancelIssue,
  useResumeIssue,
  useTerminateIssue,
  useTriggerAIReview,
  useSetIssueProfile,
} from '../../queries/issues';
import { stateBadgeColor, type BadgeColor } from '../../utils/format';

function orchestratorBadgeColor(state: 'running' | 'retrying' | 'paused' | 'idle'): BadgeColor {
  if (state === 'running') return 'success';
  if (state === 'retrying') return 'warning';
  return 'light';
}

// Prose styles for markdown content — requires @tailwindcss/typography
const proseClass = `
  prose prose-sm dark:prose-invert max-w-none
  text-gray-800 dark:text-gray-200
  prose-p:my-1 prose-p:leading-relaxed
  prose-headings:font-semibold prose-headings:mt-3 prose-headings:mb-1
  prose-code:text-xs prose-code:bg-gray-100 dark:prose-code:bg-gray-800 prose-code:px-1 prose-code:rounded prose-code:text-gray-800 dark:prose-code:text-gray-200 prose-code:before:content-none prose-code:after:content-none
  prose-pre:bg-gray-100 dark:prose-pre:bg-gray-800 prose-pre:p-3 prose-pre:rounded-lg prose-pre:text-xs
  prose-ul:my-1 prose-ol:my-1 prose-li:my-0.5
  prose-blockquote:border-l-2 prose-blockquote:border-gray-300 dark:prose-blockquote:border-gray-600 prose-blockquote:pl-3 prose-blockquote:text-gray-500 dark:prose-blockquote:text-gray-400
  prose-a:text-blue-600 dark:prose-a:text-blue-400
  prose-strong:text-gray-900 dark:prose-strong:text-white
`
  .trim()
  .replace(/\s+/g, ' ');

export default function IssueDetailModal() {
  const selectedIdentifier = useSymphonyStore((s) => s.selectedIdentifier);
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const { data: issuesList = [] } = useIssues();
  const issue = issuesList.find((i) => i.identifier === selectedIdentifier) ?? null;
  const cancelIssueMutation = useCancelIssue();
  const terminateIssueMutation = useTerminateIssue();
  const resumeIssueMutation = useResumeIssue();
  const triggerAIReviewMutation = useTriggerAIReview();
  const availableProfiles = useSymphonyStore(
    (s) => s.snapshot?.availableProfiles ?? EMPTY_PROFILES,
  );
  const setIssueProfileMutation = useSetIssueProfile();

  const [cancelling, setCancelling] = useState(false);
  const [cancelled, setCancelled] = useState(false);
  const [terminating, setTerminating] = useState(false);
  const [terminated, setTerminated] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [reviewQueued, setReviewQueued] = useState(false);

  if (!selectedIdentifier || !issue) return null;

  const handleCancel = async () => {
    setCancelling(true);
    try {
      await cancelIssueMutation.mutateAsync(issue.identifier);
      setCancelled(true);
      setTimeout(() => {
        setCancelled(false);
      }, 2000);
    } catch {
      /* error handled by mutation */
    } finally {
      setCancelling(false);
    }
  };

  const handleAIReview = async () => {
    setReviewing(true);
    try {
      await triggerAIReviewMutation.mutateAsync(issue.identifier);
      setReviewQueued(true);
      setTimeout(() => {
        setReviewQueued(false);
      }, 3000);
    } catch {
      /* error handled by mutation */
    } finally {
      setReviewing(false);
    }
  };

  const handleTerminate = async () => {
    setTerminating(true);
    try {
      await terminateIssueMutation.mutateAsync(issue.identifier);
      setTerminated(true);
      setTimeout(() => {
        setTerminated(false);
        close();
      }, 1500);
    } catch {
      /* error handled by mutation */
    } finally {
      setTerminating(false);
    }
  };

  const close = () => {
    setSelectedIdentifier(null);
    setCancelled(false);
    setTerminated(false);
    setReviewQueued(false);
  };

  const isInReview = issue.state.toLowerCase().includes('review');

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      onClick={close}
    >
      <div
        className="flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-gray-900"
        onClick={(e) => {
          e.stopPropagation();
        }}
      >
        {/* Header */}
        <div className="flex items-start justify-between border-b border-gray-200 px-6 py-4 dark:border-gray-800">
          <div>
            <div className="mb-1 flex items-center gap-3">
              <span className="font-mono text-lg font-bold text-gray-900 dark:text-white">
                {issue.identifier}
              </span>
              <Badge color={stateBadgeColor(issue.state)} size="sm">
                {issue.state}
              </Badge>
              <Badge color={orchestratorBadgeColor(issue.orchestratorState)} size="sm">
                {issue.orchestratorState}
              </Badge>
            </div>
            <p className="line-clamp-2 text-sm text-gray-600 dark:text-gray-400">{issue.title}</p>
          </div>
          <button
            onClick={close}
            className="mt-1 ml-4 text-xl leading-none text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            aria-label="Close"
          >
            ×
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 space-y-4 overflow-y-auto px-6 py-4">
          {issue.url && (
            <a
              href={issue.url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-blue-600 hover:underline dark:text-blue-400"
            >
              View in tracker →
            </a>
          )}

          {/* Priority + Labels */}
          {(issue.priority != null || (issue.labels && issue.labels.length > 0)) && (
            <div className="flex flex-wrap items-center gap-2">
              {issue.priority != null && (
                <span className="inline-flex items-center rounded bg-orange-100 px-2 py-0.5 text-xs font-medium text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
                  P{issue.priority}
                </span>
              )}
              {issue.labels?.map((label) => (
                <span
                  key={label}
                  className="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300"
                >
                  {label}
                </span>
              ))}
            </div>
          )}

          {/* Agent Profile */}
          {availableProfiles.length > 0 && (
            <div>
              <h4 className="mb-1 text-xs font-medium tracking-wider text-gray-500 uppercase">
                Agent Profile
              </h4>
              {(() => {
                const locked = issue.state.toLowerCase().includes('progress');
                return locked ? (
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      {issue.agentProfile ?? 'Default'}
                    </span>
                    <span className="rounded bg-amber-50 px-1.5 py-0.5 text-[10px] text-amber-600 dark:bg-amber-900/20 dark:text-amber-400">
                      locked while In Progress
                    </span>
                  </div>
                ) : (
                  <select
                    value={issue.agentProfile ?? ''}
                    onChange={(e) => {
                      setIssueProfileMutation.mutate({
                        identifier: issue.identifier,
                        profile: e.target.value,
                      });
                    }}
                    className="rounded border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
                  >
                    <option value="">Default</option>
                    {availableProfiles.map((p) => (
                      <option key={p} value={p}>
                        {p}
                      </option>
                    ))}
                  </select>
                );
              })()}
            </div>
          )}

          {/* Branch */}
          {issue.branchName && (
            <div>
              <h4 className="mb-1 text-xs font-medium tracking-wider text-gray-500 uppercase">
                Branch
              </h4>
              <div className="flex items-center gap-2">
                <code className="rounded bg-gray-100 px-2 py-1 font-mono text-xs text-gray-700 dark:bg-gray-800 dark:text-gray-300">
                  {issue.branchName}
                </code>
                <button
                  onClick={() => {
                    void navigator.clipboard.writeText(issue.branchName ?? '').catch(() => {});
                  }}
                  className="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  title="Copy branch name"
                >
                  Copy
                </button>
              </div>
            </div>
          )}

          {/* Blocked by */}
          {issue.blockedBy && issue.blockedBy.length > 0 && (
            <div>
              <h4 className="mb-1 text-xs font-medium tracking-wider text-gray-500 uppercase">
                Blocked by
              </h4>
              <div className="flex flex-wrap gap-1.5">
                {issue.blockedBy.map((id) => (
                  <span
                    key={id}
                    className="inline-flex items-center rounded bg-red-50 px-2 py-0.5 font-mono text-xs text-red-700 dark:bg-red-900/20 dark:text-red-400"
                  >
                    {id}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Description — rendered as markdown */}
          <div>
            <h4 className="mb-2 text-xs font-medium tracking-wider text-gray-500 uppercase">
              Description
            </h4>
            {issue.description ? (
              <div className={proseClass}>
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{issue.description}</ReactMarkdown>
              </div>
            ) : (
              <p className="text-sm text-gray-400 italic">No description</p>
            )}
          </div>

          {/* Comments */}
          {issue.comments && issue.comments.length > 0 && (
            <div>
              <h4 className="mb-2 text-xs font-medium tracking-wider text-gray-500 uppercase">
                Comments ({issue.comments.length})
              </h4>
              <div className="space-y-4">
                {issue.comments.map((c, i) => (
                  <div
                    key={i}
                    className="rounded-xl border border-gray-100 bg-gray-50 p-3 dark:border-gray-800 dark:bg-gray-900/40"
                  >
                    <div className="mb-2 flex items-center gap-2">
                      <span className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-400 to-purple-500 text-[10px] font-bold text-white">
                        {(c.author.charAt(0) || '?').toUpperCase()}
                      </span>
                      <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        {c.author}
                      </span>
                      {c.createdAt && (
                        <span className="ml-auto text-xs text-gray-400">
                          {new Date(c.createdAt).toLocaleDateString(undefined, {
                            month: 'short',
                            day: 'numeric',
                            year: 'numeric',
                          })}
                        </span>
                      )}
                    </div>
                    <div className={proseClass}>
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{c.body}</ReactMarkdown>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        {(issue.orchestratorState === 'running' ||
          issue.orchestratorState === 'paused' ||
          isInReview) && (
          <div className="flex items-center justify-between gap-3 border-t border-gray-200 px-6 py-4 dark:border-gray-800">
            {isInReview &&
              (reviewQueued ? (
                <span className="text-sm font-medium text-green-600 dark:text-green-400">
                  ✓ AI reviewer queued
                </span>
              ) : (
                <button
                  onClick={handleAIReview}
                  disabled={reviewing}
                  className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-violet-700 disabled:opacity-50"
                >
                  {reviewing ? 'Dispatching…' : '🤖 AI Review'}
                </button>
              ))}
            <div className="ml-auto flex items-center gap-2">
              {issue.orchestratorState === 'paused' &&
                (terminated ? (
                  <span className="text-sm font-medium text-red-600 dark:text-red-400">
                    ✕ Discarded
                  </span>
                ) : (
                  <>
                    <button
                      onClick={async () => {
                        try {
                          await resumeIssueMutation.mutateAsync(issue.identifier);
                          close();
                        } catch {
                          /* error handled by mutation */
                        }
                      }}
                      className="rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700"
                    >
                      ▶ Resume Agent
                    </button>
                    <button
                      onClick={handleTerminate}
                      disabled={terminating}
                      className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
                    >
                      {terminating ? 'Discarding...' : '✕ Discard'}
                    </button>
                  </>
                ))}
              {issue.orchestratorState === 'running' &&
                (cancelled ? (
                  <span className="text-sm font-medium text-green-600 dark:text-green-400">
                    ⏸ Paused
                  </span>
                ) : terminated ? (
                  <span className="text-sm font-medium text-red-600 dark:text-red-400">
                    ✕ Cancelled
                  </span>
                ) : (
                  <>
                    <button
                      onClick={handleCancel}
                      disabled={cancelling || terminating}
                      className="rounded-lg bg-amber-500 px-4 py-2 text-sm font-medium text-white hover:bg-amber-600 disabled:opacity-50"
                    >
                      {cancelling ? 'Pausing...' : '⏸ Pause Agent'}
                    </button>
                    <button
                      onClick={handleTerminate}
                      disabled={cancelling || terminating}
                      className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
                    >
                      {terminating ? 'Cancelling...' : '✕ Cancel Agent'}
                    </button>
                  </>
                ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
