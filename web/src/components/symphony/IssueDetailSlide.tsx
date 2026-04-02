import React, { Suspense, useCallback, useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';

const LazyMarkdown = React.lazy(() =>
  Promise.all([import('react-markdown'), import('remark-gfm')]).then(
    ([{ default: ReactMarkdown }, { default: remarkGfm }]) => ({
      default: (props: { children: string }) => (
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{props.children}</ReactMarkdown>
      ),
    }),
  ),
);
import { useSymphonyStore } from '../../store/symphonyStore';
import Badge from '../ui/badge/Badge';
import { SlidePanel } from '../ui/SlidePanel/SlidePanel';
import {
  useIssues,
  useIssue,
  useCancelIssue,
  useResumeIssue,
  useTerminateIssue,
  useSetIssueProfile,
  useProvideInput,
  useDismissInput,
  ISSUES_KEY,
} from '../../queries/issues';
import { stateBadgeColor, EMPTY_PROFILE_LABEL, EMPTY_PROFILES, proseClass } from '../../utils/format';

export default function IssueDetailSlide() {
  const selectedIdentifier = useSymphonyStore((s) => s.selectedIdentifier);
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const availableProfiles = useSymphonyStore(
    (s) => s.snapshot?.availableProfiles ?? EMPTY_PROFILES,
  );
  const queryClient = useQueryClient();

  const { data: issuesList = [] } = useIssues();
  const { data: freshIssue } = useIssue(selectedIdentifier ?? '');
  const issue = freshIssue ?? issuesList.find((i) => i.identifier === selectedIdentifier) ?? null;

  const cancelIssueMutation = useCancelIssue();
  const terminateIssueMutation = useTerminateIssue();
  const resumeIssueMutation = useResumeIssue();
  const setIssueProfileMutation = useSetIssueProfile();
  const provideInputMutation = useProvideInput();
  const dismissInputMutation = useDismissInput();
  const [replyText, setReplyText] = useState('');

  const close = useCallback(() => {
    setSelectedIdentifier(null);
  }, [setSelectedIdentifier]);

  // Invalidate issues cache when the slide opens so comments/branch info are fresh.
  useEffect(() => {
    if (selectedIdentifier) {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
    }
  }, [selectedIdentifier, queryClient]);

  if (!selectedIdentifier || !issue) return null;

  const isInReview = issue.state.toLowerCase() === 'in review';
  const isProfileLocked = issue.state.toLowerCase().includes('progress');

  return (
    <SlidePanel
      open
      direction="right"
      title={issue.identifier}
      onClose={close}
    >
      {/* Sub-header: badges + title */}
      <div
        className="flex-shrink-0 border-b px-5 py-3 space-y-1 border-theme-line"
      >
        <div className="flex items-center gap-2 flex-wrap">
          <Badge color={stateBadgeColor(issue.state)} size="sm">{issue.state}</Badge>
          <Badge color={issue.orchestratorState === 'running' ? 'success' : issue.orchestratorState === 'retrying' ? 'warning' : 'light'} size="sm">
            {issue.orchestratorState}
          </Badge>
        </div>
        <p className="text-xl font-semibold leading-tight text-theme-text">
          {issue.title}
        </p>
      </div>

      {/* Scrollable body */}
      <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
        {issue.url && (
          <a
            href={issue.url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm hover:underline text-theme-accent"
          >
            View in tracker →
          </a>
        )}

        {/* Priority + Labels */}
        {(issue.priority != null || (issue.labels && issue.labels.length > 0)) && (
          <div className="flex flex-wrap items-center gap-2">
            {issue.priority != null && (
              <span
                className="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-theme-warning-soft text-theme-warning"
              >
                P{issue.priority}
              </span>
            )}
            {issue.labels?.map((label) => (
              <span
                key={label}
                className="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-theme-bg-soft text-theme-text-secondary"
              >
                {label}
              </span>
            ))}
          </div>
        )}

        {/* Agent Profile */}
        {availableProfiles.length > 0 && (
          <div>
            <h4
              className="mb-1 text-xs font-medium uppercase tracking-wider"
            >
              Agent Profile
            </h4>
            {isProfileLocked ? (
              <div className="flex items-center gap-2">
                <span className="text-xs text-theme-text-secondary">
                  {issue.agentProfile ?? EMPTY_PROFILE_LABEL}
                </span>
                <span
                  className="rounded px-1.5 py-0.5 text-[10px] bg-theme-warning-soft text-theme-warning"
                >
                  locked while In Progress
                </span>
              </div>
            ) : (
              <select
                value={issue.agentProfile ?? ''}
                onChange={(e) => {
                  setIssueProfileMutation.mutate({ identifier: issue.identifier, profile: e.target.value });
                }}
                className="rounded-[var(--radius-sm)] border px-3 py-2 text-[13px] focus:outline-none"
                style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)', color: 'var(--text)', cursor: 'pointer', minWidth: '160px' }}
              >
                <option value="">{EMPTY_PROFILE_LABEL}</option>
                {availableProfiles.map((p) => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
            )}
          </div>
        )}

        {/* Branch */}
        {issue.branchName && (
          <div>
            <h4
              className="mb-1 text-xs font-medium uppercase tracking-wider"
            >
              Branch
            </h4>
            <div className="flex items-center gap-2">
              <code
                className="rounded px-2 py-1 font-mono text-xs bg-theme-bg-soft text-theme-text"
              >
                {issue.branchName}
              </code>
              <button
                onClick={() => { void navigator.clipboard.writeText(issue.branchName ?? '').catch(() => {}); }}
                className="text-xs"
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
            <h4
              className="mb-1 text-xs font-medium uppercase tracking-wider"
            >
              Blocked by
            </h4>
            <div className="flex flex-wrap gap-1.5">
              {issue.blockedBy.map((id) => (
                <span
                  key={id}
                  className="inline-flex items-center rounded px-2 py-0.5 font-mono text-xs bg-theme-danger-soft text-theme-danger"
                >
                  {id}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Description */}
        <div>
          <h4
            className="mb-2 text-xs font-medium uppercase tracking-wider"
          >
            Description
          </h4>
          {issue.description ? (
            <div className={proseClass}>
              <Suspense fallback={<div className="animate-pulse">Loading...</div>}>
                <LazyMarkdown>{issue.description}</LazyMarkdown>
              </Suspense>
            </div>
          ) : (
            <p className="text-sm italic text-theme-muted">No description</p>
          )}
        </div>

        {/* Comments */}
        {issue.comments && issue.comments.length > 0 && (
          <div>
            <h4
              className="mb-2 text-xs font-medium uppercase tracking-wider"
            >
              Comments ({issue.comments.length})
            </h4>
            <div className="space-y-4">
              {issue.comments.map((c, i) => (
                <div
                  key={`${c.author}-${c.createdAt ?? String(i)}`}
                  className="rounded-lg border p-3 space-y-2 border-theme-line bg-theme-bg-soft"
                >
                  <div className="flex items-center gap-2">
                    <span
                      className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full text-[10px] font-bold text-white"
                      style={{ background: 'var(--gradient-accent)' }}
                    >
                      {(c.author || '?').charAt(0).toUpperCase()}
                    </span>
                    <span className="text-sm font-medium text-theme-text">
                      {c.author || 'Unknown'}
                    </span>
                    {c.createdAt && (
                      <span className="ml-auto text-xs text-theme-muted">
                        {new Date(c.createdAt).toLocaleDateString(undefined, {
                          month: 'short',
                          day: 'numeric',
                          year: 'numeric',
                        })}
                      </span>
                    )}
                  </div>
                  <div className={proseClass}>
                    <Suspense fallback={<div className="animate-pulse">Loading...</div>}>
                      <LazyMarkdown>{c.body}</LazyMarkdown>
                    </Suspense>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Input Required — reply UI */}
        {issue.orchestratorState === 'input_required' && (
          <div className="rounded-lg border p-4 space-y-3 border-orange-500/30 bg-orange-500/5">
            <div className="flex items-center gap-2">
              <span className="h-2.5 w-2.5 rounded-full bg-orange-400" />
              <h4 className="text-sm font-semibold text-orange-400">
                Agent needs your input
              </h4>
            </div>
            {issue.error && (
              <div className={proseClass}>
                <Suspense fallback={<div className="animate-pulse">Loading...</div>}>
                  <LazyMarkdown>{issue.error}</LazyMarkdown>
                </Suspense>
              </div>
            )}
            <textarea
              value={replyText}
              onChange={(e) => { setReplyText(e.target.value); }}
              placeholder="Type your reply… (will be posted as a comment to the tracker)"
              rows={4}
              className="w-full rounded-lg border px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-orange-400 border-theme-line bg-theme-bg-elevated text-theme-text placeholder:text-theme-muted"
            />
            <div className="flex items-center gap-2">
              <button
                onClick={() => {
                  if (!replyText.trim()) return;
                  provideInputMutation.mutate(
                    { identifier: issue.identifier, message: replyText.trim() },
                    { onSuccess: () => { setReplyText(''); } },
                  );
                }}
                disabled={provideInputMutation.isPending || !replyText.trim()}
                className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 bg-orange-500"
              >
                {provideInputMutation.isPending ? 'Sending…' : 'Reply & Resume Agent'}
              </button>
              <button
                onClick={() => { dismissInputMutation.mutate(issue.identifier); }}
                disabled={dismissInputMutation.isPending}
                className="rounded-lg px-4 py-2 text-sm font-medium hover:opacity-90 disabled:opacity-50 text-theme-text-secondary bg-theme-bg-soft"
              >
                {dismissInputMutation.isPending ? 'Dismissing…' : 'Dismiss'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Sticky action footer */}
      {(issue.orchestratorState === 'running' ||
        issue.orchestratorState === 'retrying' ||
        issue.orchestratorState === 'paused' ||
        issue.orchestratorState === 'input_required' ||
        isInReview) && (
        <div
          className="flex-shrink-0 flex items-center justify-between gap-3 border-t px-5 py-4 border-theme-line"
        >
          {/* AI Review button removed — feature not yet ready for production */}

          <div className="ml-auto flex items-center gap-2">
            {/* Paused state */}
            {issue.orchestratorState === 'paused' && (
              <>
                <button
                  onClick={() => { resumeIssueMutation.mutate(issue.identifier); close(); }}
                  disabled={resumeIssueMutation.isPending}
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 bg-theme-success"
                >
                  {resumeIssueMutation.isPending ? 'Resuming…' : '▶ Resume Agent'}
                </button>
                <button
                  onClick={() => { terminateIssueMutation.mutate(issue.identifier); }}
                  disabled={terminateIssueMutation.isPending}
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 bg-theme-danger"
                >
                  {terminateIssueMutation.isPending ? 'Discarding…' : '✕ Discard'}
                </button>
              </>
            )}

            {/* Running state */}
            {issue.orchestratorState === 'running' && (
              <>
                <button
                  onClick={() => { cancelIssueMutation.mutate(issue.identifier); }}
                  disabled={cancelIssueMutation.isPending || terminateIssueMutation.isPending}
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 bg-theme-warning"
                >
                  {cancelIssueMutation.isPending ? 'Pausing…' : '⏸ Pause Agent'}
                </button>
                <button
                  onClick={() => { terminateIssueMutation.mutate(issue.identifier); }}
                  disabled={cancelIssueMutation.isPending || terminateIssueMutation.isPending}
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 bg-theme-danger"
                >
                  {terminateIssueMutation.isPending ? 'Cancelling…' : '✕ Cancel Agent'}
                </button>
              </>
            )}

            {/* Retrying state */}
            {issue.orchestratorState === 'retrying' && (
              <button
                onClick={() => { cancelIssueMutation.mutate(issue.identifier); }}
                disabled={cancelIssueMutation.isPending}
                className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 bg-theme-warning"
              >
                {cancelIssueMutation.isPending ? 'Cancelling…' : '✕ Cancel Retry'}
              </button>
            )}
          </div>
        </div>
      )}
    </SlidePanel>
  );
}
