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
    <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
      <div className="border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Workspace</h2>
      </div>
      <div className="px-6 py-5">
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
              className={`h-5 w-9 rounded-full transition-colors ${
                autoClearWorkspace ? 'bg-brand-500' : 'bg-gray-200 dark:bg-gray-700'
              }`}
            />
            <div
              aria-hidden="true"
              className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${
                autoClearWorkspace ? 'translate-x-4' : 'translate-x-0.5'
              }`}
            />
          </div>
          <div>
            <span className="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Auto-clear workspace on success
            </span>
            <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
              When a task completes successfully (reaches the completion state), automatically
              delete the cloned workspace directory. Logs are always kept for visibility.
            </span>
            {error && (
              <span role="alert" className="mt-1 block text-xs text-red-500 dark:text-red-400">
                {error}
              </span>
            )}
          </div>
        </label>
      </div>
    </div>
  );
}
