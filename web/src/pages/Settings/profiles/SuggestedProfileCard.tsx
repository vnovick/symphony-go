import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Modal } from '../../../components/ui/modal';
import { proseClass } from '../../../utils/format';
import type { SuggestedProfile } from './suggestedProfiles';
import { backendLabel, backendBadgeClass } from './ProfileEditorFields';

export function SuggestedProfileCard({
  suggestion,
  onAdd,
  onPreview,
  saving,
}: {
  suggestion: SuggestedProfile;
  onAdd: (s: SuggestedProfile) => Promise<void>;
  onPreview: (s: SuggestedProfile) => void;
  saving: boolean;
}) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => { onPreview(suggestion); }}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') onPreview(suggestion);
      }}
      className="flex cursor-pointer flex-col gap-2 rounded-[var(--radius-md)] border border-dashed p-3 transition-all hover:opacity-90 border-theme-line bg-theme-bg-soft"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-xs font-semibold text-theme-text">
            {suggestion.label}
          </p>
          <span
            className={`mt-0.5 inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium ${backendBadgeClass(suggestion.backend)}`}
          >
            {backendLabel(suggestion.backend)} · {suggestion.model}
          </span>
        </div>
        <button
          onClick={(e) => {
            e.stopPropagation();
            void onAdd(suggestion);
          }}
          disabled={saving}
          className="flex-shrink-0 rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors disabled:opacity-50 hover:opacity-80"
          style={{ borderColor: 'var(--line)', background: 'var(--panel)', color: 'var(--text-secondary)' }}
        >
          {saving ? '…' : '+ Add'}
        </button>
      </div>
      <p className="text-[11px] leading-relaxed text-theme-text-secondary">
        {suggestion.description}
      </p>
      <p className="text-[10px] text-theme-muted">Click to preview full prompt</p>
    </div>
  );
}

export function TemplatePreviewModal({
  suggestion,
  onClose,
  onAdd,
  saving,
}: {
  suggestion: SuggestedProfile | null;
  onClose: () => void;
  onAdd: (s: SuggestedProfile) => Promise<void>;
  saving: boolean;
}) {
  return (
    <Modal isOpen={suggestion !== null} onClose={onClose} className="mx-4 my-8 max-w-2xl">
      {suggestion && (
        <div className="space-y-5 p-6">
          <div>
            <h2 className="text-lg font-semibold text-theme-text">
              {suggestion.label}
            </h2>
            <p className="mt-0.5 text-sm text-theme-text-secondary">
              {suggestion.description}
            </p>
            <div className="mt-2 flex items-center gap-2">
              <span
                className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${backendBadgeClass(suggestion.backend)}`}
              >
                {backendLabel(suggestion.backend)}
              </span>
              <span className="font-mono text-xs text-theme-text-secondary">
                {suggestion.model}
              </span>
              <span className="text-xs text-theme-muted">
                · profile id:{' '}
                <code
                  className="rounded px-1 font-mono text-[11px] bg-theme-bg-soft text-theme-text-secondary"
                >
                  {suggestion.id}
                </code>
              </span>
            </div>
          </div>

          <div
            className="max-h-[50vh] overflow-y-auto rounded-[var(--radius-sm)] border p-4 border-theme-line bg-theme-panel-strong"
          >
            <div className={proseClass}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {suggestion.prompt}
              </ReactMarkdown>
            </div>
          </div>

          <div
            className="flex justify-end gap-3 border-t pt-4 border-theme-line"
          >
            <button
              onClick={onClose}
              className="rounded-[var(--radius-sm)] border px-4 py-2 text-sm transition-colors hover:opacity-80 border-theme-line text-theme-text-secondary"
            >
              Cancel
            </button>
            <button
              onClick={() => {
                void onAdd(suggestion);
                onClose();
              }}
              disabled={saving}
              className="rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50 bg-theme-accent"
            >
              {saving ? 'Adding…' : `Add "${suggestion.id}" profile`}
            </button>
          </div>
        </div>
      )}
    </Modal>
  );
}
