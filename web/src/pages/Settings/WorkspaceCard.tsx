import { useState } from 'react';

interface WorkspaceCardProps {
  autoClearWorkspace: boolean;
  onToggle: (enabled: boolean) => Promise<boolean>;
}

export function WorkspaceCard({ autoClearWorkspace, onToggle }: WorkspaceCardProps) {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const handleChange = async (enabled: boolean) => {
    if (saving) return;
    setSaving(true);
    setError('');
    const ok = await onToggle(enabled);
    setSaving(false);
    if (!ok) setError('Failed to save workspace setting. Please try again.');
  };

  return (
    <div
      className="overflow-hidden rounded-[var(--radius-md)]"
      style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)' }}
    >
      <div
        className="border-b px-5 py-4"
        style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)' }}
      >
        <h2 className="text-sm font-semibold" style={{ color: 'var(--text)' }}>
          Workspace
        </h2>
      </div>
      <div className="px-5 py-5">
        <label className="flex cursor-pointer items-start gap-4">
          <div className="relative mt-0.5 flex-shrink-0">
            <input
              type="checkbox"
              className="sr-only"
              checked={autoClearWorkspace}
              disabled={saving}
              onChange={(e) => {
                void handleChange(e.target.checked);
              }}
            />
            <div
              aria-hidden="true"
              className="h-5 w-9 rounded-full transition-colors"
              style={{ background: autoClearWorkspace ? 'var(--accent)' : 'var(--bg-soft)' }}
            />
            <div
              aria-hidden="true"
              className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${
                autoClearWorkspace ? 'translate-x-4' : 'translate-x-0.5'
              }`}
            />
          </div>
          <div>
            <span className="block text-sm font-medium" style={{ color: 'var(--text)' }}>
              Auto-clear workspace on success
            </span>
            <span className="mt-0.5 block text-xs" style={{ color: 'var(--text-secondary)' }}>
              When a task completes successfully (reaches the completion state), automatically
              delete the cloned workspace directory. Logs are always kept for visibility.
            </span>
            {error && (
              <span role="alert" className="mt-1 block text-xs" style={{ color: 'var(--danger)' }}>
                {error}
              </span>
            )}
          </div>
        </label>
      </div>
    </div>
  );
}
