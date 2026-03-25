import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import Badge from '../ui/badge/Badge';
import { SlidePanel } from '../ui/SlidePanel/SlidePanel';
import {
  useIssues,
  useIssue,
  useCancelIssue,
  useResumeIssue,
  useTerminateIssue,
  useTriggerAIReview,
  useSetIssueProfile,
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
  const triggerAIReviewMutation = useTriggerAIReview();
  const setIssueProfileMutation = useSetIssueProfile();

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
        className="flex-shrink-0 border-b px-5 py-3 space-y-1"
        style={{ borderColor: 'var(--line)' }}
      >
        <div className="flex items-center gap-2 flex-wrap">
          <Badge color={stateBadgeColor(issue.state)} size="sm">{issue.state}</Badge>
          <Badge color={issue.orchestratorState === 'running' ? 'success' : issue.orchestratorState === 'retrying' ? 'warning' : 'light'} size="sm">
            {issue.orchestratorState}
          </Badge>
        </div>
        <p className="text-xl font-semibold leading-tight" style={{ color: 'var(--text)' }}>
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
            className="text-sm hover:underline"
            style={{ color: 'var(--accent)' }}
          >
            View in tracker →
          </a>
        )}

        {/* Priority + Labels */}
        {(issue.priority != null || (issue.labels && issue.labels.length > 0)) && (
          <div className="flex flex-wrap items-center gap-2">
            {issue.priority != null && (
              <span
                className="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium"
                style={{ background: 'var(--warning-soft)', color: 'var(--warning)' }}
              >
                P{issue.priority}
              </span>
            )}
            {issue.labels?.map((label) => (
              <span
                key={label}
                className="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium"
                style={{ background: 'var(--bg-soft)', color: 'var(--text-secondary)' }}
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
              style={{ color: 'var(--muted)' }}
            >
              Agent Profile
            </h4>
            {isProfileLocked ? (
              <div className="flex items-center gap-2">
                <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                  {issue.agentProfile ?? EMPTY_PROFILE_LABEL}
                </span>
                <span
                  className="rounded px-1.5 py-0.5 text-[10px]"
                  style={{ background: 'var(--warning-soft)', color: 'var(--warning)' }}
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
              style={{ color: 'var(--muted)' }}
            >
              Branch
            </h4>
            <div className="flex items-center gap-2">
              <code
                className="rounded px-2 py-1 font-mono text-xs"
                style={{ background: 'var(--bg-soft)', color: 'var(--text)' }}
              >
                {issue.branchName}
              </code>
              <button
                onClick={() => { void navigator.clipboard.writeText(issue.branchName ?? '').catch(() => {}); }}
                className="text-xs"
                style={{ color: 'var(--muted)' }}
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
              style={{ color: 'var(--muted)' }}
            >
              Blocked by
            </h4>
            <div className="flex flex-wrap gap-1.5">
              {issue.blockedBy.map((id) => (
                <span
                  key={id}
                  className="inline-flex items-center rounded px-2 py-0.5 font-mono text-xs"
                  style={{ background: 'var(--danger-soft)', color: 'var(--danger)' }}
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
            style={{ color: 'var(--muted)' }}
          >
            Description
          </h4>
          {issue.description ? (
            <div className={proseClass}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{issue.description}</ReactMarkdown>
            </div>
          ) : (
            <p className="text-sm italic" style={{ color: 'var(--muted)' }}>No description</p>
          )}
        </div>

        {/* Comments */}
        {issue.comments && issue.comments.length > 0 && (
          <div>
            <h4
              className="mb-2 text-xs font-medium uppercase tracking-wider"
              style={{ color: 'var(--muted)' }}
            >
              Comments ({issue.comments.length})
            </h4>
            <div className="space-y-4">
              {issue.comments.map((c, i) => (
                <div
                  key={`${c.author}-${c.createdAt ?? String(i)}`}
                  className="rounded-lg border p-3 space-y-2"
                  style={{ borderColor: 'var(--line)', background: 'var(--bg-soft)' }}
                >
                  <div className="flex items-center gap-2">
                    <span
                      className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full text-[10px] font-bold text-white"
                      style={{ background: 'var(--gradient-accent)' }}
                    >
                      {(c.author || '?').charAt(0).toUpperCase()}
                    </span>
                    <span className="text-sm font-medium" style={{ color: 'var(--text)' }}>
                      {c.author || 'Unknown'}
                    </span>
                    {c.createdAt && (
                      <span className="ml-auto text-xs" style={{ color: 'var(--muted)' }}>
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

      {/* Sticky action footer */}
      {(issue.orchestratorState === 'running' ||
        issue.orchestratorState === 'retrying' ||
        issue.orchestratorState === 'paused' ||
        isInReview) && (
        <div
          className="flex-shrink-0 flex items-center justify-between gap-3 border-t px-5 py-4"
          style={{ borderColor: 'var(--line)' }}
        >
          {/* AI Review button (in-review state) */}
          {isInReview && (
            <button
              onClick={() => { triggerAIReviewMutation.mutate(issue.identifier); }}
              disabled={triggerAIReviewMutation.isPending}
              className="rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
              style={{ background: 'var(--purple)' }}
            >
              {triggerAIReviewMutation.isPending ? 'Dispatching…' : '✦ AI Review'}
            </button>
          )}

          <div className="ml-auto flex items-center gap-2">
            {/* Paused state */}
            {issue.orchestratorState === 'paused' && (
              <>
                <button
                  onClick={() => { resumeIssueMutation.mutate(issue.identifier); close(); }}
                  disabled={resumeIssueMutation.isPending}
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
                  style={{ background: 'var(--success)' }}
                >
                  {resumeIssueMutation.isPending ? 'Resuming…' : '▶ Resume Agent'}
                </button>
                <button
                  onClick={() => { terminateIssueMutation.mutate(issue.identifier); }}
                  disabled={terminateIssueMutation.isPending}
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
                  style={{ background: 'var(--danger)' }}
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
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
                  style={{ background: 'var(--warning)' }}
                >
                  {cancelIssueMutation.isPending ? 'Pausing…' : '⏸ Pause Agent'}
                </button>
                <button
                  onClick={() => { terminateIssueMutation.mutate(issue.identifier); }}
                  disabled={cancelIssueMutation.isPending || terminateIssueMutation.isPending}
                  className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
                  style={{ background: 'var(--danger)' }}
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
                className="rounded-lg px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
                style={{ background: 'var(--warning)' }}
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
