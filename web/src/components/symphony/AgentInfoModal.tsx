import { memo } from 'react';
import { Modal } from '../ui/modal';
import type { ProfileDef } from '../../types/schemas';

interface AgentInfoModalProps {
  profileName: string | null;
  profileDef?: ProfileDef;
  onClose: () => void;
}

export const AgentInfoModal = memo(function AgentInfoModal({
  profileName,
  profileDef,
  onClose,
}: AgentInfoModalProps) {
  return (
    <Modal
      isOpen={profileName !== null}
      onClose={onClose}
      showCloseButton
      className="max-w-lg max-h-[85vh] overflow-y-auto p-6"
    >
      {profileName && (
        <div data-testid="agent-info-content">
          <h2 className="text-base font-semibold text-theme-text">{profileName}</h2>
          {profileDef?.backend && (
            <span className="mt-1 inline-block rounded-full px-2 py-0.5 text-[10px] font-medium bg-theme-accent-soft text-theme-accent-strong">
              {profileDef.backend}
            </span>
          )}
          {profileDef?.prompt ? (
            <pre className="mt-3 max-h-[60vh] overflow-y-auto whitespace-pre-wrap rounded-lg p-4 text-xs leading-relaxed border border-theme-line bg-theme-panel-strong text-theme-text-secondary">
              {profileDef.prompt}
            </pre>
          ) : (
            <p className="mt-3 text-sm text-theme-muted">
              No prompt configured for this profile.
            </p>
          )}
        </div>
      )}
    </Modal>
  );
});
