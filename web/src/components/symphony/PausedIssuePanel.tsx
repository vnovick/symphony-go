import { memo, useState, useCallback } from 'react';
import { Modal } from '../ui/modal';
import type { TrackerIssue } from '../../types/schemas';
import {
  useSetIssueBackend,
  useSetIssueProfile,
  useResumeIssue,
  useTerminateIssue,
} from '../../queries/issues';
import { useToastStore } from '../../store/toastStore';
import { useSymphonyStore } from '../../store/symphonyStore';

interface PausedIssuePanelProps {
  isOpen: boolean;
  issue: TrackerIssue | undefined;
  onClose: () => void;
  availableProfiles: string[];
  defaultBackend: string;
}

function resolveCurrentBackend(
  issue: TrackerIssue,
  defaultBackend: string,
): 'claude' | 'codex' {
  if (issue.agentBackend) return /codex/i.test(issue.agentBackend) ? 'codex' : 'claude';
  return /codex/i.test(defaultBackend) ? 'codex' : 'claude';
}

export const PausedIssuePanel = memo(function PausedIssuePanel({
  isOpen,
  issue,
  onClose,
  availableProfiles,
  defaultBackend,
}: PausedIssuePanelProps) {
  const currentBackend = issue ? resolveCurrentBackend(issue, defaultBackend) : 'claude';
  const [selectedBackend, setSelectedBackend] = useState<'claude' | 'codex'>(currentBackend);
  const [selectedProfile, setSelectedProfile] = useState(issue?.agentProfile ?? '');

  const setIssueBackendMutation = useSetIssueBackend();
  const setIssueProfileMutation = useSetIssueProfile();
  const resumeMutation = useResumeIssue();
  const terminateMutation = useTerminateIssue();
  const refreshSnapshot = useSymphonyStore((s) => s.refreshSnapshot);

  const backendChanged = selectedBackend !== currentBackend;

  const handleBackendToggle = useCallback(
    (backend: 'claude' | 'codex') => {
      if (!issue) return;
      setSelectedBackend(backend);
      setIssueBackendMutation.mutate(
        { identifier: issue.identifier, backend },
        {
          onError: () => {
            useToastStore.getState().addToast('Failed to switch backend', 'error');
            setSelectedBackend(currentBackend);
          },
        },
      );
    },
    [issue, currentBackend, setIssueBackendMutation],
  );

  const handleProfileChange = useCallback(
    (profile: string) => {
      if (!issue) return;
      setSelectedProfile(profile);
      setIssueProfileMutation.mutate({ identifier: issue.identifier, profile });
    },
    [issue, setIssueProfileMutation],
  );

  const handleResume = useCallback(() => {
    if (!issue) return;
    resumeMutation.mutate(issue.identifier, {
      onSuccess: () => {
        void refreshSnapshot();
        onClose();
      },
      onError: () => {
        useToastStore.getState().addToast('Failed to resume issue', 'error');
      },
    });
  }, [issue, resumeMutation, refreshSnapshot, onClose]);

  const handleTerminate = useCallback(() => {
    if (!issue) return;
    terminateMutation.mutate(issue.identifier, {
      onSuccess: () => {
        void refreshSnapshot();
        onClose();
      },
      onError: () => {
        useToastStore.getState().addToast('Failed to terminate issue', 'error');
      },
    });
  }, [issue, terminateMutation, refreshSnapshot, onClose]);

  const resumeLabel = backendChanged
    ? `Resume with ${selectedBackend === 'codex' ? 'Codex' : 'Claude'}`
    : 'Resume';

  return (
    <Modal isOpen={isOpen} onClose={onClose} className="max-w-md mx-auto p-6">
      {issue && (
        <div className="space-y-4">
          {/* Header */}
          <div className="flex items-center gap-2">
            <span className="font-mono text-sm font-medium text-theme-text-secondary">
              {issue.identifier}
            </span>
            <span className="rounded bg-theme-warning-soft px-1.5 py-0.5 text-[10px] font-medium text-theme-warning">
              paused
            </span>
          </div>

          <h3 className="text-base font-medium text-theme-text">{issue.title}</h3>

          {/* Agent Backend */}
          <div className="rounded-lg border border-theme-line bg-theme-panel-strong p-3">
            <span className="text-[11px] font-semibold uppercase tracking-wide text-theme-muted">
              Agent Backend
            </span>
            <div className="mt-2 inline-flex overflow-hidden rounded-md border border-theme-line">
              <button
                onClick={() => {
                  handleBackendToggle('claude');
                }}
                className={`px-3 py-1.5 text-xs font-semibold transition-colors ${
                  selectedBackend === 'claude'
                    ? 'bg-theme-accent-soft text-theme-accent-strong'
                    : 'text-theme-muted hover:text-theme-text'
                }`}
              >
                Claude
              </button>
              <button
                onClick={() => {
                  handleBackendToggle('codex');
                }}
                className={`px-3 py-1.5 text-xs font-semibold transition-colors ${
                  selectedBackend === 'codex'
                    ? 'bg-theme-teal-soft text-theme-teal'
                    : 'text-theme-muted hover:text-theme-text'
                }`}
              >
                Codex
              </button>
            </div>
          </div>

          {/* Agent Profile */}
          {availableProfiles.length > 0 && (
            <div className="rounded-lg border border-theme-line bg-theme-panel-strong p-3">
              <span className="text-[11px] font-semibold uppercase tracking-wide text-theme-muted">
                Agent Profile
              </span>
              <div className="mt-2 flex flex-wrap gap-1.5">
                <button
                  onClick={() => {
                    handleProfileChange('');
                  }}
                  className={`rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                    selectedProfile === ''
                      ? 'border border-theme-accent bg-theme-accent-soft text-theme-accent-strong'
                      : 'border border-theme-line text-theme-muted hover:text-theme-text'
                  }`}
                >
                  default
                </button>
                {availableProfiles.map((p) => (
                  <button
                    key={p}
                    onClick={() => {
                      handleProfileChange(p);
                    }}
                    className={`rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                      selectedProfile === p
                        ? 'border border-theme-accent bg-theme-accent-soft text-theme-accent-strong'
                        : 'border border-theme-line text-theme-muted hover:text-theme-text'
                    }`}
                  >
                    {p}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-2 pt-1">
            <button
              onClick={handleResume}
              disabled={resumeMutation.isPending}
              className="flex-1 rounded-lg bg-theme-success-soft py-2 text-sm font-semibold text-theme-success transition-colors hover:bg-theme-success/20 disabled:opacity-50"
            >
              {resumeMutation.isPending ? 'Resuming\u2026' : `\u25B6 ${resumeLabel}`}
            </button>
            <button
              onClick={handleTerminate}
              disabled={terminateMutation.isPending}
              className="flex-1 rounded-lg bg-theme-danger-soft py-2 text-sm font-semibold text-theme-danger transition-colors hover:bg-theme-danger/20 disabled:opacity-50"
            >
              {terminateMutation.isPending ? 'Terminating\u2026' : '\u2715 Terminate'}
            </button>
          </div>
        </div>
      )}
    </Modal>
  );
});
